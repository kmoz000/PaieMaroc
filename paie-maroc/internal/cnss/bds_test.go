package cnss_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/Niceblueman/PaieMaroc/paie-maroc/internal/cnss"
)

type memorySource struct {
	existing map[string]cnss.Declaration
	entrants []cnss.Declaration
}

func (source memorySource) Existing(number string) (cnss.Declaration, bool, error) {
	declaration, found := source.existing[number]
	return declaration, found, nil
}

func (source memorySource) WalkEntrants(yield func(cnss.Declaration) error) error {
	for _, declaration := range source.entrants {
		if err := yield(declaration); err != nil {
			return err
		}
	}
	return nil
}

func TestGenerateWithEntrant(t *testing.T) {
	t.Parallel()

	preEstablished := testPreEstablished()
	source := memorySource{
		existing: map[string]cnss.Declaration{
			"168764721": {Days: 26, GrossCentimes: 800_000, CappedGrossCentimes: 600_000},
		},
		entrants: []cnss.Declaration{
			{CNSS: "000000000", FullName: "SARA BENNANI", CIN: "BE123456", Days: 20, GrossCentimes: 500_000, CappedGrossCentimes: 500_000},
		},
	}
	var output bytes.Buffer
	totals, err := cnss.Generate(strings.NewReader(preEstablished), &output, source)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if totals.EmployeeCount != 2 || totals.EntrantCount != 1 {
		t.Fatalf("totals = %+v", totals)
	}
	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 7 {
		t.Fatalf("line count = %d, want 7", len(lines))
	}
	for index, line := range lines {
		if len(line) != 260 {
			t.Errorf("line %d length = %d", index+1, len(line))
		}
	}
	if got := lines[2][:3]; got != "B02" {
		t.Errorf("existing record = %q", got)
	}
	if got := lines[4][:3]; got != "B04" {
		t.Errorf("entrant record = %q", got)
	}
}

func TestGenerateRequiresEveryPreEstablishedEmployee(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	_, err := cnss.Generate(strings.NewReader(testPreEstablished()), &output, memorySource{existing: map[string]cnss.Declaration{}})
	if err == nil || !strings.Contains(err.Error(), "absent de la paie") {
		t.Fatalf("Generate() error = %v", err)
	}
}

func TestGenerateLeaverReversesFamilyAllowance(t *testing.T) {
	t.Parallel()

	source := memorySource{existing: map[string]cnss.Declaration{
		"168764721": {Situation: "SO"},
	}}
	var output bytes.Buffer
	_, err := cnss.Generate(strings.NewReader(testPreEstablished()), &output, source)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	lines := strings.Split(output.String(), "\n")
	if got := lines[2][105:111]; got != "030000" {
		t.Errorf("AF reversal = %q, want 030000", got)
	}
}

func TestGenerateRejectsSummaryOverflow(t *testing.T) {
	t.Parallel()
	source := memorySource{
		existing: map[string]cnss.Declaration{
			"168764721": {Days: 26, GrossCentimes: 10_000_000_000_000, CappedGrossCentimes: 600_000},
		},
	}
	var output bytes.Buffer
	_, err := cnss.Generate(strings.NewReader(testPreEstablished()), &output, source)
	if err == nil || !strings.Contains(err.Error(), "largeur CNSS") {
		t.Fatalf("Generate() error = %v, want width error", err)
	}
}

func testPreEstablished() string {
	a00 := "A00" + "00000000000001" + "A0" + strings.Repeat(" ", 241)
	a01 := "A01" + "1773341" + "202610" + fixed("ATLAS CONSEIL", 40) + fixed("SERVICES", 40) +
		fixed("1 RUE ATLAS", 120) + fixed("CASABLANCA", 20) + fixed("20000", 6) + "01" + "20261001" + "20261110"
	a02Prefix := "A02" + "1773341" + "202610" + "168764721" + fixed("AMINE ALAOUI", 60) + "02" + "030000" + "000000" + "030000"
	a02 := a02Prefix + strings.Repeat(" ", 155)
	a03 := "A03" + "1773341" + "202610" + "000001" + "000002" + "000000030000" + "000000000000" +
		"000000030000" + fmt.Sprintf("%015d", 168764721) + strings.Repeat(" ", 181)
	return strings.Join([]string{a00, a01, a02, a03}, "\n") + "\n"
}

func fixed(value string, width int) string {
	return value + strings.Repeat(" ", width-len(value))
}
