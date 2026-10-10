package payroll_test

import (
	"testing"

	"github.com/Niceblueman/PaieMaroc/paie-maroc/internal/payroll"
)

func TestCalculate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		employee         payroll.Employee
		wantCNSS         int64
		wantAMO          int64
		wantIncomeTax    int64
		wantNetPay       int64
		wantEmployerCost int64
	}{
		{
			name:             "salary below CNSS cap",
			employee:         validEmployee(500_000),
			wantCNSS:         22_400,
			wantAMO:          11_300,
			wantIncomeTax:    0,
			wantNetPay:       466_300,
			wantEmployerCost: 605_450,
		},
		{
			name:             "CNSS contribution is capped",
			employee:         validEmployee(1_000_000),
			wantCNSS:         26_880,
			wantAMO:          22_600,
			wantIncomeTax:    60_156,
			wantNetPay:       890_364,
			wantEmployerCost: 1_174_980,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := payroll.Calculate(tt.employee)
			if err != nil {
				t.Fatalf("Calculate() error = %v", err)
			}
			if got.EmployeeCNSS != tt.wantCNSS {
				t.Errorf("EmployeeCNSS = %d, want %d", got.EmployeeCNSS, tt.wantCNSS)
			}
			if got.EmployeeAMO != tt.wantAMO {
				t.Errorf("EmployeeAMO = %d, want %d", got.EmployeeAMO, tt.wantAMO)
			}
			if got.IncomeTax != tt.wantIncomeTax {
				t.Errorf("IncomeTax = %d, want %d", got.IncomeTax, tt.wantIncomeTax)
			}
			if got.NetPay != tt.wantNetPay {
				t.Errorf("NetPay = %d, want %d", got.NetPay, tt.wantNetPay)
			}
			if got.EmployerCost != tt.wantEmployerCost {
				t.Errorf("EmployerCost = %d, want %d", got.EmployerCost, tt.wantEmployerCost)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(*payroll.Employee)
		wantField string
	}{
		{
			name:      "invalid RIB length",
			mutate:    func(employee *payroll.Employee) { employee.RIB = "123" },
			wantField: "rib",
		},
		{
			name:      "invalid days",
			mutate:    func(employee *payroll.Employee) { employee.DaysWorked = 31 },
			wantField: "daysWorked",
		},
		{
			name:      "negative exempt amount",
			mutate:    func(employee *payroll.Employee) { employee.ExemptCentimes = -1 },
			wantField: "exemptCentimes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			employee := validEmployee(500_000)
			tt.mutate(&employee)
			issues := payroll.Validate(employee)
			if len(issues) == 0 {
				t.Fatal("Validate() returned no issue")
			}
			if issues[0].Field != tt.wantField {
				t.Errorf("first field = %q, want %q", issues[0].Field, tt.wantField)
			}
		})
	}
}

func TestCalculateBatchRejectsDuplicates(t *testing.T) {
	t.Parallel()

	first := validEmployee(500_000)
	second := validEmployee(600_000)
	second.FirstName = "Sara"
	second.CIN = "BE654321"
	second.CNSS = "7654321"

	_, issues := payroll.CalculateBatch([]payroll.Employee{first, second})
	if len(issues) != 2 {
		t.Fatalf("issues count = %d, want 2", len(issues))
	}
}

func validEmployee(gross int64) payroll.Employee {
	return payroll.Employee{
		ID:             "EMP-001",
		FirstName:      "Amine",
		LastName:       "Alaoui",
		CIN:            "AB123456",
		CNSS:           "1234567",
		RIB:            "011780000012345678901234",
		DaysWorked:     26,
		Dependants:     0,
		GrossCentimes:  gross,
		ExemptCentimes: 0,
	}
}
