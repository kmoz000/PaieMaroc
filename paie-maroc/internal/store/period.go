package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/payroll"
)

const (
	PeriodDraft  = "draft"
	PeriodClosed = "closed"
)

var periodPattern = regexp.MustCompile(`^20[0-9]{2}-(0[1-9]|1[0-2])$`)

type Period struct {
	Period        string `json:"period"`
	Status        string `json:"status"`
	CopiedFrom    string `json:"copiedFrom"`
	EmployeeCount int64  `json:"employeeCount"`
	GrossCentimes int64  `json:"grossCentimes"`
	CreatedAt     string `json:"createdAt"`
	ClosedAt      string `json:"closedAt"`
}

type CreatePeriodInput struct {
	Period   string `json:"period"`
	CopyFrom string `json:"copyFrom"`
}

func (store *Store) ListPeriods(ctx context.Context) ([]Period, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT p.period, p.status, p.copied_from,
		COUNT(e.employee_id), COALESCE(SUM(e.gross_centimes), 0), p.created_at, p.closed_at
		FROM payroll_periods p LEFT JOIN payroll_entries e ON e.period = p.period
		GROUP BY p.period ORDER BY p.period DESC`)
	if err != nil {
		return nil, fmt.Errorf("lecture des périodes: %w", err)
	}
	defer rows.Close()
	periods := []Period{}
	for rows.Next() {
		var period Period
		var copiedFrom, closedAt sql.NullString
		if err := rows.Scan(&period.Period, &period.Status, &copiedFrom, &period.EmployeeCount,
			&period.GrossCentimes, &period.CreatedAt, &closedAt); err != nil {
			return nil, fmt.Errorf("décodage période: %w", err)
		}
		period.CopiedFrom = copiedFrom.String
		period.ClosedAt = closedAt.String
		periods = append(periods, period)
	}
	return periods, rows.Err()
}

// CreatePeriod snapshots the employee payroll inputs. CopyFrom carries the
// prior month's amounts; an empty CopyFrom starts from employee defaults.
func (store *Store) CreatePeriod(ctx context.Context, input CreatePeriodInput) (Period, error) {
	input.Period = strings.TrimSpace(input.Period)
	input.CopyFrom = strings.TrimSpace(input.CopyFrom)
	if !periodPattern.MatchString(input.Period) {
		return Period{}, errors.New("la période doit respecter AAAA-MM")
	}
	if input.CopyFrom != "" && !periodPattern.MatchString(input.CopyFrom) {
		return Period{}, errors.New("la période source doit respecter AAAA-MM")
	}
	if input.CopyFrom == input.Period {
		return Period{}, errors.New("une période ne peut pas se recopier elle-même")
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return Period{}, fmt.Errorf("début création période: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO payroll_periods (period, copied_from) VALUES (?, NULLIF(?, ''))`, input.Period, input.CopyFrom); err != nil {
		return Period{}, fmt.Errorf("création période: %w", err)
	}
	if input.CopyFrom != "" {
		result, err := tx.ExecContext(ctx, `INSERT INTO payroll_entries (
			period, employee_id, days_worked, dependants, gross_centimes, exempt_centimes, advance_centimes
		) SELECT ?, employee_id, days_worked, dependants, gross_centimes, exempt_centimes, 0
			FROM payroll_entries WHERE period = ?`, input.Period, input.CopyFrom)
		if err != nil {
			return Period{}, fmt.Errorf("recopie de la paie: %w", err)
		}
		if copied, _ := result.RowsAffected(); copied == 0 {
			return Period{}, errors.New("la période source est vide ou introuvable")
		}
	} else {
		if _, err := tx.ExecContext(ctx, `INSERT INTO payroll_entries (
			period, employee_id, days_worked, dependants, gross_centimes, exempt_centimes, advance_centimes
		) SELECT ?, id, days_worked, dependants, gross_centimes, exempt_centimes, 0 FROM employees`, input.Period); err != nil {
			return Period{}, fmt.Errorf("initialisation de la paie: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Period{}, fmt.Errorf("validation période: %w", err)
	}
	return store.GetPeriod(ctx, input.Period)
}

func (store *Store) GetPeriod(ctx context.Context, value string) (Period, error) {
	var period Period
	var copiedFrom, closedAt sql.NullString
	err := store.db.QueryRowContext(ctx, `SELECT p.period, p.status, p.copied_from,
		COUNT(e.employee_id), COALESCE(SUM(e.gross_centimes), 0), p.created_at, p.closed_at
		FROM payroll_periods p LEFT JOIN payroll_entries e ON e.period = p.period
		WHERE p.period = ? GROUP BY p.period`, value).Scan(
		&period.Period, &period.Status, &copiedFrom, &period.EmployeeCount,
		&period.GrossCentimes, &period.CreatedAt, &closedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Period{}, errors.New("période introuvable")
	}
	if err != nil {
		return Period{}, fmt.Errorf("lecture période: %w", err)
	}
	period.CopiedFrom = copiedFrom.String
	period.ClosedAt = closedAt.String
	return period, nil
}

func (store *Store) ListPeriodEmployees(ctx context.Context, period, cursor, prefix string, limit int) (Page, error) {
	if limit <= 0 || limit > maxPageSize {
		limit = 50
	}
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	query := `SELECT e.id, e.first_name, e.last_name, e.cin, e.cnss, e.rib,
		pe.days_worked, pe.dependants, pe.gross_centimes, pe.exempt_centimes, pe.advance_centimes
		FROM payroll_entries pe JOIN employees e ON e.id = pe.employee_id
		WHERE pe.period = ? AND e.id > ?`
	args := []any{period, cursor}
	if prefix != "" {
		query += ` AND (e.id LIKE ? OR e.search_last_name LIKE ? OR e.cnss LIKE ?)`
		needle := prefix + "%"
		args = append(args, needle, needle, needle)
	}
	query += ` ORDER BY e.id LIMIT ?`
	args = append(args, limit+1)
	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Page{}, fmt.Errorf("lecture paie mensuelle: %w", err)
	}
	defer rows.Close()
	items := make([]payroll.Employee, 0, limit)
	for rows.Next() {
		var employee payroll.Employee
		if err := rows.Scan(&employee.ID, &employee.FirstName, &employee.LastName, &employee.CIN,
			&employee.CNSS, &employee.RIB, &employee.DaysWorked, &employee.Dependants,
			&employee.GrossCentimes, &employee.ExemptCentimes, &employee.AdvanceCentimes); err != nil {
			return Page{}, fmt.Errorf("décodage paie mensuelle: %w", err)
		}
		items = append(items, employee)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	var nextCursor string
	if len(items) > limit {
		nextCursor = items[limit-1].ID
		items = items[:limit]
	}
	var total int64
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM payroll_entries WHERE period = ?`, period).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("comptage paie mensuelle: %w", err)
	}
	return Page{Items: items, NextCursor: nextCursor, Total: total}, nil
}

// SavePeriodEmployee updates the single employee master record and the
// selected draft month's payroll values in one transaction.
func (store *Store) SavePeriodEmployee(ctx context.Context, period string, employee payroll.Employee) error {
	if issues := payroll.Validate(employee); len(issues) > 0 {
		return fmt.Errorf("%s: %s", issues[0].Field, issues[0].Message)
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("début enregistrement salarié: %w", err)
	}
	defer tx.Rollback()
	if err := requireDraftPeriod(ctx, tx, period); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, upsertSQL, employeeArgs(employee)...); err != nil {
		return fmt.Errorf("enregistrement salarié: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO payroll_entries (
		period, employee_id, days_worked, dependants, gross_centimes, exempt_centimes, advance_centimes
	) VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(period, employee_id) DO UPDATE SET
		days_worked=excluded.days_worked, dependants=excluded.dependants,
		gross_centimes=excluded.gross_centimes, exempt_centimes=excluded.exempt_centimes,
		advance_centimes=excluded.advance_centimes, updated_at=CURRENT_TIMESTAMP`,
		period, employee.ID, employee.DaysWorked, employee.Dependants, employee.GrossCentimes,
		employee.ExemptCentimes, employee.AdvanceCentimes); err != nil {
		return fmt.Errorf("enregistrement paie mensuelle: %w", err)
	}
	return tx.Commit()
}

func (store *Store) ClosePeriod(ctx context.Context, period string) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("début clôture: %w", err)
	}
	defer tx.Rollback()
	if err := requireDraftPeriod(ctx, tx, period); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.id, e.first_name, e.last_name, e.cin, e.cnss, e.rib,
		pe.days_worked, pe.dependants, pe.gross_centimes, pe.exempt_centimes, pe.advance_centimes
		FROM payroll_entries pe JOIN employees e ON e.id = pe.employee_id WHERE pe.period = ?`, period)
	if err != nil {
		return fmt.Errorf("contrôle de clôture: %w", err)
	}
	count := 0
	for rows.Next() {
		var employee payroll.Employee
		if err := rows.Scan(&employee.ID, &employee.FirstName, &employee.LastName, &employee.CIN,
			&employee.CNSS, &employee.RIB, &employee.DaysWorked, &employee.Dependants,
			&employee.GrossCentimes, &employee.ExemptCentimes, &employee.AdvanceCentimes); err != nil {
			rows.Close()
			return fmt.Errorf("lecture de clôture: %w", err)
		}
		if _, err := payroll.Calculate(employee); err != nil {
			rows.Close()
			return fmt.Errorf("clôture impossible pour %s: %w", employee.ID, err)
		}
		count++
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if count == 0 {
		return errors.New("une paie vide ne peut pas être clôturée")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payroll_periods SET status = 'closed',
		closed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE period = ?`, period); err != nil {
		return fmt.Errorf("clôture de la période: %w", err)
	}
	return tx.Commit()
}

func (store *Store) ReopenPeriod(ctx context.Context, period string) error {
	result, err := store.db.ExecContext(ctx, `UPDATE payroll_periods SET status = 'draft',
		closed_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE period = ? AND status = 'closed'`, period)
	if err != nil {
		return fmt.Errorf("réouverture de la période: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return errors.New("période clôturée introuvable")
	}
	return nil
}

func (store *Store) DeletePeriod(ctx context.Context, period string) error {
	result, err := store.db.ExecContext(ctx, `DELETE FROM payroll_periods WHERE period = ?`, period)
	if err != nil {
		return fmt.Errorf("suppression de la période: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return errors.New("période introuvable")
	}
	return nil
}

func requireDraftPeriod(ctx context.Context, query databaseQuerier, period string) error {
	var status string
	if err := query.QueryRowContext(ctx, `SELECT status FROM payroll_periods WHERE period = ?`, period).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("période introuvable")
		}
		return err
	}
	if status != PeriodDraft {
		return errors.New("rouvrez la période clôturée avant de la modifier")
	}
	return nil
}
