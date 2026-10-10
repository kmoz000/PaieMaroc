// Package payroll contains the deterministic Moroccan payroll calculation core.
package payroll

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	RulesVersion = "MA-2026.1"
	cnssCap      = int64(600_000)
)

var (
	digitsOnly = regexp.MustCompile(`^[0-9]+$`)
	validCIN   = regexp.MustCompile(`^[A-Z]{1,2}[0-9]{3,8}$`)
)

// Employee is the validated monthly payroll input. Monetary values are centimes.
type Employee struct {
	ID              string `json:"id"`
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	CIN             string `json:"cin"`
	CNSS            string `json:"cnss"`
	RIB             string `json:"rib"`
	DaysWorked      int    `json:"daysWorked"`
	Dependants      int    `json:"dependants"`
	GrossCentimes   int64  `json:"grossCentimes"`
	ExemptCentimes  int64  `json:"exemptCentimes"`
	AdvanceCentimes int64  `json:"advanceCentimes"`
}

// Result exposes every intermediate amount used to obtain net pay.
type Result struct {
	Employee            Employee `json:"employee"`
	TaxableGross        int64    `json:"taxableGross"`
	EmployeeCNSS        int64    `json:"employeeCnss"`
	EmployeeAMO         int64    `json:"employeeAmo"`
	ProfessionalExpense int64    `json:"professionalExpense"`
	IncomeTax           int64    `json:"incomeTax"`
	NetPay              int64    `json:"netPay"`
	EmployerCNSS        int64    `json:"employerCnss"`
	EmployerAMO         int64    `json:"employerAmo"`
	FamilyAllowance     int64    `json:"familyAllowance"`
	TrainingTax         int64    `json:"trainingTax"`
	EmployerCost        int64    `json:"employerCost"`
	RulesVersion        string   `json:"rulesVersion"`
}

// BatchResult contains calculations and control totals.
type BatchResult struct {
	Rows                 []Result `json:"rows"`
	GrossTotalCentimes   int64    `json:"grossTotalCentimes"`
	NetTotalCentimes     int64    `json:"netTotalCentimes"`
	EmployeeSocialTotal  int64    `json:"employeeSocialTotal"`
	IncomeTaxTotal       int64    `json:"incomeTaxTotal"`
	EmployerChargesTotal int64    `json:"employerChargesTotal"`
	EmployerCostCentimes int64    `json:"employerCostCentimes"`
	RulesVersion         string   `json:"rulesVersion"`
}

// ValidationIssue points to a field that must be corrected before export.
type ValidationIssue struct {
	EmployeeID string `json:"employeeId"`
	Field      string `json:"field"`
	Message    string `json:"message"`
}

// Validate checks boundary data before it enters payroll or export logic.
func Validate(employee Employee) []ValidationIssue {
	issues := []ValidationIssue{}
	add := func(field, message string) {
		issues = append(issues, ValidationIssue{
			EmployeeID: employee.ID,
			Field:      field,
			Message:    message,
		})
	}

	if strings.TrimSpace(employee.ID) == "" {
		add("id", "Le matricule interne est obligatoire.")
	}
	if strings.TrimSpace(employee.FirstName) == "" {
		add("firstName", "Le prénom est obligatoire.")
	}
	if strings.TrimSpace(employee.LastName) == "" {
		add("lastName", "Le nom est obligatoire.")
	}

	cin := strings.ToUpper(strings.ReplaceAll(employee.CIN, " ", ""))
	if !validCIN.MatchString(cin) {
		add("cin", "Le CIN doit contenir 1 à 2 lettres puis 3 à 8 chiffres.")
	}

	cnss := compactDigits(employee.CNSS)
	if len(cnss) < 6 || len(cnss) > 12 || !digitsOnly.MatchString(cnss) {
		add("cnss", "Le numéro CNSS doit contenir 6 à 12 chiffres.")
	}

	rib := compactDigits(employee.RIB)
	if len(rib) != 24 || !digitsOnly.MatchString(rib) {
		add("rib", "Le RIB marocain doit contenir exactement 24 chiffres.")
	}
	if employee.DaysWorked < 0 || employee.DaysWorked > 30 {
		add("daysWorked", "Le nombre de jours doit être compris entre 0 et 30.")
	}
	if employee.Dependants < 0 || employee.Dependants > 6 {
		add("dependants", "Le nombre de personnes à charge doit être compris entre 0 et 6.")
	}
	if employee.GrossCentimes <= 0 {
		add("grossCentimes", "Le salaire brut doit être supérieur à zéro.")
	}
	if employee.ExemptCentimes < 0 || employee.ExemptCentimes > employee.GrossCentimes {
		add("exemptCentimes", "Les éléments exonérés doivent être compris entre zéro et le brut.")
	}
	if employee.AdvanceCentimes < 0 {
		add("advanceCentimes", "L'avance ne peut pas être négative.")
	}

	return issues
}

// Calculate returns the monthly payroll result using the MA-2026.1 ruleset.
func Calculate(employee Employee) (Result, error) {
	if issues := Validate(employee); len(issues) > 0 {
		return Result{}, fmt.Errorf("invalid employee %q: %s", employee.ID, issues[0].Message)
	}

	taxableGross := employee.GrossCentimes - employee.ExemptCentimes
	cnssBase := min(taxableGross, cnssCap)
	employeeCNSS := percentage(cnssBase, 448)
	employeeAMO := percentage(taxableGross, 226)
	professionalExpense := monthlyProfessionalExpense(taxableGross)
	taxableAnnual := max(
		int64(0),
		(taxableGross-employeeCNSS-employeeAMO-professionalExpense)*12,
	)
	incomeTaxAnnual := annualIncomeTax(taxableAnnual)
	familyReliefAnnual := int64(min(employee.Dependants, 6)) * 50_000
	incomeTax := max(int64(0), roundDiv(incomeTaxAnnual-familyReliefAnnual, 12))

	employerCNSS := percentage(cnssBase, 898)
	// Employer AMO combines the 2.26% share and 1.85% employer-only increase.
	employerAMO := percentage(taxableGross, 411)
	familyAllowance := percentage(taxableGross, 640)
	trainingTax := percentage(taxableGross, 160)
	employerCharges := employerCNSS + employerAMO + familyAllowance + trainingTax
	netPay := employee.GrossCentimes - employeeCNSS - employeeAMO - incomeTax - employee.AdvanceCentimes
	if netPay < 0 {
		return Result{}, errors.New("les retenues dépassent le salaire brut")
	}

	return Result{
		Employee:            normalizedEmployee(employee),
		TaxableGross:        taxableGross,
		EmployeeCNSS:        employeeCNSS,
		EmployeeAMO:         employeeAMO,
		ProfessionalExpense: professionalExpense,
		IncomeTax:           incomeTax,
		NetPay:              netPay,
		EmployerCNSS:        employerCNSS,
		EmployerAMO:         employerAMO,
		FamilyAllowance:     familyAllowance,
		TrainingTax:         trainingTax,
		EmployerCost:        employee.GrossCentimes + employerCharges,
		RulesVersion:        RulesVersion,
	}, nil
}

// CalculateBatch validates the complete set and rejects duplicate identifiers.
func CalculateBatch(employees []Employee) (BatchResult, []ValidationIssue) {
	result := BatchResult{
		Rows:         []Result{},
		RulesVersion: RulesVersion,
	}
	issues := []ValidationIssue{}
	seenIDs := map[string]struct{}{}
	seenRIBs := map[string]string{}

	for _, employee := range employees {
		issues = append(issues, Validate(employee)...)

		id := strings.TrimSpace(employee.ID)
		if _, exists := seenIDs[id]; exists && id != "" {
			issues = append(issues, ValidationIssue{
				EmployeeID: id,
				Field:      "id",
				Message:    "Ce matricule est utilisé plusieurs fois.",
			})
		}
		seenIDs[id] = struct{}{}

		rib := compactDigits(employee.RIB)
		if owner, exists := seenRIBs[rib]; exists && rib != "" {
			issues = append(issues, ValidationIssue{
				EmployeeID: id,
				Field:      "rib",
				Message:    fmt.Sprintf("Ce RIB est aussi utilisé par %s.", owner),
			})
		}
		seenRIBs[rib] = id
	}
	if len(issues) > 0 {
		return result, issues
	}

	for _, employee := range employees {
		row, err := Calculate(employee)
		if err != nil {
			issues = append(issues, ValidationIssue{
				EmployeeID: employee.ID,
				Field:      "grossCentimes",
				Message:    err.Error(),
			})
			continue
		}
		result.Rows = append(result.Rows, row)
		result.GrossTotalCentimes += employee.GrossCentimes
		result.NetTotalCentimes += row.NetPay
		result.EmployeeSocialTotal += row.EmployeeCNSS + row.EmployeeAMO
		result.IncomeTaxTotal += row.IncomeTax
		result.EmployerChargesTotal += row.EmployerCNSS + row.EmployerAMO + row.FamilyAllowance + row.TrainingTax
		result.EmployerCostCentimes += row.EmployerCost
	}

	return result, issues
}

func annualIncomeTax(taxableAnnual int64) int64 {
	switch {
	case taxableAnnual <= 4_000_000:
		return 0
	case taxableAnnual <= 6_000_000:
		return percentage(taxableAnnual, 1_000) - 400_000
	case taxableAnnual <= 8_000_000:
		return percentage(taxableAnnual, 2_000) - 1_000_000
	case taxableAnnual <= 10_000_000:
		return percentage(taxableAnnual, 3_000) - 1_800_000
	case taxableAnnual <= 18_000_000:
		return percentage(taxableAnnual, 3_400) - 2_200_000
	default:
		return percentage(taxableAnnual, 3_700) - 2_740_000
	}
}

func monthlyProfessionalExpense(taxableGross int64) int64 {
	annualGross := taxableGross * 12
	rateBasisPoints := int64(3_500)
	if annualGross > 7_800_000 {
		rateBasisPoints = 2_500
	}
	annualExpense := min(percentage(annualGross, rateBasisPoints), 3_500_000)
	return roundDiv(annualExpense, 12)
}

func percentage(value, basisPoints int64) int64 {
	whole := value / 10_000
	remainder := value % 10_000
	return whole*basisPoints + roundDiv(remainder*basisPoints, 10_000)
}

func roundDiv(value, divisor int64) int64 {
	if value < 0 {
		return -roundDiv(-value, divisor)
	}
	return (value + divisor/2) / divisor
}

func normalizedEmployee(employee Employee) Employee {
	employee.ID = strings.TrimSpace(employee.ID)
	employee.FirstName = strings.TrimSpace(employee.FirstName)
	employee.LastName = strings.TrimSpace(employee.LastName)
	employee.CIN = strings.ToUpper(strings.ReplaceAll(employee.CIN, " ", ""))
	employee.CNSS = compactDigits(employee.CNSS)
	employee.RIB = compactDigits(employee.RIB)
	return employee
}

func compactDigits(value string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", ".", "")
	return replacer.Replace(strings.TrimSpace(value))
}
