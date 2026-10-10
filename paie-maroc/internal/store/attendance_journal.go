package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/attendance"
)

const maxAttendancePageSize = 200

func (store *Store) ListAttendancePunches(ctx context.Context, filter attendance.PunchFilter) (attendance.PunchPage, error) {
	where, args, err := attendanceWhere(filter)
	if err != nil {
		return attendance.PunchPage{}, err
	}
	limit := filter.Limit
	if limit <= 0 || limit > maxAttendancePageSize {
		limit = 50
	}
	query := `SELECT p.id, COALESCE(p.employee_id, ''),
		COALESCE(e.first_name || ' ' || e.last_name, ''), COALESCE(p.device_id, 0),
		COALESCE(d.name, ''), p.device_user_id, p.occurred_at, p.direction,
		p.verification, p.source, p.note, p.edited, p.updated_at
		FROM attendance_punches p
		LEFT JOIN employees e ON e.id = p.employee_id
		LEFT JOIN attendance_devices d ON d.id = p.device_id` + where +
		` ORDER BY p.id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return attendance.PunchPage{}, fmt.Errorf("lecture du journal de pointage: %w", err)
	}
	defer rows.Close()
	items := make([]attendance.Punch, 0, limit)
	for rows.Next() {
		var punch attendance.Punch
		var edited int
		if err := rows.Scan(&punch.ID, &punch.EmployeeID, &punch.EmployeeName, &punch.DeviceID,
			&punch.DeviceName, &punch.DeviceUserID, &punch.OccurredAt, &punch.Direction,
			&punch.Verification, &punch.Source, &punch.Note, &edited, &punch.UpdatedAt); err != nil {
			return attendance.PunchPage{}, fmt.Errorf("décodage du pointage: %w", err)
		}
		punch.Edited = edited == 1
		punch.Unlinked = punch.EmployeeID == ""
		items = append(items, punch)
	}
	if err := rows.Err(); err != nil {
		return attendance.PunchPage{}, err
	}
	if err := rows.Close(); err != nil {
		return attendance.PunchPage{}, err
	}
	page := attendance.PunchPage{Items: items, Total: -1}
	if len(items) > limit {
		page.NextCursor = items[limit-1].ID
		page.Items = items[:limit]
	}
	if filter.Cursor == 0 {
		countWhere, countArgs, err := attendanceWhere(attendance.PunchFilter{
			From: filter.From, To: filter.To, EmployeeID: filter.EmployeeID,
			DeviceID: filter.DeviceID, Query: filter.Query,
		})
		if err != nil {
			return attendance.PunchPage{}, err
		}
		countQuery := `SELECT COUNT(*), COALESCE(SUM(CASE WHEN p.employee_id IS NULL THEN 1 ELSE 0 END), 0)
			FROM attendance_punches p LEFT JOIN employees e ON e.id = p.employee_id` + countWhere
		if err := store.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&page.Total, &page.Unlinked); err != nil {
			return attendance.PunchPage{}, fmt.Errorf("comptage du journal: %w", err)
		}
	}
	return page, nil
}

// WalkAttendancePunches streams the matching journal in chronological order;
// export memory usage therefore stays independent of the record count.
func (store *Store) WalkAttendancePunches(ctx context.Context, filter attendance.PunchFilter, yield func(attendance.Punch) error) error {
	filter.Cursor = 0
	where, args, err := attendanceWhere(filter)
	if err != nil {
		return err
	}
	query := `SELECT p.id, COALESCE(p.employee_id, ''),
		COALESCE(e.first_name || ' ' || e.last_name, ''), COALESCE(p.device_id, 0),
		COALESCE(d.name, ''), p.device_user_id, p.occurred_at, p.direction,
		p.verification, p.source, p.note, p.edited, p.updated_at
		FROM attendance_punches p
		LEFT JOIN employees e ON e.id = p.employee_id
		LEFT JOIN attendance_devices d ON d.id = p.device_id` + where +
		` ORDER BY p.occurred_at, p.id`
	rows, err := store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("export du journal: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var punch attendance.Punch
		var edited int
		if err := rows.Scan(&punch.ID, &punch.EmployeeID, &punch.EmployeeName, &punch.DeviceID,
			&punch.DeviceName, &punch.DeviceUserID, &punch.OccurredAt, &punch.Direction,
			&punch.Verification, &punch.Source, &punch.Note, &edited, &punch.UpdatedAt); err != nil {
			return err
		}
		punch.Edited, punch.Unlinked = edited == 1, punch.EmployeeID == ""
		if err := yield(punch); err != nil {
			return err
		}
	}
	return rows.Err()
}

func attendanceWhere(filter attendance.PunchFilter) (string, []any, error) {
	clauses := []string{" WHERE 1=1"}
	args := []any{}
	if filter.From != "" {
		if _, err := time.Parse("2006-01-02", filter.From); err != nil {
			return "", nil, errors.New("from: date de début invalide")
		}
		clauses = append(clauses, " AND p.occurred_at >= ?")
		args = append(args, filter.From+"T00:00:00")
	}
	if filter.To != "" {
		to, err := time.Parse("2006-01-02", filter.To)
		if err != nil {
			return "", nil, errors.New("to: date de fin invalide")
		}
		clauses = append(clauses, " AND p.occurred_at < ?")
		args = append(args, to.AddDate(0, 0, 1).Format("2006-01-02")+"T00:00:00")
	}
	if employeeID := strings.TrimSpace(filter.EmployeeID); employeeID != "" {
		clauses = append(clauses, " AND p.employee_id = ?")
		args = append(args, employeeID)
	}
	if filter.DeviceID > 0 {
		clauses = append(clauses, " AND p.device_id = ?")
		args = append(args, filter.DeviceID)
	}
	if query := strings.ToUpper(strings.TrimSpace(filter.Query)); query != "" {
		clauses = append(clauses, " AND (UPPER(COALESCE(p.employee_id, '')) LIKE ? OR UPPER(COALESCE(e.first_name || ' ' || e.last_name, '')) LIKE ? OR p.device_user_id LIKE ?)")
		needle := query + "%"
		args = append(args, needle, needle, needle)
	}
	if filter.Cursor > 0 {
		clauses = append(clauses, " AND p.id < ?")
		args = append(args, filter.Cursor)
	}
	return strings.Join(clauses, ""), args, nil
}

func (store *Store) SaveAttendancePunch(ctx context.Context, actorID int64, input attendance.PunchInput) (attendance.Punch, error) {
	validated, err := attendance.ValidatePunch(input)
	if err != nil {
		return attendance.Punch{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return attendance.Punch{}, err
	}
	defer tx.Rollback()

	beforeJSON := "{}"
	action := "create"
	if validated.ID == 0 {
		result, err := tx.ExecContext(ctx, `INSERT INTO attendance_punches
			(device_id, employee_id, occurred_at, direction, verification, source, note)
			VALUES (NULLIF(?, 0), ?, ?, ?, ?, 'manual', ?)`, validated.DeviceID,
			validated.EmployeeID, validated.OccurredAt, validated.Direction, validated.Verification, validated.Note)
		if err != nil {
			return attendance.Punch{}, fmt.Errorf("ajout du pointage: %w", err)
		}
		validated.ID, err = result.LastInsertId()
		if err != nil {
			return attendance.Punch{}, err
		}
	} else {
		action = "edit"
		before, err := attendancePunchByID(ctx, tx, validated.ID)
		if err != nil {
			return attendance.Punch{}, err
		}
		beforeBytes, _ := json.Marshal(before)
		beforeJSON = string(beforeBytes)
		result, err := tx.ExecContext(ctx, `UPDATE attendance_punches SET employee_id = ?,
			device_id = NULLIF(?, 0), occurred_at = ?, direction = ?, verification = ?, note = ?,
			edited = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, validated.EmployeeID,
			validated.DeviceID, validated.OccurredAt, validated.Direction, validated.Verification,
			validated.Note, validated.ID)
		if err != nil {
			return attendance.Punch{}, fmt.Errorf("correction du pointage: %w", err)
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return attendance.Punch{}, errors.New("pointage introuvable")
		}
	}
	afterBytes, _ := json.Marshal(validated)
	reason := validated.Reason
	if action == "create" && reason == "" {
		reason = "Ajout manuel"
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO attendance_punch_changes
		(punch_id, action, reason, changed_by, before_json, after_json) VALUES (?, ?, ?, ?, ?, ?)`,
		validated.ID, action, reason, actorID, beforeJSON, string(afterBytes)); err != nil {
		return attendance.Punch{}, fmt.Errorf("journalisation de la correction: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return attendance.Punch{}, err
	}
	return store.AttendancePunchByID(ctx, validated.ID)
}

func (store *Store) AttendancePunchByID(ctx context.Context, id int64) (attendance.Punch, error) {
	return attendancePunchByID(ctx, store.db, id)
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func attendancePunchByID(ctx context.Context, query rowQuerier, id int64) (attendance.Punch, error) {
	var punch attendance.Punch
	var edited int
	err := query.QueryRowContext(ctx, `SELECT p.id, COALESCE(p.employee_id, ''),
		COALESCE(e.first_name || ' ' || e.last_name, ''), COALESCE(p.device_id, 0),
		COALESCE(d.name, ''), p.device_user_id, p.occurred_at, p.direction,
		p.verification, p.source, p.note, p.edited, p.updated_at
		FROM attendance_punches p LEFT JOIN employees e ON e.id = p.employee_id
		LEFT JOIN attendance_devices d ON d.id = p.device_id WHERE p.id = ?`, id).Scan(
		&punch.ID, &punch.EmployeeID, &punch.EmployeeName, &punch.DeviceID, &punch.DeviceName,
		&punch.DeviceUserID, &punch.OccurredAt, &punch.Direction, &punch.Verification,
		&punch.Source, &punch.Note, &edited, &punch.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return attendance.Punch{}, errors.New("pointage introuvable")
	}
	if err != nil {
		return attendance.Punch{}, err
	}
	punch.Edited, punch.Unlinked = edited == 1, punch.EmployeeID == ""
	return punch, nil
}

func (store *Store) ListAttendancePunchChanges(ctx context.Context, punchID int64) ([]attendance.PunchChange, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT c.id, c.punch_id, c.action, c.reason,
		u.display_name, c.before_json, c.after_json, c.changed_at
		FROM attendance_punch_changes c JOIN users u ON u.id = c.changed_by
		WHERE c.punch_id = ? ORDER BY c.id DESC`, punchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	changes := []attendance.PunchChange{}
	for rows.Next() {
		var change attendance.PunchChange
		if err := rows.Scan(&change.ID, &change.PunchID, &change.Action, &change.Reason,
			&change.ChangedBy, &change.BeforeJSON, &change.AfterJSON, &change.ChangedAt); err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	return changes, rows.Err()
}

func (store *Store) LastAttendanceSync(ctx context.Context, deviceID int64) (time.Time, error) {
	var value sql.NullString
	if err := store.db.QueryRowContext(ctx, `SELECT last_synced_at FROM attendance_sync_state WHERE device_id = ?`, deviceID).Scan(&value); errors.Is(err, sql.ErrNoRows) {
		return time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local), nil
	} else if err != nil {
		return time.Time{}, err
	}
	parsed, err := time.Parse(time.RFC3339, value.String)
	if err != nil {
		return time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local), nil
	}
	return parsed, nil
}

// AttendanceImportSession caches employee links once while committing each
// bounded device batch independently. A failed transfer can therefore resume
// safely without retaining the complete device journal in memory.
type AttendanceImportSession struct {
	store    *Store
	deviceID int64
	links    map[string]string
}

func (store *Store) BeginAttendanceImport(ctx context.Context, deviceID int64) (*AttendanceImportSession, error) {
	links := make(map[string]string)
	rows, err := store.db.QueryContext(ctx, `SELECT device_user_id, employee_id FROM attendance_employee_links WHERE device_id = ?`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var userID, employeeID string
		if err := rows.Scan(&userID, &employeeID); err != nil {
			return nil, err
		}
		links[userID] = employeeID
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &AttendanceImportSession{store: store, deviceID: deviceID, links: links}, nil
}

func (session *AttendanceImportSession) ImportBatch(ctx context.Context, raw []attendance.RawPunch) (attendance.SyncResult, error) {
	result := attendance.SyncResult{DeviceID: session.deviceID, Read: int64(len(raw))}
	if len(raw) == 0 {
		return result, nil
	}
	tx, err := session.store.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	statement, err := tx.PrepareContext(ctx, `INSERT INTO attendance_punches
		(device_id, employee_id, device_user_id, external_id, occurred_at, direction, verification, source)
		VALUES (?, NULLIF(?, ''), ?, ?, ?, ?, ?, 'device') ON CONFLICT(device_id, external_id) DO NOTHING`)
	if err != nil {
		return result, err
	}
	defer statement.Close()
	for _, punch := range raw {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		employeeID := session.links[punch.DeviceUserID]
		execResult, err := statement.ExecContext(ctx, session.deviceID, employeeID, punch.DeviceUserID,
			punch.ExternalID, punch.OccurredAt.Format(time.RFC3339), punch.Direction, punch.Verification)
		if err != nil {
			return result, fmt.Errorf("import du pointage: %w", err)
		}
		changed, _ := execResult.RowsAffected()
		if changed == 0 {
			result.Ignored++
			continue
		}
		result.Imported++
		if employeeID == "" {
			result.Unlinked++
		}
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func (store *Store) CompleteAttendanceSync(ctx context.Context, deviceID int64, latest time.Time) error {
	latestValue := ""
	if !latest.IsZero() {
		latestValue = latest.Format(time.RFC3339)
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO attendance_sync_state(device_id, last_synced_at, last_error)
		VALUES (?, NULLIF(?, ''), '') ON CONFLICT(device_id) DO UPDATE SET
		last_synced_at = CASE
			WHEN excluded.last_synced_at IS NULL THEN attendance_sync_state.last_synced_at
			WHEN attendance_sync_state.last_synced_at IS NULL THEN excluded.last_synced_at
			WHEN unixepoch(excluded.last_synced_at) > unixepoch(attendance_sync_state.last_synced_at) THEN excluded.last_synced_at
			ELSE attendance_sync_state.last_synced_at END,
		last_error = '', updated_at = CURRENT_TIMESTAMP`, deviceID, latestValue); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE attendance_devices SET last_status = 'online',
		last_checked_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, deviceID); err != nil {
		return err
	}
	return tx.Commit()
}

// ImportAttendancePunches remains the convenient atomic API for tests and
// small adapters. Standalone device synchronization uses the streaming session.
func (store *Store) ImportAttendancePunches(ctx context.Context, deviceID int64, raw []attendance.RawPunch) (attendance.SyncResult, error) {
	session, err := store.BeginAttendanceImport(ctx, deviceID)
	if err != nil {
		return attendance.SyncResult{DeviceID: deviceID}, err
	}
	result, err := session.ImportBatch(ctx, raw)
	if err != nil {
		return result, err
	}
	var latest time.Time
	for _, punch := range raw {
		if punch.OccurredAt.After(latest) {
			latest = punch.OccurredAt
		}
	}
	if err := store.CompleteAttendanceSync(ctx, deviceID, latest); err != nil {
		return result, err
	}
	result.Message = fmt.Sprintf("%d nouveau(x), %d déjà présent(s), %d non lié(s)", result.Imported, result.Ignored, result.Unlinked)
	return result, nil
}

func (store *Store) RecordAttendanceSyncError(ctx context.Context, deviceID int64, syncErr error) {
	_, _ = store.db.ExecContext(ctx, `INSERT INTO attendance_sync_state(device_id, last_error)
		VALUES (?, ?) ON CONFLICT(device_id) DO UPDATE SET last_error = excluded.last_error,
		updated_at = CURRENT_TIMESTAMP`, deviceID, syncErr.Error())
	_, _ = store.db.ExecContext(ctx, `UPDATE attendance_devices SET last_status = 'offline',
		last_checked_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, deviceID)
}

func (store *Store) AttendanceDeviceByID(ctx context.Context, id int64) (attendance.Device, error) {
	return store.attendanceDeviceByID(ctx, id)
}

func (store *Store) ListAttendanceHolidays(ctx context.Context, month string) ([]attendance.Holiday, error) {
	parsed, err := time.Parse("2006-01", month)
	if err != nil {
		return nil, errors.New("mois invalide, format attendu AAAA-MM")
	}
	if err := store.ensureFixedHolidays(ctx, parsed.Year()); err != nil {
		return nil, err
	}
	next := parsed.AddDate(0, 1, 0).Format("2006-01")
	rows, err := store.db.QueryContext(ctx, `SELECT date, name, kind, paid, premium_basis_points, status, source
		FROM attendance_holidays WHERE date >= ? AND date < ? ORDER BY date`, month+"-01", next+"-01")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	holidays := []attendance.Holiday{}
	for rows.Next() {
		var holiday attendance.Holiday
		var paid int
		if err := rows.Scan(&holiday.Date, &holiday.Name, &holiday.Kind, &paid,
			&holiday.PremiumBasisPoints, &holiday.Status, &holiday.Source); err != nil {
			return nil, err
		}
		holiday.Paid = paid == 1
		holidays = append(holidays, holiday)
	}
	return holidays, rows.Err()
}

func (store *Store) SaveAttendanceHoliday(ctx context.Context, holiday attendance.Holiday) error {
	if _, err := time.Parse("2006-01-02", holiday.Date); err != nil {
		return errors.New("date: date invalide")
	}
	holiday.Name = strings.TrimSpace(holiday.Name)
	if holiday.Name == "" {
		return errors.New("name: le nom est obligatoire")
	}
	if holiday.Kind != "national" && holiday.Kind != "religious" && holiday.Kind != "company" {
		return errors.New("kind: type invalide")
	}
	if holiday.Status != "confirmed" && holiday.Status != "provisional" {
		return errors.New("status: statut invalide")
	}
	if holiday.PremiumBasisPoints < 0 || holiday.PremiumBasisPoints > 50000 {
		return errors.New("premiumBasisPoints: majoration invalide")
	}
	_, err := store.db.ExecContext(ctx, `INSERT INTO attendance_holidays
		(date, name, kind, paid, premium_basis_points, status, source) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(date) DO UPDATE SET name=excluded.name, kind=excluded.kind, paid=excluded.paid,
		premium_basis_points=excluded.premium_basis_points, status=excluded.status,
		source=excluded.source, updated_at=CURRENT_TIMESTAMP`, holiday.Date, holiday.Name, holiday.Kind,
		holiday.Paid, holiday.PremiumBasisPoints, holiday.Status, strings.TrimSpace(holiday.Source))
	return err
}

func (store *Store) DeleteAttendanceHoliday(ctx context.Context, date string) error {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return errors.New("date invalide")
	}
	_, err := store.db.ExecContext(ctx, `DELETE FROM attendance_holidays WHERE date = ?`, date)
	return err
}

func (store *Store) ensureFixedHolidays(ctx context.Context, year int) error {
	// Fixed statutory dates are materialized locally. Religious dates are not
	// guessed because their official observance is announced separately.
	// Correct only the former built-in value; administrator overrides have a
	// different source and remain untouched.
	if _, err := store.db.ExecContext(ctx, `UPDATE attendance_holidays
		SET premium_basis_points = 10000,
			source = 'Calendrier légal intégré; contrôle annuel requis',
			updated_at = CURRENT_TIMESTAMP
		WHERE source = 'Décret n° 2-04-426; calendrier légal intégré'`); err != nil {
		return err
	}
	fixed := []struct{ date, name string }{
		{"01-01", "Nouvel An"}, {"01-11", "Manifeste de l’Indépendance"},
		{"01-14", "Nouvel An amazigh"}, {"05-01", "Fête du Travail"},
		{"07-30", "Fête du Trône"}, {"08-14", "Allégeance Oued Eddahab"},
		{"08-20", "Révolution du Roi et du Peuple"}, {"08-21", "Fête de la Jeunesse"},
		{"11-06", "Marche Verte"}, {"11-18", "Fête de l’Indépendance"},
	}
	for _, item := range fixed {
		_, err := store.db.ExecContext(ctx, `INSERT INTO attendance_holidays
			(date, name, kind, paid, premium_basis_points, status, source)
			VALUES (?, ?, 'national', 1, 10000, 'confirmed', 'Calendrier légal intégré; contrôle annuel requis')
			ON CONFLICT(date) DO NOTHING`, strconv.Itoa(year)+"-"+item.date, item.name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (store *Store) GetAttendanceWorkPolicy(ctx context.Context) (attendance.WorkPolicy, error) {
	policy := attendance.DefaultWorkPolicy()
	err := store.db.QueryRowContext(ctx, `SELECT sector, weekly_minutes, day_start, day_end,
		break_minutes, grace_minutes, day_premium_basis_points, night_premium_basis_points,
		rest_day_premium_basis_points, rest_night_premium_basis_points, rules_version
		FROM attendance_work_policy WHERE id = 1`).Scan(&policy.Sector, &policy.WeeklyMinutes,
		&policy.DayStart, &policy.DayEnd, &policy.BreakMinutes, &policy.GraceMinutes,
		&policy.DayPremiumBasisPoints, &policy.NightPremiumBasisPoints,
		&policy.RestDayPremiumBasisPoints, &policy.RestNightPremiumBasisPoints, &policy.RulesVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return policy, nil
	}
	return policy, err
}

func (store *Store) SaveAttendanceWorkPolicy(ctx context.Context, policy attendance.WorkPolicy) error {
	if policy.Sector != "non_agricultural" && policy.Sector != "agricultural" {
		return errors.New("sector: secteur invalide")
	}
	if policy.WeeklyMinutes <= 0 || policy.WeeklyMinutes > 7*24*60 {
		return errors.New("weeklyMinutes: durée hebdomadaire invalide")
	}
	for field, value := range map[string]string{"dayStart": policy.DayStart, "dayEnd": policy.DayEnd} {
		if _, err := time.Parse("15:04", value); err != nil {
			return fmt.Errorf("%s: horaire invalide", field)
		}
	}
	if policy.BreakMinutes < 0 || policy.BreakMinutes > 720 || policy.GraceMinutes < 0 || policy.GraceMinutes > 180 {
		return errors.New("durée de pause ou tolérance invalide")
	}
	_, err := store.db.ExecContext(ctx, `INSERT INTO attendance_work_policy
		(id, sector, weekly_minutes, day_start, day_end, break_minutes, grace_minutes,
		day_premium_basis_points, night_premium_basis_points, rest_day_premium_basis_points,
		rest_night_premium_basis_points, rules_version) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET sector=excluded.sector, weekly_minutes=excluded.weekly_minutes,
		day_start=excluded.day_start, day_end=excluded.day_end, break_minutes=excluded.break_minutes,
		grace_minutes=excluded.grace_minutes, day_premium_basis_points=excluded.day_premium_basis_points,
		night_premium_basis_points=excluded.night_premium_basis_points,
		rest_day_premium_basis_points=excluded.rest_day_premium_basis_points,
		rest_night_premium_basis_points=excluded.rest_night_premium_basis_points,
		rules_version=excluded.rules_version, updated_at=CURRENT_TIMESTAMP`, policy.Sector,
		policy.WeeklyMinutes, policy.DayStart, policy.DayEnd, policy.BreakMinutes, policy.GraceMinutes,
		policy.DayPremiumBasisPoints, policy.NightPremiumBasisPoints,
		policy.RestDayPremiumBasisPoints, policy.RestNightPremiumBasisPoints, policy.RulesVersion)
	return err
}
