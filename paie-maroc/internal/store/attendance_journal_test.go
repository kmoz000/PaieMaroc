package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/attendance"
	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/auth"
)

func TestAttendanceJournalImportIsIdempotentAndReconcilesLinks(t *testing.T) {
	t.Parallel()
	database := openStore(t)
	ctx := context.Background()
	if err := database.Upsert(ctx, validEmployee("EMP-001", "168764721")); err != nil {
		t.Fatal(err)
	}
	device, err := database.SaveAttendanceDevice(ctx, attendance.Device{
		Name: "Entrée", Model: "MB10-VL", Protocol: attendance.ProtocolStandalone,
		Host: "192.168.1.201", Port: 4370, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 10, 8, 30, 0, 0, time.Local)
	raw := []attendance.RawPunch{{
		ExternalID: "42|2026-10-10T08:30:00", DeviceUserID: "42", OccurredAt: at,
		Direction: attendance.DirectionEntry, Verification: attendance.VerificationFace,
	}}
	first, err := database.ImportAttendancePunches(ctx, device.ID, raw)
	if err != nil || first.Imported != 1 || first.Unlinked != 1 {
		t.Fatalf("first import = %+v, %v", first, err)
	}
	second, err := database.ImportAttendancePunches(ctx, device.ID, raw)
	if err != nil || second.Imported != 0 || second.Ignored != 1 {
		t.Fatalf("second import = %+v, %v", second, err)
	}
	cursor, err := database.LastAttendanceSync(ctx, device.ID)
	if err != nil || !cursor.Equal(at) {
		t.Fatalf("duplicate import moved cursor: %v, %v", cursor, err)
	}
	if err := database.LinkAttendanceEmployee(ctx, device.ID, "EMP-001", "42"); err != nil {
		t.Fatal(err)
	}
	page, err := database.ListAttendancePunches(ctx, attendance.PunchFilter{From: "2026-10-01", To: "2026-10-31", Limit: 50})
	if err != nil || page.Total != 1 || page.Unlinked != 0 || page.Items[0].EmployeeID != "EMP-001" {
		t.Fatalf("page = %+v, %v", page, err)
	}
}

func TestAttendanceStreamingSessionAdvancesCursorOnlyWhenCompleted(t *testing.T) {
	t.Parallel()
	database := openStore(t)
	ctx := context.Background()
	device, err := database.SaveAttendanceDevice(ctx, attendance.Device{
		Name: "Entrée", Model: "MB10-VL", Protocol: attendance.ProtocolStandalone,
		Host: "192.168.1.201", Port: 4370, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	initial := time.Date(2026, 10, 10, 8, 0, 0, 0, time.Local)
	if _, err := database.ImportAttendancePunches(ctx, device.ID, []attendance.RawPunch{{
		ExternalID: "1", DeviceUserID: "1", OccurredAt: initial,
		Direction: attendance.DirectionEntry, Verification: attendance.VerificationCard,
	}}); err != nil {
		t.Fatal(err)
	}

	session, err := database.BeginAttendanceImport(ctx, device.ID)
	if err != nil {
		t.Fatal(err)
	}
	later := initial.Add(time.Hour)
	result, err := session.ImportBatch(ctx, []attendance.RawPunch{{
		ExternalID: "2", DeviceUserID: "2", OccurredAt: later,
		Direction: attendance.DirectionExit, Verification: attendance.VerificationFingerprint,
	}})
	if err != nil || result.Imported != 1 {
		t.Fatalf("batch result = %+v, %v", result, err)
	}
	cursor, err := database.LastAttendanceSync(ctx, device.ID)
	if err != nil || !cursor.Equal(initial) {
		t.Fatalf("partial import moved cursor: %v, %v", cursor, err)
	}
	if err := database.CompleteAttendanceSync(ctx, device.ID, later); err != nil {
		t.Fatal(err)
	}
	cursor, err = database.LastAttendanceSync(ctx, device.ID)
	if err != nil || !cursor.Equal(later) {
		t.Fatalf("completed cursor = %v, %v", cursor, err)
	}
}

func TestAttendanceJournalUsesStableKeysetPagination(t *testing.T) {
	t.Parallel()
	database := openStore(t)
	ctx := context.Background()
	device, err := database.SaveAttendanceDevice(ctx, attendance.Device{
		Name: "Entrée", Model: "MB10-VL", Protocol: attendance.ProtocolStandalone,
		Host: "192.168.1.201", Port: 4370, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := []attendance.RawPunch{
		{ExternalID: "1", DeviceUserID: "1", OccurredAt: time.Date(2026, 10, 10, 8, 0, 0, 0, time.Local), Direction: attendance.DirectionEntry, Verification: attendance.VerificationCard},
		{ExternalID: "2", DeviceUserID: "1", OccurredAt: time.Date(2026, 10, 10, 12, 0, 0, 0, time.Local), Direction: attendance.DirectionExit, Verification: attendance.VerificationCard},
		{ExternalID: "3", DeviceUserID: "1", OccurredAt: time.Date(2026, 10, 10, 13, 0, 0, 0, time.Local), Direction: attendance.DirectionEntry, Verification: attendance.VerificationCard},
	}
	if _, err := database.ImportAttendancePunches(ctx, device.ID, raw); err != nil {
		t.Fatal(err)
	}
	first, err := database.ListAttendancePunches(ctx, attendance.PunchFilter{From: "2026-10-01", To: "2026-10-31", Limit: 2})
	if err != nil || len(first.Items) != 2 || first.Total != 3 || first.NextCursor == 0 {
		t.Fatalf("first page = %+v, %v", first, err)
	}
	second, err := database.ListAttendancePunches(ctx, attendance.PunchFilter{From: "2026-10-01", To: "2026-10-31", Limit: 2, Cursor: first.NextCursor})
	if err != nil || len(second.Items) != 1 || second.Total != -1 || second.NextCursor != 0 {
		t.Fatalf("second page = %+v, %v", second, err)
	}
	if first.Items[1].ID <= second.Items[0].ID {
		t.Fatalf("pagination order is not strictly descending: first=%d second=%d", first.Items[1].ID, second.Items[0].ID)
	}
}

func TestAttendanceCorrectionCreatesAuditHistory(t *testing.T) {
	t.Parallel()
	database := openStore(t)
	ctx := context.Background()
	admin, err := database.CreateInitialAdmin(ctx, auth.CreateInput{
		Username: "admin", DisplayName: "Admin Paie", Password: "mot de passe robuste",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Upsert(ctx, validEmployee("EMP-001", "168764721")); err != nil {
		t.Fatal(err)
	}
	created, err := database.SaveAttendancePunch(ctx, admin.ID, attendance.PunchInput{
		EmployeeID: "EMP-001", OccurredAt: "2026-10-10T08:30:00+01:00",
		Direction: attendance.DirectionEntry, Verification: attendance.VerificationManual,
		Note: "Feuille de présence",
	})
	if err != nil {
		t.Fatal(err)
	}
	corrected, err := database.SaveAttendancePunch(ctx, admin.ID, attendance.PunchInput{
		ID: created.ID, EmployeeID: "EMP-001", OccurredAt: "2026-10-10T08:20:00+01:00",
		Direction: attendance.DirectionEntry, Verification: attendance.VerificationManual,
		Note: "Heure contrôlée", Reason: "Correction validée par le responsable",
	})
	if err != nil || !corrected.Edited {
		t.Fatalf("corrected = %+v, %v", corrected, err)
	}
	changes, err := database.ListAttendancePunchChanges(ctx, created.ID)
	if err != nil || len(changes) != 2 || changes[0].Action != "edit" || changes[0].ChangedBy != "Admin Paie" {
		t.Fatalf("changes = %+v, %v", changes, err)
	}
}

func TestAttendanceHolidaysMaterializeFixedCalendarAndAllowCompanyDay(t *testing.T) {
	t.Parallel()
	database := openStore(t)
	ctx := context.Background()
	holidays, err := database.ListAttendanceHolidays(ctx, "2026-11")
	if err != nil || len(holidays) != 2 || holidays[0].PremiumBasisPoints != 10000 {
		t.Fatalf("fixed holidays = %+v, %v", holidays, err)
	}
	if err := database.SaveAttendanceHoliday(ctx, attendance.Holiday{
		Date: "2026-11-27", Name: "Inventaire annuel", Kind: "company", Paid: true,
		PremiumBasisPoints: 7500, Status: "confirmed", Source: "Décision interne DRH",
	}); err != nil {
		t.Fatal(err)
	}
	holidays, err = database.ListAttendanceHolidays(ctx, "2026-11")
	if err != nil || len(holidays) != 3 || holidays[2].PremiumBasisPoints != 7500 {
		t.Fatalf("custom holidays = %+v, %v", holidays, err)
	}
}
