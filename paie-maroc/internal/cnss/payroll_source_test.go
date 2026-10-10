package cnss

import (
	"context"
	"strings"
	"testing"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/payroll"
)

func TestPayrollSourceInfersExistingEmployeesAndEntrants(t *testing.T) {
	t.Parallel()
	employees := []payroll.Employee{
		{ID: "EMP-001", FirstName: "Amine", LastName: "Alaoui", CIN: "AB123456", CNSS: "168764721", DaysWorked: 26, GrossCentimes: 800_000},
		{ID: "EMP-002", FirstName: "Sara", LastName: "Bennani", CIN: "BE123456", DaysWorked: 20, GrossCentimes: 500_000},
	}
	source, stats, err := OpenPayrollSource(context.Background(), t.TempDir()+"/cnss.db", employeeWalker(employees))
	if err != nil {
		t.Fatalf("OpenPayrollSource() error = %v", err)
	}
	defer source.Close()
	if stats.Imported != 2 {
		t.Fatalf("Imported = %d, want 2", stats.Imported)
	}
	existing, found, err := source.Existing("168764721")
	if err != nil || !found || existing.GrossCentimes != 800_000 {
		t.Fatalf("Existing() = %+v, %v, %v", existing, found, err)
	}
	var entrants []Declaration
	if err := source.WalkEntrants(func(declaration Declaration) error {
		entrants = append(entrants, declaration)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(entrants) != 1 || entrants[0].CNSS != "000000000" || entrants[0].FullName != "Sara Bennani" {
		t.Fatalf("entrants = %+v", entrants)
	}
}

func TestPayrollSourceRejectsDuplicateCNSSDuringMatching(t *testing.T) {
	t.Parallel()
	employees := []payroll.Employee{
		{ID: "EMP-001", CNSS: "168764721"},
		{ID: "EMP-002", CNSS: "168764721"},
	}
	source, _, err := OpenPayrollSource(context.Background(), t.TempDir()+"/cnss.db", employeeWalker(employees))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	_, _, err = source.Existing("168764721")
	if err == nil || !strings.Contains(err.Error(), "plusieurs salariés") {
		t.Fatalf("Existing() error = %v", err)
	}
}

func employeeWalker(employees []payroll.Employee) PayrollWalker {
	return func(yield func(payroll.Employee) error) error {
		for _, employee := range employees {
			if err := yield(employee); err != nil {
				return err
			}
		}
		return nil
	}
}
