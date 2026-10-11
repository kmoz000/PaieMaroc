// Package store provides disk-backed employee storage for large payrolls.
package store

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
	"time"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/payroll"
	_ "modernc.org/sqlite"
)

const (
	maxPageSize = 200
	commitEvery = 5_000
)

// Store keeps payroll data on disk and exposes bounded pages to Wails.
type Store struct {
	db *sql.DB
}

type Page struct {
	Items      []payroll.Employee `json:"items"`
	NextCursor string             `json:"nextCursor"`
	Total      int64              `json:"total"`
}

type ImportStats struct {
	Imported   int64    `json:"imported"`
	Rejected   int64    `json:"rejected"`
	Errors     []string `json:"errors"`
	DurationMS int64    `json:"durationMs"`
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("chemin de base vide")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("ouverture SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db}
	if err := store.prepare(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if path != ":memory:" {
		_ = os.Chmod(path, 0o600)
	}
	return store, nil
}

func (store *Store) Close() error {
	return store.db.Close()
}

func (store *Store) Upsert(ctx context.Context, employee payroll.Employee) error {
	if issues := payroll.Validate(employee); len(issues) > 0 {
		return fmt.Errorf("%s: %s", issues[0].Field, issues[0].Message)
	}
	_, err := store.db.ExecContext(ctx, upsertSQL, employeeArgs(employee)...)
	if err != nil {
		return fmt.Errorf("enregistrement salarié: %w", err)
	}
	return nil
}

func (store *Store) Delete(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("matricule vide")
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM employees WHERE id = ?`, id); err != nil {
		return fmt.Errorf("suppression salarié: %w", err)
	}
	return nil
}

// List uses keyset pagination, so page cost stays stable as the table grows.
func (store *Store) List(ctx context.Context, cursor, prefix string, limit int) (Page, error) {
	if limit <= 0 || limit > maxPageSize {
		limit = 50
	}
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	query := `SELECT id, first_name, last_name, cin, cnss, rib, days_worked, dependants,
		gross_centimes, exempt_centimes, advance_centimes
		FROM employees WHERE id > ?`
	args := []any{cursor}
	if prefix != "" {
		query += ` AND (id LIKE ? OR search_last_name LIKE ? OR cnss LIKE ?)`
		needle := prefix + "%"
		args = append(args, needle, needle, needle)
	}
	query += ` ORDER BY id LIMIT ?`
	args = append(args, limit+1)

	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Page{}, fmt.Errorf("lecture salariés: %w", err)
	}
	defer rows.Close()

	items := make([]payroll.Employee, 0, limit)
	for rows.Next() {
		var employee payroll.Employee
		if err := rows.Scan(
			&employee.ID,
			&employee.FirstName,
			&employee.LastName,
			&employee.CIN,
			&employee.CNSS,
			&employee.RIB,
			&employee.DaysWorked,
			&employee.Dependants,
			&employee.GrossCentimes,
			&employee.ExemptCentimes,
			&employee.AdvanceCentimes,
		); err != nil {
			return Page{}, fmt.Errorf("décodage salarié: %w", err)
		}
		items = append(items, employee)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("parcours salariés: %w", err)
	}

	var nextCursor string
	if len(items) > limit {
		nextCursor = items[limit-1].ID
		items = items[:limit]
	}
	var total int64
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM employees`).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("comptage salariés: %w", err)
	}
	return Page{Items: items, NextCursor: nextCursor, Total: total}, nil
}

// WalkPayroll calculates rows one at a time in key order. The callback must not
// retain rows indefinitely; this contract keeps memory usage independent of size.
func (store *Store) WalkPayroll(ctx context.Context, yield func(payroll.Result) error) error {
	rows, err := store.db.QueryContext(ctx, `SELECT id, first_name, last_name, cin, cnss, rib,
		days_worked, dependants, gross_centimes, exempt_centimes, advance_centimes
		FROM employees ORDER BY id`)
	if err != nil {
		return fmt.Errorf("parcours de la paie: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var employee payroll.Employee
		if err := rows.Scan(
			&employee.ID, &employee.FirstName, &employee.LastName, &employee.CIN, &employee.CNSS,
			&employee.RIB, &employee.DaysWorked, &employee.Dependants, &employee.GrossCentimes,
			&employee.ExemptCentimes, &employee.AdvanceCentimes,
		); err != nil {
			return fmt.Errorf("décodage de la paie: %w", err)
		}
		result, err := payroll.Calculate(employee)
		if err != nil {
			return fmt.Errorf("calcul %s: %w", employee.ID, err)
		}
		if err := yield(result); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("parcours de la paie: %w", err)
	}
	return nil
}

// ImportCSV streams records and commits bounded batches; it never loads the file in memory.
func (store *Store) ImportCSV(ctx context.Context, reader io.Reader) (ImportStats, error) {
	startedAt := time.Now()
	buffered := bufio.NewReaderSize(reader, 64*1024)
	csvReader := csv.NewReader(buffered)
	csvReader.Comma = ';'
	csvReader.ReuseRecord = true
	csvReader.FieldsPerRecord = -1

	header, err := csvReader.Read()
	if err != nil {
		return ImportStats{}, fmt.Errorf("lecture entête CSV: %w", err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\uFEFF")
	}
	columns, err := mapColumns(header)
	if err != nil {
		return ImportStats{}, err
	}

	transaction, statement, err := store.importTransaction(ctx)
	if err != nil {
		return ImportStats{}, err
	}
	defer statement.Close()
	defer transaction.Rollback()

	stats := ImportStats{Errors: []string{}}
	line := 1
	inBatch := 0
	for {
		line++
		record, readErr := csvReader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			stats.Rejected++
			stats.addError(fmt.Sprintf("ligne %d: %v", line, readErr))
			continue
		}
		employee, parseErr := employeeFromRecord(record, columns)
		if parseErr != nil {
			stats.Rejected++
			stats.addError(fmt.Sprintf("ligne %d: %v", line, parseErr))
			continue
		}
		if _, execErr := statement.ExecContext(ctx, employeeArgs(employee)...); execErr != nil {
			return stats, fmt.Errorf("import ligne %d: %w", line, execErr)
		}
		stats.Imported++
		inBatch++
		if inBatch < commitEvery {
			continue
		}
		if err := statement.Close(); err != nil {
			return stats, fmt.Errorf("finalisation lot: %w", err)
		}
		if err := transaction.Commit(); err != nil {
			return stats, fmt.Errorf("validation lot: %w", err)
		}
		transaction, statement, err = store.importTransaction(ctx)
		if err != nil {
			return stats, err
		}
		inBatch = 0
	}
	if err := statement.Close(); err != nil {
		return stats, fmt.Errorf("finalisation import: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return stats, fmt.Errorf("validation import: %w", err)
	}
	stats.DurationMS = time.Since(startedAt).Milliseconds()
	return stats, nil
}

// ImportCSVForPeriod streams a large employee file into the unique employee
// register and the selected draft month in bounded transactions.
func (store *Store) ImportCSVForPeriod(ctx context.Context, period string, reader io.Reader) (ImportStats, error) {
	startedAt := time.Now()
	csvReader := csv.NewReader(bufio.NewReaderSize(reader, 64*1024))
	csvReader.Comma = ';'
	csvReader.ReuseRecord = true
	csvReader.FieldsPerRecord = -1
	header, err := csvReader.Read()
	if err != nil {
		return ImportStats{}, fmt.Errorf("lecture entête CSV: %w", err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\uFEFF")
	}
	columns, err := mapColumns(header)
	if err != nil {
		return ImportStats{}, err
	}

	var transaction *sql.Tx
	var employeeStatement, periodStatement *sql.Stmt
	openBatch := func() error {
		transaction, err = store.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err = requireDraftPeriod(ctx, transaction, period); err != nil {
			_ = transaction.Rollback()
			transaction = nil
			return err
		}
		employeeStatement, err = transaction.PrepareContext(ctx, upsertSQL)
		if err != nil {
			_ = transaction.Rollback()
			transaction = nil
			return err
		}
		periodStatement, err = transaction.PrepareContext(ctx, upsertPeriodEntrySQL)
		if err != nil {
			_ = employeeStatement.Close()
			_ = transaction.Rollback()
			transaction = nil
			return err
		}
		return nil
	}
	commitBatch := func() error {
		if err := employeeStatement.Close(); err != nil {
			return err
		}
		if err := periodStatement.Close(); err != nil {
			return err
		}
		if err := transaction.Commit(); err != nil {
			return err
		}
		transaction = nil
		return nil
	}
	if err := openBatch(); err != nil {
		return ImportStats{}, fmt.Errorf("début import mensuel: %w", err)
	}
	defer func() {
		if transaction != nil {
			_ = transaction.Rollback()
		}
	}()

	stats := ImportStats{Errors: []string{}}
	line, inBatch := 1, 0
	for {
		line++
		record, readErr := csvReader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			stats.Rejected++
			stats.addError(fmt.Sprintf("ligne %d: %v", line, readErr))
			continue
		}
		employee, parseErr := employeeFromRecord(record, columns)
		if parseErr != nil {
			stats.Rejected++
			stats.addError(fmt.Sprintf("ligne %d: %v", line, parseErr))
			continue
		}
		if _, err := employeeStatement.ExecContext(ctx, employeeArgs(employee)...); err != nil {
			return stats, fmt.Errorf("import salarié ligne %d: %w", line, err)
		}
		if _, err := periodStatement.ExecContext(ctx, period, employee.ID, employee.DaysWorked,
			employee.Dependants, employee.GrossCentimes, employee.ExemptCentimes, employee.AdvanceCentimes); err != nil {
			return stats, fmt.Errorf("import paie ligne %d: %w", line, err)
		}
		stats.Imported++
		inBatch++
		if inBatch == commitEvery {
			if err := commitBatch(); err != nil {
				return stats, fmt.Errorf("validation lot: %w", err)
			}
			if err := openBatch(); err != nil {
				return stats, fmt.Errorf("nouveau lot: %w", err)
			}
			inBatch = 0
		}
	}
	if err := commitBatch(); err != nil {
		return stats, fmt.Errorf("validation import: %w", err)
	}
	stats.DurationMS = time.Since(startedAt).Milliseconds()
	return stats, nil
}

func (store *Store) prepare() error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA synchronous = NORMAL`,
		`PRAGMA temp_store = FILE`,
		`PRAGMA busy_timeout = 5000`,
		`CREATE TABLE IF NOT EXISTS employees (
			id TEXT PRIMARY KEY,
			first_name TEXT NOT NULL,
			last_name TEXT NOT NULL,
			search_last_name TEXT NOT NULL,
			cin TEXT NOT NULL,
			cnss TEXT NOT NULL,
			rib TEXT NOT NULL,
			days_worked INTEGER NOT NULL,
			dependants INTEGER NOT NULL,
			gross_centimes INTEGER NOT NULL,
			exempt_centimes INTEGER NOT NULL,
			advance_centimes INTEGER NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		) STRICT`,
		`CREATE INDEX IF NOT EXISTS employees_last_name_idx ON employees(search_last_name, id)`,
		`CREATE INDEX IF NOT EXISTS employees_cnss_idx ON employees(cnss, id)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			username TEXT NOT NULL UNIQUE COLLATE NOCASE,
			display_name TEXT NOT NULL,
			role TEXT NOT NULL CHECK(role IN ('admin', 'manager')),
			password_hash TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0, 1)),
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_login_at TEXT
		) STRICT`,
		`CREATE INDEX IF NOT EXISTS users_role_enabled_idx ON users(role, enabled)`,
		`CREATE TABLE IF NOT EXISTS payroll_periods (
			period TEXT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft', 'closed')),
			copied_from TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			closed_at TEXT,
			FOREIGN KEY(copied_from) REFERENCES payroll_periods(period) ON DELETE SET NULL
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS payroll_entries (
			period TEXT NOT NULL,
			employee_id TEXT NOT NULL,
			days_worked INTEGER NOT NULL,
			dependants INTEGER NOT NULL,
			gross_centimes INTEGER NOT NULL,
			exempt_centimes INTEGER NOT NULL,
			advance_centimes INTEGER NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(period, employee_id),
			FOREIGN KEY(period) REFERENCES payroll_periods(period) ON DELETE CASCADE,
			FOREIGN KEY(employee_id) REFERENCES employees(id) ON DELETE RESTRICT
		) STRICT`,
		`CREATE INDEX IF NOT EXISTS payroll_entries_employee_idx ON payroll_entries(employee_id, period)`,
		`CREATE TABLE IF NOT EXISTS attendance_devices (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			model TEXT NOT NULL,
			protocol TEXT NOT NULL CHECK(protocol IN ('standalone', 'adms', 'biotime')),
			transport TEXT NOT NULL DEFAULT 'lan' CHECK(transport IN ('lan', 'wireguard')),
			serial_number TEXT NOT NULL DEFAULT '',
			host TEXT NOT NULL,
			port INTEGER NOT NULL CHECK(port BETWEEN 1 AND 65535),
			enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0, 1)),
			last_status TEXT NOT NULL DEFAULT 'never',
			last_checked_at TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(host, port)
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS attendance_employee_links (
			device_id INTEGER NOT NULL,
			employee_id TEXT NOT NULL,
			device_user_id TEXT NOT NULL,
			PRIMARY KEY(device_id, employee_id),
			UNIQUE(device_id, device_user_id),
			FOREIGN KEY(device_id) REFERENCES attendance_devices(id) ON DELETE CASCADE,
			FOREIGN KEY(employee_id) REFERENCES employees(id) ON DELETE CASCADE
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS attendance_punches (
			id INTEGER PRIMARY KEY,
			device_id INTEGER,
			employee_id TEXT,
			device_user_id TEXT NOT NULL DEFAULT '',
			external_id TEXT,
			occurred_at TEXT NOT NULL,
			direction TEXT NOT NULL CHECK(direction IN ('entry', 'exit', 'unknown')),
			verification TEXT NOT NULL CHECK(verification IN ('face', 'fingerprint', 'card', 'password', 'manual', 'other')),
			source TEXT NOT NULL CHECK(source IN ('device', 'manual')),
			note TEXT NOT NULL DEFAULT '',
			edited INTEGER NOT NULL DEFAULT 0 CHECK(edited IN (0, 1)),
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(device_id, external_id),
			FOREIGN KEY(device_id) REFERENCES attendance_devices(id) ON DELETE SET NULL,
			FOREIGN KEY(employee_id) REFERENCES employees(id) ON DELETE SET NULL
		) STRICT`,
		`CREATE INDEX IF NOT EXISTS attendance_punches_time_idx ON attendance_punches(occurred_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS attendance_punches_employee_time_idx ON attendance_punches(employee_id, occurred_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS attendance_punches_device_time_idx ON attendance_punches(device_id, occurred_at DESC, id DESC)`,
		`CREATE TABLE IF NOT EXISTS attendance_punch_changes (
			id INTEGER PRIMARY KEY,
			punch_id INTEGER NOT NULL,
			action TEXT NOT NULL CHECK(action IN ('create', 'edit')),
			reason TEXT NOT NULL,
			changed_by INTEGER NOT NULL,
			before_json TEXT NOT NULL,
			after_json TEXT NOT NULL,
			changed_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(punch_id) REFERENCES attendance_punches(id) ON DELETE CASCADE,
			FOREIGN KEY(changed_by) REFERENCES users(id) ON DELETE RESTRICT
		) STRICT`,
		`CREATE INDEX IF NOT EXISTS attendance_punch_changes_punch_idx ON attendance_punch_changes(punch_id, id DESC)`,
		`CREATE TABLE IF NOT EXISTS attendance_sync_state (
			device_id INTEGER PRIMARY KEY,
			last_synced_at TEXT,
			last_error TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(device_id) REFERENCES attendance_devices(id) ON DELETE CASCADE
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS attendance_holidays (
			date TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			kind TEXT NOT NULL CHECK(kind IN ('national', 'religious', 'company')),
			paid INTEGER NOT NULL CHECK(paid IN (0, 1)),
			premium_basis_points INTEGER NOT NULL CHECK(premium_basis_points BETWEEN 0 AND 50000),
			status TEXT NOT NULL CHECK(status IN ('confirmed', 'provisional')),
			source TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		) STRICT`,
		`CREATE TABLE IF NOT EXISTS attendance_work_policy (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			sector TEXT NOT NULL CHECK(sector IN ('non_agricultural', 'agricultural')),
			weekly_minutes INTEGER NOT NULL CHECK(weekly_minutes BETWEEN 1 AND 10080),
			day_start TEXT NOT NULL,
			day_end TEXT NOT NULL,
			break_minutes INTEGER NOT NULL CHECK(break_minutes BETWEEN 0 AND 720),
			grace_minutes INTEGER NOT NULL CHECK(grace_minutes BETWEEN 0 AND 180),
			day_premium_basis_points INTEGER NOT NULL,
			night_premium_basis_points INTEGER NOT NULL,
			rest_day_premium_basis_points INTEGER NOT NULL,
			rest_night_premium_basis_points INTEGER NOT NULL,
			rules_version TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		) STRICT`,
	}
	for _, statement := range statements {
		if _, err := store.db.Exec(statement); err != nil {
			return fmt.Errorf("initialisation base: %w", err)
		}
	}
	if err := store.ensureColumn("attendance_devices", "transport", "TEXT NOT NULL DEFAULT 'lan' CHECK(transport IN ('lan', 'wireguard'))"); err != nil {
		return err
	}
	if err := store.ensureColumn("attendance_devices", "serial_number", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err := store.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS attendance_devices_serial_idx
		ON attendance_devices(serial_number COLLATE NOCASE) WHERE serial_number <> ''`); err != nil {
		return fmt.Errorf("index des numéros de série pointeuses: %w", err)
	}
	return nil
}

func (store *Store) ensureColumn(table, column, declaration string) error {
	rows, err := store.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return fmt.Errorf("lecture du schéma %s: %w", table, err)
	}
	found := false
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return fmt.Errorf("lecture du schéma %s: %w", table, err)
		}
		if name == column {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("fermeture du schéma %s: %w", table, err)
	}
	if found {
		return nil
	}
	if _, err := store.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + declaration); err != nil {
		return fmt.Errorf("migration %s.%s: %w", table, column, err)
	}
	return nil
}

func (store *Store) importTransaction(ctx context.Context) (*sql.Tx, *sql.Stmt, error) {
	transaction, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("début lot: %w", err)
	}
	statement, err := transaction.PrepareContext(ctx, upsertSQL)
	if err != nil {
		_ = transaction.Rollback()
		return nil, nil, fmt.Errorf("préparation import: %w", err)
	}
	return transaction, statement, nil
}

func (stats *ImportStats) addError(message string) {
	if len(stats.Errors) < 100 {
		stats.Errors = append(stats.Errors, message)
	}
}

func mapColumns(header []string) (map[string]int, error) {
	columns := make(map[string]int, len(header))
	for index, name := range header {
		columns[strings.ToLower(strings.TrimSpace(name))] = index
	}
	required := []string{"matricule", "prenom", "nom", "cin", "cnss", "rib", "jours", "personnes_a_charge", "salaire_brut_mad"}
	for _, name := range required {
		if _, exists := columns[name]; !exists {
			return nil, fmt.Errorf("colonne obligatoire absente: %s", name)
		}
	}
	return columns, nil
}

func employeeFromRecord(record []string, columns map[string]int) (payroll.Employee, error) {
	value := func(name string) string {
		index, exists := columns[name]
		if !exists || index >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[index])
	}
	days, err := strconv.Atoi(value("jours"))
	if err != nil {
		return payroll.Employee{}, errors.New("jours invalides")
	}
	dependants, err := strconv.Atoi(value("personnes_a_charge"))
	if err != nil {
		return payroll.Employee{}, errors.New("personnes à charge invalides")
	}
	gross, err := parseMAD(value("salaire_brut_mad"))
	if err != nil {
		return payroll.Employee{}, fmt.Errorf("salaire brut: %w", err)
	}
	exempt, err := parseOptionalMAD(value("exonere_mad"))
	if err != nil {
		return payroll.Employee{}, fmt.Errorf("exonéré: %w", err)
	}
	advance, err := parseOptionalMAD(value("avance_mad"))
	if err != nil {
		return payroll.Employee{}, fmt.Errorf("avance: %w", err)
	}
	employee := payroll.Employee{
		ID: value("matricule"), FirstName: value("prenom"), LastName: value("nom"), CIN: value("cin"),
		CNSS: value("cnss"), RIB: value("rib"), DaysWorked: days, Dependants: dependants,
		GrossCentimes: gross, ExemptCentimes: exempt, AdvanceCentimes: advance,
	}
	if issues := payroll.Validate(employee); len(issues) > 0 {
		return payroll.Employee{}, fmt.Errorf("%s: %s", issues[0].Field, issues[0].Message)
	}
	return employee, nil
}

func parseOptionalMAD(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	return parseMAD(value)
}

func parseMAD(value string) (int64, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
	parts := strings.Split(normalized, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("montant invalide")
	}
	dirhams, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || dirhams < 0 {
		return 0, errors.New("montant invalide")
	}
	var centimes int64
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

func employeeArgs(employee payroll.Employee) []any {
	return []any{
		employee.ID, employee.FirstName, employee.LastName, strings.ToUpper(employee.LastName), employee.CIN,
		digits(employee.CNSS), digits(employee.RIB), employee.DaysWorked, employee.Dependants,
		employee.GrossCentimes, employee.ExemptCentimes, employee.AdvanceCentimes,
	}
}

func digits(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if character >= '0' && character <= '9' {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

const upsertSQL = `INSERT INTO employees (
	id, first_name, last_name, search_last_name, cin, cnss, rib, days_worked, dependants,
	gross_centimes, exempt_centimes, advance_centimes
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	first_name=excluded.first_name,
	last_name=excluded.last_name,
	search_last_name=excluded.search_last_name,
	cin=excluded.cin,
	cnss=excluded.cnss,
	rib=excluded.rib,
	days_worked=excluded.days_worked,
	dependants=excluded.dependants,
	gross_centimes=excluded.gross_centimes,
	exempt_centimes=excluded.exempt_centimes,
	advance_centimes=excluded.advance_centimes,
	updated_at=CURRENT_TIMESTAMP`

const upsertPeriodEntrySQL = `INSERT INTO payroll_entries (
	period, employee_id, days_worked, dependants, gross_centimes, exempt_centimes, advance_centimes
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(period, employee_id) DO UPDATE SET
	days_worked=excluded.days_worked,
	dependants=excluded.dependants,
	gross_centimes=excluded.gross_centimes,
	exempt_centimes=excluded.exempt_centimes,
	advance_centimes=excluded.advance_centimes,
	updated_at=CURRENT_TIMESTAMP`
