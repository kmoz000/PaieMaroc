package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/attendance"
)

var deviceUserIDPattern = regexp.MustCompile(`^[0-9]{1,9}$`)

func (store *Store) ListAttendanceDevices(ctx context.Context) ([]attendance.Device, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT d.id, d.name, d.model, d.protocol, d.transport,
		d.serial_number, d.host, d.port,
		d.enabled, d.last_status, d.last_checked_at, COUNT(l.employee_id)
		FROM attendance_devices d LEFT JOIN attendance_employee_links l ON l.device_id = d.id
		GROUP BY d.id ORDER BY d.name, d.id`)
	if err != nil {
		return nil, fmt.Errorf("lecture des pointeuses: %w", err)
	}
	defer rows.Close()
	devices := []attendance.Device{}
	for rows.Next() {
		var device attendance.Device
		var enabled int
		var lastChecked sql.NullString
		if err := rows.Scan(&device.ID, &device.Name, &device.Model, &device.Protocol, &device.Transport,
			&device.SerialNumber, &device.Host,
			&device.Port, &enabled, &device.LastStatus, &lastChecked, &device.LinkedUsers); err != nil {
			return nil, fmt.Errorf("décodage pointeuse: %w", err)
		}
		device.Enabled = enabled == 1
		device.LastCheckedAt = lastChecked.String
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

func (store *Store) SaveAttendanceDevice(ctx context.Context, device attendance.Device) (attendance.Device, error) {
	validated, err := attendance.ValidateDevice(device)
	if err != nil {
		return attendance.Device{}, err
	}
	if validated.ID == 0 {
		result, err := store.db.ExecContext(ctx, `INSERT INTO attendance_devices
			(name, model, protocol, transport, serial_number, host, port, enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			validated.Name, validated.Model, validated.Protocol, validated.Transport, validated.SerialNumber,
			validated.Host, validated.Port, validated.Enabled)
		if err != nil {
			return attendance.Device{}, fmt.Errorf("ajout de la pointeuse: %w", err)
		}
		validated.ID, err = result.LastInsertId()
		if err != nil {
			return attendance.Device{}, err
		}
	} else {
		result, err := store.db.ExecContext(ctx, `UPDATE attendance_devices SET name = ?, model = ?,
			protocol = ?, transport = ?, serial_number = ?, host = ?, port = ?, enabled = ?,
			updated_at = CURRENT_TIMESTAMP WHERE id = ?`, validated.Name, validated.Model, validated.Protocol,
			validated.Transport, validated.SerialNumber, validated.Host, validated.Port, validated.Enabled, validated.ID)
		if err != nil {
			return attendance.Device{}, fmt.Errorf("modification de la pointeuse: %w", err)
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return attendance.Device{}, errors.New("pointeuse introuvable")
		}
	}
	return store.attendanceDeviceByID(ctx, validated.ID)
}

func (store *Store) RecordDeviceProbe(ctx context.Context, id int64, result attendance.ProbeResult) error {
	status := "offline"
	if result.Reachable {
		status = "online"
	}
	changed, err := store.db.ExecContext(ctx, `UPDATE attendance_devices SET last_status = ?,
		last_checked_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("état de la pointeuse: %w", err)
	}
	if count, _ := changed.RowsAffected(); count != 1 {
		return errors.New("pointeuse introuvable")
	}
	return nil
}

func (store *Store) LinkAttendanceEmployee(ctx context.Context, deviceID int64, employeeID, deviceUserID string) error {
	if !deviceUserIDPattern.MatchString(deviceUserID) {
		return errors.New("l’identifiant ZKTeco doit contenir 1 à 9 chiffres")
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("liaison salarié-pointeuse: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO attendance_employee_links
		(device_id, employee_id, device_user_id) VALUES (?, ?, ?)
		ON CONFLICT(device_id, employee_id) DO UPDATE SET device_user_id = excluded.device_user_id`,
		deviceID, employeeID, deviceUserID)
	if err != nil {
		return fmt.Errorf("liaison salarié-pointeuse: %w", err)
	}
	// Reconcile previously downloaded events without touching records that an
	// administrator already linked or corrected.
	if _, err := tx.ExecContext(ctx, `UPDATE attendance_punches SET employee_id = ?,
		updated_at = CURRENT_TIMESTAMP WHERE device_id = ? AND device_user_id = ?
		AND employee_id IS NULL`, employeeID, deviceID, deviceUserID); err != nil {
		return fmt.Errorf("rattachement de l’historique: %w", err)
	}
	return tx.Commit()
}

func (store *Store) attendanceDeviceByID(ctx context.Context, id int64) (attendance.Device, error) {
	var device attendance.Device
	var enabled int
	var lastChecked sql.NullString
	err := store.db.QueryRowContext(ctx, `SELECT d.id, d.name, d.model, d.protocol, d.transport,
		d.serial_number, d.host, d.port,
		d.enabled, d.last_status, d.last_checked_at, COUNT(l.employee_id)
		FROM attendance_devices d LEFT JOIN attendance_employee_links l ON l.device_id = d.id
		WHERE d.id = ? GROUP BY d.id`, id).Scan(
		&device.ID, &device.Name, &device.Model, &device.Protocol, &device.Transport,
		&device.SerialNumber, &device.Host, &device.Port,
		&enabled, &device.LastStatus, &lastChecked, &device.LinkedUsers,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return attendance.Device{}, errors.New("pointeuse introuvable")
	}
	if err != nil {
		return attendance.Device{}, err
	}
	device.Enabled = enabled == 1
	device.LastCheckedAt = lastChecked.String
	return device, nil
}

func (store *Store) AttendanceDeviceBySerial(ctx context.Context, serial string) (attendance.Device, error) {
	var device attendance.Device
	var enabled int
	var lastChecked sql.NullString
	err := store.db.QueryRowContext(ctx, `SELECT d.id, d.name, d.model, d.protocol, d.transport,
		d.serial_number, d.host, d.port, d.enabled, d.last_status, d.last_checked_at,
		COUNT(l.employee_id) FROM attendance_devices d
		LEFT JOIN attendance_employee_links l ON l.device_id = d.id
		WHERE d.serial_number = ? COLLATE NOCASE GROUP BY d.id`, serial).Scan(
		&device.ID, &device.Name, &device.Model, &device.Protocol, &device.Transport,
		&device.SerialNumber, &device.Host, &device.Port, &enabled, &device.LastStatus,
		&lastChecked, &device.LinkedUsers,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return attendance.Device{}, errors.New("pointeuse PUSH / ADMS inconnue")
	}
	if err != nil {
		return attendance.Device{}, fmt.Errorf("lecture de la pointeuse PUSH / ADMS: %w", err)
	}
	device.Enabled = enabled == 1
	device.LastCheckedAt = lastChecked.String
	return device, nil
}
