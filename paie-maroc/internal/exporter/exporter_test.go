package exporter_test

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/exporter"
	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/payroll"
)

func TestGenerateSIMT500(t *testing.T) {
	t.Parallel()

	file, err := exporter.Generate(validRequest(exporter.FormatSIMT500))
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(file.Content), "\r\n"), "\r\n")
	if len(lines) != 4 {
		t.Fatalf("line count = %d, want 4", len(lines))
	}
	for index, line := range lines {
		if len(line) != 500 {
			t.Errorf("line %d length = %d, want 500", index+1, len(line))
		}
	}
	if !strings.HasPrefix(lines[0], "1000000000000000701120261010091500MAD232") {
		t.Errorf("unexpected header prefix: %q", lines[0][:40])
	}
	if got := lines[3][:27]; got != "110000200000000000001211500" {
		t.Errorf("footer control fields = %q", got)
	}
}

func TestGenerateCSVPreventsFormulaInjection(t *testing.T) {
	t.Parallel()

	request := validRequest(exporter.FormatCSV)
	request.Payroll.Rows[0].Employee.FirstName = "=cmd"
	file, err := exporter.Generate(request)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !strings.Contains(string(file.Content), "'=cmd") {
		t.Fatal("expected formula-like employee name to be escaped")
	}
}

func TestGenerateISO20022IsWellFormed(t *testing.T) {
	t.Parallel()

	file, err := exporter.Generate(validRequest(exporter.FormatISO20022))
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	var value any
	if err := xml.Unmarshal(file.Content, &value); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}
	if !strings.Contains(string(file.Content), "12115.00") {
		t.Fatal("control sum missing")
	}
}

func TestGenerateRejectsControlTotalMismatch(t *testing.T) {
	t.Parallel()

	request := validRequest(exporter.FormatSIMT500)
	request.Payroll.NetTotalCentimes++
	_, err := exporter.Generate(request)
	if err == nil {
		t.Fatal("Generate() error = nil, want control total error")
	}
}

func validRequest(format exporter.Format) exporter.Request {
	rows := []payroll.Result{
		{
			Employee: payroll.Employee{
				ID: "EMP-001", FirstName: "Amine", LastName: "Alaoui", CNSS: "1234567",
				CIN: "AB123456", RIB: "011780000012345678901234", DaysWorked: 26, GrossCentimes: 700_000,
			},
			NetPay: 621_500,
		},
		{
			Employee: payroll.Employee{
				ID: "EMP-002", FirstName: "Sara", LastName: "Bennani", CNSS: "7654321",
				CIN: "BE654321", RIB: "022780000098765432109876", DaysWorked: 26, GrossCentimes: 700_000,
			},
			NetPay: 590_000,
		},
	}
	return exporter.Request{
		Format: format,
		Company: exporter.Company{
			Name: "Atlas Conseil", AffiliateCNSS: "1234567", RIB: "011780000000000000000123", Participant: "011",
		},
		Payroll:     payroll.BatchResult{Rows: rows, NetTotalCentimes: 1_211_500, GrossTotalCentimes: 1_400_000},
		Period:      "2026-10",
		ExecutionAt: time.Date(2026, 10, 10, 9, 15, 0, 0, time.UTC),
		BatchNumber: 7,
	}
}
