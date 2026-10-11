package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/attendance"
)

func TestOpenMigratesExistingAttendanceDevices(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "legacy.db")
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(`CREATE TABLE attendance_devices (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		model TEXT NOT NULL,
		protocol TEXT NOT NULL,
		host TEXT NOT NULL,
		port INTEGER NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 1,
		last_status TEXT NOT NULL DEFAULT 'never',
		last_checked_at TEXT,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(host, port)
	) STRICT`)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() migration error = %v", err)
	}
	defer store.Close()
	device, err := store.SaveAttendanceDevice(context.Background(), attendance.Device{
		Name: "Entrée", Model: "MB10-VL", Protocol: attendance.ProtocolADMS,
		Transport: attendance.TransportWireGuard, SerialNumber: "MB10-LEGACY-01",
		Host: "10.90.0.10", Port: 8088, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if device.Transport != attendance.TransportWireGuard || device.SerialNumber != "MB10-LEGACY-01" {
		t.Fatalf("device = %+v", device)
	}
}
