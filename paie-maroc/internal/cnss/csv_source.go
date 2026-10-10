package cnss

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// ImportStats describes the disk-backed CNSS declaration import.
type ImportStats struct {
	Imported int64
}

// CSVSource stores large declaration files in a temporary SQLite database.
// Existing lookups and entrant iteration therefore use bounded memory.
type CSVSource struct {
	ctx  context.Context
	db   *sql.DB
	path string
}

// OpenCSVSource imports a semicolon-delimited declaration CSV into a private
// temporary database. The caller must close the returned source.
func OpenCSVSource(ctx context.Context, reader io.Reader, path string) (*CSVSource, ImportStats, error) {
	if path == "" {
		return nil, ImportStats{}, errors.New("chemin temporaire vide")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, ImportStats{}, fmt.Errorf("ouverture de la base CNSS temporaire: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	source := &CSVSource{ctx: ctx, db: db, path: path}
	if err := source.prepare(); err != nil {
		source.Close()
		return nil, ImportStats{}, err
	}
	stats, err := source.importCSV(reader)
	if err != nil {
		source.Close()
		return nil, stats, err
	}
	_ = os.Chmod(path, 0o600)
	return source, stats, nil
}

func (source *CSVSource) Close() error {
	err := source.db.Close()
	removeErr := os.Remove(source.path)
	if err != nil {
		return err
	}
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return removeErr
	}
	return nil
}

func (source *CSVSource) Existing(number string) (Declaration, bool, error) {
	rows, err := source.db.QueryContext(source.ctx, `SELECT cnss, full_name, cin, days, gross_centimes,
		capped_centimes, situation, af_reversal FROM declarations
		WHERE entrant = 0 AND cnss = ? ORDER BY id LIMIT 2`, number)
	if err != nil {
		return Declaration{}, false, err
	}
	defer rows.Close()
	var declaration Declaration
	if !rows.Next() {
		return Declaration{}, false, rows.Err()
	}
	if err := scanDeclaration(rows, &declaration); err != nil {
		return Declaration{}, false, err
	}
	if rows.Next() {
		return Declaration{}, false, fmt.Errorf("plusieurs déclarations pour le numéro CNSS %s", number)
	}
	return declaration, true, rows.Err()
}

func (source *CSVSource) WalkEntrants(yield func(Declaration) error) error {
	rows, err := source.db.QueryContext(source.ctx, `SELECT cnss, full_name, cin, days, gross_centimes,
		capped_centimes, situation, af_reversal FROM declarations
		WHERE entrant = 1 ORDER BY cnss, id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := source.ctx.Err(); err != nil {
			return err
		}
		var declaration Declaration
		if err := scanDeclaration(rows, &declaration); err != nil {
			return err
		}
		if err := yield(declaration); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (source *CSVSource) prepare() error {
	statements := []string{
		`PRAGMA journal_mode = OFF`,
		`PRAGMA synchronous = OFF`,
		`PRAGMA temp_store = FILE`,
		`CREATE TABLE declarations (
			id INTEGER PRIMARY KEY,
			cnss TEXT NOT NULL,
			full_name TEXT NOT NULL,
			cin TEXT NOT NULL,
			days INTEGER NOT NULL,
			gross_centimes INTEGER NOT NULL,
			capped_centimes INTEGER NOT NULL,
			situation TEXT NOT NULL,
			af_reversal INTEGER NOT NULL,
			entrant INTEGER NOT NULL CHECK (entrant IN (0, 1))
		) STRICT`,
		`CREATE INDEX declaration_lookup_idx ON declarations(entrant, cnss, id)`,
	}
	for _, statement := range statements {
		if _, err := source.db.ExecContext(source.ctx, statement); err != nil {
			return fmt.Errorf("préparation de la source CNSS: %w", err)
		}
	}
	return nil
}

func (source *CSVSource) importCSV(reader io.Reader) (ImportStats, error) {
	csvReader := csv.NewReader(bufio.NewReaderSize(reader, 128*1024))
	csvReader.Comma = ';'
	csvReader.ReuseRecord = true
	csvReader.FieldsPerRecord = -1
	header, err := csvReader.Read()
	if err != nil {
		return ImportStats{}, fmt.Errorf("lecture de l'entête CNSS: %w", err)
	}
	header[0] = strings.TrimPrefix(header[0], "\uFEFF")
	columns, err := cnssColumns(header)
	if err != nil {
		return ImportStats{}, err
	}
	tx, err := source.db.BeginTx(source.ctx, nil)
	if err != nil {
		return ImportStats{}, err
	}
	defer tx.Rollback()
	statement, err := tx.PrepareContext(source.ctx, `INSERT INTO declarations
		(cnss, full_name, cin, days, gross_centimes, capped_centimes, situation, af_reversal, entrant)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return ImportStats{}, err
	}
	defer statement.Close()

	var stats ImportStats
	for line := 2; ; line++ {
		record, readErr := csvReader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return stats, fmt.Errorf("ligne CNSS %d: %w", line, readErr)
		}
		declaration, entrant, parseErr := declarationFromRecord(record, columns)
		if parseErr != nil {
			return stats, fmt.Errorf("ligne CNSS %d: %w", line, parseErr)
		}
		if _, err := statement.ExecContext(source.ctx,
			declaration.CNSS, declaration.FullName, declaration.CIN, declaration.Days,
			declaration.GrossCentimes, declaration.CappedGrossCentimes, declaration.Situation,
			declaration.FamilyAllowanceReversal, boolInt(entrant),
		); err != nil {
			return stats, fmt.Errorf("ligne CNSS %d: %w", line, err)
		}
		stats.Imported++
	}
	if stats.Imported == 0 {
		return stats, errors.New("le fichier de déclarations CNSS est vide")
	}
	if err := statement.Close(); err != nil {
		return stats, err
	}
	if err := tx.Commit(); err != nil {
		return stats, err
	}
	return stats, nil
}

func cnssColumns(header []string) (map[string]int, error) {
	columns := make(map[string]int, len(header))
	for index, value := range header {
		columns[strings.ToLower(strings.TrimSpace(value))] = index
	}
	required := []string{"cnss", "nom_complet", "cin", "jours", "salaire_reel_mad", "entrant"}
	for _, column := range required {
		if _, exists := columns[column]; !exists {
			return nil, fmt.Errorf("colonne CNSS obligatoire absente: %s", column)
		}
	}
	return columns, nil
}

func declarationFromRecord(record []string, columns map[string]int) (Declaration, bool, error) {
	value := func(name string) string {
		index, ok := columns[name]
		if !ok || index >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[index])
	}
	days, err := strconv.Atoi(value("jours"))
	if err != nil {
		return Declaration{}, false, errors.New("jours invalides")
	}
	gross, err := parseMAD(value("salaire_reel_mad"))
	if err != nil {
		return Declaration{}, false, fmt.Errorf("salaire réel: %w", err)
	}
	cappedText := value("salaire_plafonne_mad")
	capped := min(gross, monthlyCap)
	if cappedText != "" {
		capped, err = parseMAD(cappedText)
		if err != nil {
			return Declaration{}, false, fmt.Errorf("salaire plafonné: %w", err)
		}
	}
	reversal, err := parseOptionalMAD(value("af_a_reverser_mad"))
	if err != nil {
		return Declaration{}, false, fmt.Errorf("AF à reverser: %w", err)
	}
	entrant, err := parseBool(value("entrant"))
	if err != nil {
		return Declaration{}, false, err
	}
	declaration := Declaration{
		CNSS: value("cnss"), FullName: value("nom_complet"), CIN: value("cin"), Days: days,
		GrossCentimes: gross, CappedGrossCentimes: capped,
		Situation: strings.ToUpper(value("situation")), FamilyAllowanceReversal: reversal,
	}
	if declaration.CNSS == "" && entrant {
		declaration.CNSS = "000000000"
	}
	if err := validateDeclaration(declaration, entrant); err != nil {
		return Declaration{}, false, err
	}
	return declaration, entrant, nil
}

func scanDeclaration(scanner interface{ Scan(...any) error }, declaration *Declaration) error {
	return scanner.Scan(
		&declaration.CNSS, &declaration.FullName, &declaration.CIN, &declaration.Days,
		&declaration.GrossCentimes, &declaration.CappedGrossCentimes, &declaration.Situation,
		&declaration.FamilyAllowanceReversal,
	)
}

func parseMAD(value string) (int64, error) {
	value = strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("montant invalide")
	}
	dirhams, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || dirhams < 0 || dirhams > 99_999_999_999 {
		return 0, errors.New("montant invalide")
	}
	centimes := int64(0)
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
		if len(fraction) != 2 {
			return 0, errors.New("deux décimales maximum")
		}
		centimes, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, errors.New("montant invalide")
		}
	}
	return dirhams*100 + centimes, nil
}

func parseOptionalMAD(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	return parseMAD(value)
}

func parseBool(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "1", "oui", "true", "entrant":
		return true, nil
	case "0", "non", "false", "existant":
		return false, nil
	default:
		return false, fmt.Errorf("entrant doit valoir oui/non ou 1/0, reçu %q", value)
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
