package cnss

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/payroll"
	_ "modernc.org/sqlite"
)

// PayrollWalker streams the employee values stored for one payroll period.
type PayrollWalker func(yield func(payroll.Employee) error) error

// PayrollSource is a bounded-memory bridge between monthly payroll data and
// the e-BDS generator. A temporary SQLite index records which employees were
// present in the CNSS pre-established file; every unmatched row becomes B04.
type PayrollSource struct {
	ctx  context.Context
	db   *sql.DB
	path string
}

func OpenPayrollSource(ctx context.Context, path string, walk PayrollWalker) (*PayrollSource, ImportStats, error) {
	if path == "" {
		return nil, ImportStats{}, errors.New("chemin temporaire vide")
	}
	if walk == nil {
		return nil, ImportStats{}, errors.New("source de période absente")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, ImportStats{}, fmt.Errorf("ouverture de l'index de période CNSS: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	source := &PayrollSource{ctx: ctx, db: db, path: path}
	if err := source.prepare(); err != nil {
		source.Close()
		return nil, ImportStats{}, err
	}
	stats, err := source.importPayroll(walk)
	if err != nil {
		source.Close()
		return nil, stats, err
	}
	_ = os.Chmod(path, 0o600)
	return source, stats, nil
}

func (source *PayrollSource) Close() error {
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

func (source *PayrollSource) Existing(number string) (Declaration, bool, error) {
	rows, err := source.db.QueryContext(source.ctx, `SELECT id, cnss, full_name, cin, days,
		gross_centimes, capped_centimes, situation, af_reversal
		FROM declarations WHERE cnss = ? ORDER BY id LIMIT 2`, number)
	if err != nil {
		return Declaration{}, false, err
	}
	defer rows.Close()
	var id int64
	var declaration Declaration
	if !rows.Next() {
		return Declaration{}, false, rows.Err()
	}
	if err := rows.Scan(
		&id, &declaration.CNSS, &declaration.FullName, &declaration.CIN,
		&declaration.Days, &declaration.GrossCentimes, &declaration.CappedGrossCentimes,
		&declaration.Situation, &declaration.FamilyAllowanceReversal,
	); err != nil {
		return Declaration{}, false, err
	}
	if rows.Next() {
		return Declaration{}, false, fmt.Errorf("plusieurs salariés portent le numéro CNSS %s", number)
	}
	if err := rows.Close(); err != nil {
		return Declaration{}, false, err
	}
	if _, err := source.db.ExecContext(source.ctx, `UPDATE declarations SET matched = 1 WHERE id = ?`, id); err != nil {
		return Declaration{}, false, err
	}
	return declaration, true, nil
}

func (source *PayrollSource) WalkEntrants(yield func(Declaration) error) error {
	rows, err := source.db.QueryContext(source.ctx, `SELECT cnss, full_name, cin, days,
		gross_centimes, capped_centimes, situation, af_reversal
		FROM declarations WHERE matched = 0 ORDER BY cnss, id`)
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

func (source *PayrollSource) prepare() error {
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
			matched INTEGER NOT NULL DEFAULT 0 CHECK (matched IN (0, 1))
		) STRICT`,
	}
	for _, statement := range statements {
		if _, err := source.db.ExecContext(source.ctx, statement); err != nil {
			return fmt.Errorf("préparation de l'index de période CNSS: %w", err)
		}
	}
	return nil
}

func (source *PayrollSource) importPayroll(walk PayrollWalker) (ImportStats, error) {
	tx, err := source.db.BeginTx(source.ctx, nil)
	if err != nil {
		return ImportStats{}, err
	}
	defer tx.Rollback()
	statement, err := tx.PrepareContext(source.ctx, `INSERT INTO declarations
		(cnss, full_name, cin, days, gross_centimes, capped_centimes, situation, af_reversal)
		VALUES (?, ?, ?, ?, ?, ?, '', 0)`)
	if err != nil {
		return ImportStats{}, err
	}
	defer statement.Close()

	var stats ImportStats
	err = walk(func(employee payroll.Employee) error {
		cnssNumber := strings.TrimSpace(employee.CNSS)
		if cnssNumber == "" {
			cnssNumber = "000000000"
		}
		if _, err := statement.ExecContext(
			source.ctx,
			cnssNumber,
			strings.TrimSpace(employee.FirstName+" "+employee.LastName),
			employee.CIN,
			employee.DaysWorked,
			employee.GrossCentimes,
			min(employee.GrossCentimes, monthlyCap),
		); err != nil {
			return fmt.Errorf("indexation du salarié %s: %w", employee.ID, err)
		}
		stats.Imported++
		return nil
	})
	if err != nil {
		return stats, err
	}
	if stats.Imported == 0 {
		return stats, errors.New("la période active ne contient aucun salarié")
	}
	if err := statement.Close(); err != nil {
		return stats, err
	}
	if err := tx.Commit(); err != nil {
		return stats, err
	}
	if _, err := source.db.ExecContext(source.ctx, `CREATE INDEX declaration_cnss_idx ON declarations(cnss, id)`); err != nil {
		return stats, fmt.Errorf("indexation des numéros CNSS: %w", err)
	}
	return stats, nil
}
