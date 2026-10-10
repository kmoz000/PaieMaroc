// Package cnss implements the fixed-width CNSS e-BDS exchange contract.
package cnss

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	recordLength = 260
	monthlyCap   = int64(600_000)
)

// Declaration is the payroll data added to a CNSS pre-established employee.
type Declaration struct {
	CNSS                    string
	FullName                string
	CIN                     string
	Days                    int
	GrossCentimes           int64
	CappedGrossCentimes     int64
	Situation               string
	FamilyAllowanceReversal int64
}

// DataSource permits disk-backed lookups and entrant iteration for very large files.
type DataSource interface {
	Existing(cnss string) (Declaration, bool, error)
	WalkEntrants(yield func(Declaration) error) error
}

// Totals are returned after a complete, successful stream.
type Totals struct {
	ExistingCount  int64
	EntrantCount   int64
	EmployeeCount  int64
	Days           int64
	GrossCentimes  int64
	CappedCentimes int64
	Checksums      int64
}

type sectionTotals struct {
	count     int64
	cnss      int64
	days      int64
	gross     int64
	capped    int64
	checksum  int64
	children  int64
	afPay     int64
	afDeduct  int64
	afNet     int64
	afReverse int64
}

// Generate transforms a CNSS pre-established A00-A03 stream into B00-B06.
// It uses constant memory apart from the caller-provided DataSource.
func Generate(preEstablished io.Reader, output io.Writer, source DataSource) (Totals, error) {
	return generate(preEstablished, output, source, "")
}

// GenerateForPeriod also requires the YYYYMM period embedded in A01/A02/A03
// to match the active payroll period selected by the user.
func GenerateForPeriod(preEstablished io.Reader, output io.Writer, source DataSource, expectedPeriod string) (Totals, error) {
	return generate(preEstablished, output, source, expectedPeriod)
}

func generate(preEstablished io.Reader, output io.Writer, source DataSource, expectedPeriod string) (Totals, error) {
	if source == nil {
		return Totals{}, errors.New("source de paie absente")
	}
	if expectedPeriod != "" && (len(expectedPeriod) != 6 || !digits(expectedPeriod)) {
		return Totals{}, errors.New("la période e-BDS attendue doit respecter AAAAMM")
	}
	scanner := bufio.NewScanner(preEstablished)
	scanner.Buffer(make([]byte, 4*1024), recordLength+2)
	writer := bufio.NewWriterSize(output, 64*1024)

	var existing sectionTotals
	var affiliate string
	var period string
	var lastCNSS string
	seen := map[string]bool{"A00": false, "A01": false, "A03": false}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if len(line) != recordLength {
			return Totals{}, fmt.Errorf("ligne %d: longueur %d, attendu %d", lineNumber(seen, existing.count), len(line), recordLength)
		}
		if !isASCII(line) {
			return Totals{}, errors.New("le préétabli doit contenir uniquement des caractères ASCII")
		}

		switch line[:3] {
		case "A00":
			if seen["A00"] {
				return Totals{}, errors.New("plusieurs enregistrements A00")
			}
			seen["A00"] = true
			converted := "B00" + line[3:17] + "B0" + line[19:]
			if err := writeRecord(writer, converted); err != nil {
				return Totals{}, err
			}
		case "A01":
			if !seen["A00"] || seen["A01"] {
				return Totals{}, errors.New("ordre ou doublon A01 invalide")
			}
			seen["A01"] = true
			affiliate = line[3:10]
			period = line[10:16]
			if !digits(affiliate) || !digits(period) {
				return Totals{}, errors.New("l'affilié et la période A01 doivent être numériques")
			}
			if expectedPeriod != "" && period != expectedPeriod {
				return Totals{}, fmt.Errorf("le préétabli concerne %s, pas la période active %s", period, expectedPeriod)
			}
			if !validCheckNumber(affiliate) {
				return Totals{}, errors.New("la clé du numéro d'affilié A01 est invalide")
			}
			if err := writeRecord(writer, "B01"+line[3:]); err != nil {
				return Totals{}, err
			}
		case "A02":
			if !seen["A01"] || seen["A03"] {
				return Totals{}, errors.New("ordre A02 invalide")
			}
			if line[3:10] != affiliate || line[10:16] != period {
				return Totals{}, errors.New("affilié ou période incohérent dans A02")
			}
			cnssNumber := line[16:25]
			if !digits(cnssNumber) {
				return Totals{}, errors.New("le numéro CNSS A02 doit contenir 9 chiffres")
			}
			if !digits(line[85:105]) {
				return Totals{}, errors.New("les allocations familiales A02 doivent être numériques")
			}
			if lastCNSS != "" && cnssNumber < lastCNSS {
				return Totals{}, errors.New("les A02 ne sont pas triés par numéro CNSS")
			}
			lastCNSS = cnssNumber
			declaration, found, err := source.Existing(cnssNumber)
			if err != nil {
				return Totals{}, fmt.Errorf("recherche salarié %s: %w", cnssNumber, err)
			}
			if !found {
				return Totals{}, fmt.Errorf("salarié préétabli %s absent de la paie; utilisez SO s'il est sorti", cnssNumber)
			}
			record, row, err := existingRecord(line, declaration)
			if err != nil {
				return Totals{}, fmt.Errorf("salarié %s: %w", cnssNumber, err)
			}
			if err := writeRecord(writer, record); err != nil {
				return Totals{}, err
			}
			addTotals(&existing, row)
		case "A03":
			if !seen["A01"] || seen["A03"] {
				return Totals{}, errors.New("ordre ou doublon A03 invalide")
			}
			seen["A03"] = true
			if line[3:10] != affiliate || line[10:16] != period {
				return Totals{}, errors.New("affilié ou période incohérent dans A03")
			}
			if !digits(line[16:79]) {
				return Totals{}, errors.New("les totaux A03 doivent être numériques")
			}
			if parseNumeric(line[16:22]) != existing.count {
				return Totals{}, errors.New("le nombre A03 ne correspond pas aux lignes A02")
			}
			existing.children = parseNumeric(line[22:28])
			existing.afPay = parseNumeric(line[28:40])
			existing.afDeduct = parseNumeric(line[40:52])
			existing.afNet = parseNumeric(line[52:64])
			if parseNumeric(line[64:79]) != existing.cnss {
				return Totals{}, errors.New("le total des immatriculations A03 est incohérent")
			}
			record, err := existingSummary(line, existing)
			if err != nil {
				return Totals{}, err
			}
			if err := writeRecord(writer, record); err != nil {
				return Totals{}, err
			}
		default:
			return Totals{}, fmt.Errorf("type préétabli inconnu: %q", line[:3])
		}
	}
	if err := scanner.Err(); err != nil {
		return Totals{}, fmt.Errorf("lecture du préétabli: %w", err)
	}
	if !seen["A00"] || !seen["A01"] || !seen["A03"] {
		return Totals{}, errors.New("préétabli incomplet: A00, A01 et A03 sont obligatoires")
	}

	var entrants sectionTotals
	err := source.WalkEntrants(func(declaration Declaration) error {
		record, row, err := entrantRecord(affiliate, period, declaration)
		if err != nil {
			return err
		}
		if err := writeRecord(writer, record); err != nil {
			return err
		}
		addTotals(&entrants, row)
		return nil
	})
	if err != nil {
		return Totals{}, fmt.Errorf("salariés entrants: %w", err)
	}
	if entrants.count == 0 {
		empty := "B04" + affiliate + period + strings.Repeat(" ", 9+60+8) + strings.Repeat("0", 2+13+9+19) + strings.Repeat(" ", 124)
		if err := writeRecord(writer, empty); err != nil {
			return Totals{}, err
		}
	}
	entrantSummary, err := summaryRecord("B05", affiliate, period, entrants)
	if err != nil {
		return Totals{}, err
	}
	if err := writeRecord(writer, entrantSummary); err != nil {
		return Totals{}, err
	}
	global := mergeTotals(existing, entrants)
	globalSummary, err := summaryRecord("B06", affiliate, period, global)
	if err != nil {
		return Totals{}, err
	}
	if err := writeRecord(writer, globalSummary); err != nil {
		return Totals{}, err
	}
	if err := writer.Flush(); err != nil {
		return Totals{}, fmt.Errorf("finalisation BDS: %w", err)
	}

	return Totals{
		ExistingCount:  existing.count,
		EntrantCount:   entrants.count,
		EmployeeCount:  global.count,
		Days:           global.days,
		GrossCentimes:  global.gross,
		CappedCentimes: global.capped,
		Checksums:      global.checksum,
	}, nil
}

func existingRecord(preEstablished string, declaration Declaration) (string, sectionTotals, error) {
	declaration.CNSS = preEstablished[16:25]
	if err := validateDeclaration(declaration, false); err != nil {
		return "", sectionTotals{}, err
	}
	afReverse := declaration.FamilyAllowanceReversal
	afNet := parseNumeric(preEstablished[99:105])
	if declaration.Situation == "SO" || declaration.Situation == "DE" {
		afReverse = afNet
	}
	if afReverse > afNet {
		return "", sectionTotals{}, errors.New("l'AF à reverser dépasse l'AF nette du préétabli")
	}
	cnssNumber := parseNumeric(declaration.CNSS)
	checksum := cnssNumber + afReverse + int64(declaration.Days) + declaration.GrossCentimes + declaration.CappedGrossCentimes + situationRank(declaration.Situation)
	if err := validateWidths(map[string]numericField{
		"AF à reverser": {afReverse, 6}, "jours": {int64(declaration.Days), 2},
		"salaire réel": {declaration.GrossCentimes, 13}, "salaire plafonné": {declaration.CappedGrossCentimes, 9},
		"somme de contrôle": {checksum, 19},
	}); err != nil {
		return "", sectionTotals{}, err
	}
	record := "B02" + preEstablished[3:105] +
		numeric(afReverse, 6) +
		numeric(int64(declaration.Days), 2) +
		numeric(declaration.GrossCentimes, 13) +
		numeric(declaration.CappedGrossCentimes, 9) +
		fixed(declaration.Situation, 2) +
		numeric(checksum, 19) +
		strings.Repeat(" ", 104)
	return record, sectionTotals{
		count: 1, cnss: cnssNumber, days: int64(declaration.Days), gross: declaration.GrossCentimes,
		capped: declaration.CappedGrossCentimes, checksum: checksum, afReverse: afReverse,
	}, nil
}

func existingSummary(preEstablished string, totals sectionTotals) (string, error) {
	if err := validateSummaryWidths(totals, true); err != nil {
		return "", err
	}
	record := "B03" + preEstablished[3:79] +
		numeric(totals.afReverse, 12) +
		numeric(totals.days, 6) +
		numeric(totals.gross, 15) +
		numeric(totals.capped, 13) +
		numeric(totals.checksum, 19) +
		strings.Repeat(" ", 116)
	if len(record) != recordLength {
		return "", errors.New("erreur interne de longueur B03")
	}
	return record, nil
}

func entrantRecord(affiliate, period string, declaration Declaration) (string, sectionTotals, error) {
	if declaration.CNSS == "" {
		declaration.CNSS = "000000000"
	}
	if err := validateDeclaration(declaration, true); err != nil {
		return "", sectionTotals{}, err
	}
	cnssNumber := parseNumeric(declaration.CNSS)
	checksum := cnssNumber + int64(declaration.Days) + declaration.GrossCentimes + declaration.CappedGrossCentimes
	if err := validateWidths(map[string]numericField{
		"jours entrant": {int64(declaration.Days), 2}, "salaire réel entrant": {declaration.GrossCentimes, 13},
		"salaire plafonné entrant": {declaration.CappedGrossCentimes, 9}, "somme de contrôle entrant": {checksum, 19},
	}); err != nil {
		return "", sectionTotals{}, err
	}
	record := "B04" + affiliate + period +
		numericText(declaration.CNSS, 9) +
		fixed(declaration.FullName, 60) +
		fixed(declaration.CIN, 8) +
		numeric(int64(declaration.Days), 2) +
		numeric(declaration.GrossCentimes, 13) +
		numeric(declaration.CappedGrossCentimes, 9) +
		numeric(checksum, 19) +
		strings.Repeat(" ", 124)
	return record, sectionTotals{
		count: 1, cnss: cnssNumber, days: int64(declaration.Days), gross: declaration.GrossCentimes,
		capped: declaration.CappedGrossCentimes, checksum: checksum,
	}, nil
}

func summaryRecord(kind, affiliate, period string, totals sectionTotals) (string, error) {
	if err := validateSummaryWidths(totals, false); err != nil {
		return "", err
	}
	return kind + affiliate + period +
		numeric(totals.count, 6) +
		numeric(totals.cnss, 15) +
		numeric(totals.days, 6) +
		numeric(totals.gross, 15) +
		numeric(totals.capped, 13) +
		numeric(totals.checksum, 19) +
		strings.Repeat(" ", 170), nil
}

type numericField struct {
	value int64
	width int
}

func validateSummaryWidths(totals sectionTotals, existing bool) error {
	fields := map[string]numericField{
		"nombre de salariés": {totals.count, 6}, "total immatriculations": {totals.cnss, 15},
		"total jours": {totals.days, 6}, "total salaires réels": {totals.gross, 15},
		"total salaires plafonnés": {totals.capped, 13}, "total contrôles": {totals.checksum, 19},
	}
	if existing {
		fields["total AF à reverser"] = numericField{totals.afReverse, 12}
	}
	return validateWidths(fields)
}

func validateWidths(fields map[string]numericField) error {
	for name, field := range fields {
		if field.value < 0 || len(strconv.FormatInt(field.value, 10)) > field.width {
			return fmt.Errorf("%s dépasse la largeur CNSS de %d chiffres", name, field.width)
		}
	}
	return nil
}

func validateDeclaration(declaration Declaration, entrant bool) error {
	validSituation := map[string]bool{"": true, "SO": true, "DE": true, "IT": true, "IL": true, "AT": true, "CS": true, "MS": true, "MP": true}
	if !entrant && !validSituation[declaration.Situation] {
		return fmt.Errorf("code situation invalide: %q", declaration.Situation)
	}
	if entrant && declaration.Situation != "" {
		return errors.New("un entrant B04 ne porte pas de code situation")
	}
	if declaration.Days < 0 || declaration.Days > 26 {
		return errors.New("le nombre de jours doit être compris entre 0 et 26")
	}
	if declaration.GrossCentimes < 0 || declaration.CappedGrossCentimes < 0 {
		return errors.New("les salaires ne peuvent pas être négatifs")
	}
	if declaration.CappedGrossCentimes > declaration.GrossCentimes {
		return errors.New("le salaire plafonné dépasse le salaire réel")
	}
	if declaration.CNSS != "999999999" && declaration.CappedGrossCentimes > monthlyCap {
		return errors.New("le salaire plafonné dépasse 6 000 MAD")
	}
	if declaration.Situation == "CS" || declaration.Situation == "MS" {
		if declaration.Days != 0 || declaration.GrossCentimes != 0 || declaration.CappedGrossCentimes != 0 {
			return errors.New("CS et MS exigent zéro jour et zéro salaire")
		}
	}
	if declaration.Situation == "" && declaration.CNSS != "999999999" {
		if declaration.Days == 0 || declaration.GrossCentimes == 0 || declaration.CappedGrossCentimes == 0 {
			return errors.New("une situation normale exige jours et salaires renseignés")
		}
	}
	if entrant {
		if declaration.CNSS == "000000000" && (strings.TrimSpace(declaration.FullName) == "" || strings.TrimSpace(declaration.CIN) == "") {
			return errors.New("un entrant sans numéro CNSS exige le nom complet et le CIN")
		}
		if declaration.CNSS == "999999999" && declaration.Days != 0 {
			return errors.New("la main-d'œuvre occasionnelle est déclarée sans jours")
		}
		if declaration.CNSS != "000000000" && declaration.CNSS != "999999999" && !validCNSS(declaration.CNSS) {
			return errors.New("clé du numéro CNSS entrant invalide")
		}
	}
	return nil
}

func validCNSS(value string) bool {
	return len(value) == 9 && validCheckNumber(value)
}

func validCheckNumber(value string) bool {
	if len(value) < 2 || !digits(value) {
		return false
	}
	values := make([]int, len(value))
	for index, character := range value {
		values[index] = int(character - '0')
	}
	sum := 0
	for index := 0; index < len(values)-1; index++ {
		weight := 1
		if index%2 == 1 {
			weight = 2
		}
		sum += values[index] * weight
	}
	check := (10 - sum%10) % 10
	return check == values[len(values)-1]
}

func situationRank(situation string) int64 {
	ranks := map[string]int64{"": 0, "SO": 1, "DE": 2, "IT": 3, "IL": 4, "AT": 5, "CS": 6, "MS": 7, "MP": 8}
	return ranks[situation]
}

func addTotals(total *sectionTotals, row sectionTotals) {
	total.count += row.count
	total.cnss += row.cnss
	total.days += row.days
	total.gross += row.gross
	total.capped += row.capped
	total.checksum += row.checksum
	total.afReverse += row.afReverse
}

func mergeTotals(first, second sectionTotals) sectionTotals {
	addTotals(&first, second)
	return first
}

func writeRecord(writer *bufio.Writer, record string) error {
	if len(record) != recordLength {
		return fmt.Errorf("erreur interne: enregistrement %s de %d caractères", record[:3], len(record))
	}
	if !isASCII(record) {
		return fmt.Errorf("enregistrement %s contient un caractère non ASCII", record[:3])
	}
	if _, err := writer.WriteString(record + "\n"); err != nil {
		return fmt.Errorf("écriture %s: %w", record[:3], err)
	}
	return nil
}

func numeric(value int64, width int) string {
	text := strconv.FormatInt(value, 10)
	if len(text) > width {
		return strings.Repeat("9", width)
	}
	return strings.Repeat("0", width-len(text)) + text
}

func numericText(value string, width int) string {
	if len(value) > width {
		value = value[len(value)-width:]
	}
	return strings.Repeat("0", width-len(value)) + value
}

func fixed(value string, width int) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) > width {
		value = value[:width]
	}
	return value + strings.Repeat(" ", width-len(value))
}

func parseNumeric(value string) int64 {
	number, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return number
}

func isASCII(value string) bool {
	for _, character := range value {
		if character < 9 || character > 126 || (character > 13 && character < 32) {
			return false
		}
	}
	return true
}

func digits(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func lineNumber(seen map[string]bool, details int64) int64 {
	line := details + 1
	if seen["A00"] {
		line++
	}
	if seen["A01"] {
		line++
	}
	return line
}
