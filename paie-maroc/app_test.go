package main

import (
	"context"
	"strings"
	"testing"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/auth"
	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/payroll"
	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/store"
)

func TestAppEmployeeLifecycleThroughWailsBoundary(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)

	state, err := app.CreateInitialAdmin(auth.CreateInput{
		Username: "admin.paie", DisplayName: "Admin Paie", Password: "phrase secrète robuste",
	})
	if err != nil || !state.Authenticated || state.User == nil {
		t.Fatalf("CreateInitialAdmin() = %+v, %v", state, err)
	}
	if _, err := app.CreatePeriod(store.CreatePeriodInput{Period: "2026-10"}); err != nil {
		t.Fatalf("CreatePeriod() error = %v", err)
	}

	employee := payroll.Employee{
		ID: "EMP-0001", FirstName: "Nadia", LastName: "Idrissi", CIN: "AB123456",
		CNSS: "123456789", RIB: "011780000012345678901234", DaysWorked: 26,
		Dependants: 1, GrossCentimes: 800_000,
	}
	if err := app.SavePeriodEmployee("2026-10", employee); err != nil {
		t.Fatalf("SavePeriodEmployee(new) error = %v", err)
	}
	page, err := app.ListPeriodEmployees("2026-10", "", "", 50)
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != employee.ID {
		t.Fatalf("ListPeriodEmployees() = %+v, %v", page, err)
	}

	employee.GrossCentimes = 925_000
	if err := app.SavePeriodEmployee("2026-10", employee); err != nil {
		t.Fatalf("SavePeriodEmployee(edit) error = %v", err)
	}
	batch, err := app.CalculatePayroll([]payroll.Employee{employee})
	if err != nil || len(batch.Rows) != 1 || batch.GrossTotalCentimes != 925_000 {
		t.Fatalf("CalculatePayroll() = %+v, %v", batch, err)
	}
	if err := app.ClosePeriod("2026-10"); err != nil {
		t.Fatalf("ClosePeriod() error = %v", err)
	}
	if err := app.SavePeriodEmployee("2026-10", employee); err == nil {
		t.Fatal("SavePeriodEmployee() modified a closed period")
	}
	if err := app.ReopenPeriod("2026-10"); err != nil {
		t.Fatalf("ReopenPeriod() error = %v", err)
	}
	if err := app.DeletePeriod("2026-10"); err != nil {
		t.Fatalf("DeletePeriod() error = %v", err)
	}
}

func TestAppEmployeeValidationReturnsFieldCode(t *testing.T) {
	t.Parallel()
	app := newTestApp(t)
	if _, err := app.CreateInitialAdmin(auth.CreateInput{
		Username: "admin", DisplayName: "Admin Paie", Password: "phrase secrète robuste",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.CreatePeriod(store.CreatePeriodInput{Period: "2026-10"}); err != nil {
		t.Fatal(err)
	}
	err := app.SavePeriodEmployee("2026-10", payroll.Employee{ID: "EMP-0001"})
	if err == nil || !strings.HasPrefix(err.Error(), "firstName:") {
		t.Fatalf("SavePeriodEmployee() error = %v", err)
	}
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	database, err := store.Open(t.TempDir() + "/paie-maroc.db")
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return &App{ctx: context.Background(), store: database}
}
