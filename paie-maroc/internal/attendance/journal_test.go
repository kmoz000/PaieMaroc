package attendance_test

import (
	"testing"
	"time"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/attendance"
)

func TestValidatePunchRequiresCorrectionReason(t *testing.T) {
	t.Parallel()
	input := attendance.PunchInput{
		ID: 12, EmployeeID: "EMP-001", OccurredAt: "2026-10-10T08:00:00+01:00",
		Direction: attendance.DirectionEntry, Verification: attendance.VerificationFace,
	}
	if _, err := attendance.ValidatePunch(input); err == nil {
		t.Fatal("ValidatePunch() accepted an edit without a reason")
	}
	input.Reason = "Correction validée sur la feuille de présence"
	if _, err := attendance.ValidatePunch(input); err != nil {
		t.Fatalf("ValidatePunch() error = %v", err)
	}
}

func TestSummarizeDayPairsPunchesAndFlagsIncompleteSequence(t *testing.T) {
	t.Parallel()
	punches := []attendance.Punch{
		{EmployeeID: "EMP-001", EmployeeName: "Amine Alaoui", OccurredAt: "2026-10-10T08:35:00+01:00", Direction: attendance.DirectionEntry},
		{EmployeeID: "EMP-001", EmployeeName: "Amine Alaoui", OccurredAt: "2026-10-10T12:00:00+01:00", Direction: attendance.DirectionExit},
		{EmployeeID: "EMP-001", EmployeeName: "Amine Alaoui", OccurredAt: "2026-10-10T13:00:00+01:00", Direction: attendance.DirectionEntry},
	}
	policy := attendance.DefaultWorkPolicy()
	policy.DayStart = "08:30"
	summary := attendance.SummarizeDay(punches, policy, false)
	if summary.WorkedMinutes != int((3*time.Hour+25*time.Minute)/time.Minute) {
		t.Fatalf("WorkedMinutes = %d", summary.WorkedMinutes)
	}
	if summary.LateMinutes != 0 || len(summary.Anomalies) != 1 || summary.Anomalies[0] != "Entrée sans sortie" {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestValidateExportColumnsRejectsUnknownColumn(t *testing.T) {
	t.Parallel()
	if _, err := attendance.ValidateExportColumns([]string{"employeeId", "secret"}); err == nil {
		t.Fatal("ValidateExportColumns() accepted an unknown column")
	}
}
