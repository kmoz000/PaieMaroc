package main

import (
	"context"
	"fmt"
	"os"
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

func TestGenerateCNSSBDSUsesOnlySelectedPeriodAndPreEstablishedFile(t *testing.T) {
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
	employees := []payroll.Employee{
		{ID: "EMP-0001", FirstName: "Amine", LastName: "Alaoui", CIN: "AB123456", CNSS: "168764721", RIB: "011780000012345678901234", DaysWorked: 26, GrossCentimes: 800_000},
		{ID: "EMP-0002", FirstName: "Sara", LastName: "Bennani", CIN: "BE123456", RIB: "022780000098765432109876", DaysWorked: 20, GrossCentimes: 500_000},
	}
	for _, employee := range employees {
		if err := app.SavePeriodEmployee("2026-10", employee); err != nil {
			t.Fatal(err)
		}
	}
	preEstablishedPath := t.TempDir() + "/DS_PREETABLI.txt"
	outputPath := t.TempDir() + "/DS_CNSS_2026-10.txt"
	if err := os.WriteFile(preEstablishedPath, []byte(appTestPreEstablished()), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := app.generateCNSSBDS("2026-10", preEstablishedPath, outputPath)
	if err != nil {
		t.Fatalf("generateCNSSBDS() error = %v", err)
	}
	if result.Imported != 2 || result.ExistingCount != 1 || result.EntrantCount != 1 || result.EmployeeCount != 2 {
		t.Fatalf("generateCNSSBDS() = %+v", result)
	}
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "B04") || strings.Contains(string(content), "A02") {
		t.Fatalf("generated BDS does not contain the expected converted records")
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

func appTestPreEstablished() string {
	fixed := func(value string, width int) string {
		return value + strings.Repeat(" ", width-len(value))
	}
	a00 := "A00" + "00000000000001" + "A0" + strings.Repeat(" ", 241)
	a01 := "A01" + "1773341" + "202610" + fixed("ATLAS CONSEIL", 40) + fixed("SERVICES", 40) +
		fixed("1 RUE ATLAS", 120) + fixed("CASABLANCA", 20) + fixed("20000", 6) + "01" + "20261001" + "20261110"
	a02 := "A02" + "1773341" + "202610" + "168764721" + fixed("AMINE ALAOUI", 60) +
		"02" + "030000" + "000000" + "030000" + strings.Repeat(" ", 155)
	a03 := "A03" + "1773341" + "202610" + "000001" + "000002" + "000000030000" +
		"000000000000" + "000000030000" + fmt.Sprintf("%015d", 168764721) + strings.Repeat(" ", 181)
	return strings.Join([]string{a00, a01, a02, a03}, "\n") + "\n"
}
