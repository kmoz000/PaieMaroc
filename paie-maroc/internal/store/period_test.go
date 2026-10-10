package store_test

import (
	"context"
	"testing"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/store"
)

func TestStorePeriodLifecycleAndCopy(t *testing.T) {
	t.Parallel()
	database := openStore(t)
	ctx := context.Background()
	employee := validEmployee("EMP-001", "1234567")
	if err := database.Upsert(ctx, employee); err != nil {
		t.Fatal(err)
	}
	period, err := database.CreatePeriod(ctx, store.CreatePeriodInput{Period: "2026-09"})
	if err != nil {
		t.Fatalf("CreatePeriod() error = %v", err)
	}
	if period.EmployeeCount != 1 {
		t.Fatalf("EmployeeCount = %d", period.EmployeeCount)
	}
	employee.GrossCentimes = 925_000
	employee.AdvanceCentimes = 50_000
	if err := database.SavePeriodEmployee(ctx, period.Period, employee); err != nil {
		t.Fatalf("SavePeriodEmployee() error = %v", err)
	}
	if err := database.ClosePeriod(ctx, period.Period); err != nil {
		t.Fatalf("ClosePeriod() error = %v", err)
	}
	if err := database.SavePeriodEmployee(ctx, period.Period, employee); err == nil {
		t.Fatal("SavePeriodEmployee() modified a closed period")
	}
	if err := database.ReopenPeriod(ctx, period.Period); err != nil {
		t.Fatalf("ReopenPeriod() error = %v", err)
	}
	copied, err := database.CreatePeriod(ctx, store.CreatePeriodInput{Period: "2026-10", CopyFrom: "2026-09"})
	if err != nil {
		t.Fatalf("CreatePeriod(copy) error = %v", err)
	}
	page, err := database.ListPeriodEmployees(ctx, copied.Period, "", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].GrossCentimes != 925_000 || page.Items[0].AdvanceCentimes != 0 {
		t.Fatalf("copied payroll = %+v", page.Items)
	}
	if err := database.DeletePeriod(ctx, "2026-09"); err != nil {
		t.Fatalf("DeletePeriod() error = %v", err)
	}
	remaining, err := database.GetPeriod(ctx, "2026-10")
	if err != nil || remaining.CopiedFrom != "" {
		t.Fatalf("remaining period = %+v, %v", remaining, err)
	}
}
