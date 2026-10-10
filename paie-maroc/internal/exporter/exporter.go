// Package exporter produces deterministic, locally generated payroll files.
package exporter

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/payroll"
)

// Format identifies an export contract.
type Format string

const (
	FormatSIMT500     Format = "simt500"
	FormatCSV         Format = "csv"
	FormatISO20022    Format = "iso20022"
	FormatCNSSControl Format = "cnss-control"
)

// Company contains the ordering party data required by payment exports.
type Company struct {
	Name          string `json:"name"`
	AffiliateCNSS string `json:"affiliateCnss"`
	RIB           string `json:"rib"`
	Participant   string `json:"participant"`
}

// Request is a fully specified, reproducible export operation.
type Request struct {
	Format      Format              `json:"format"`
	Company     Company             `json:"company"`
	Payroll     payroll.BatchResult `json:"payroll"`
	Period      string              `json:"period"`
	ExecutionAt time.Time           `json:"executionAt"`
	BatchNumber int                 `json:"batchNumber"`
}

// File is the generated artifact and its control metadata.
type File struct {
	Name          string `json:"name"`
	MediaType     string `json:"mediaType"`
	Content       []byte `json:"content"`
	RecordCount   int    `json:"recordCount"`
	TotalCentimes int64  `json:"totalCentimes"`
	Contract      string `json:"contract"`
}

// Generate validates common data and dispatches to the selected format.
func Generate(request Request) (File, error) {
	if err := validateRequest(request); err != nil {
		return File{}, err
	}

	switch request.Format {
	case FormatSIMT500:
		return generateSIMT500(request)
	case FormatCSV:
		return generateCSV(request)
	case FormatISO20022:
		return generateISO20022(request)
	case FormatCNSSControl:
		return generateCNSSControl(request)
	default:
		return File{}, fmt.Errorf("format d'export non pris en charge: %q", request.Format)
	}
}

func validateRequest(request Request) error {
	if len(request.Payroll.Rows) == 0 {
		return errors.New("aucun salarié à exporter")
	}
	if len(compactDigits(request.Company.RIB)) != 24 {
		return errors.New("le RIB de l'entreprise doit contenir 24 chiffres")
	}
	if request.Period == "" {
		return errors.New("la période de paie est obligatoire")
	}
	if request.ExecutionAt.IsZero() {
		return errors.New("la date d'exécution est obligatoire")
	}
	if request.BatchNumber < 0 || request.BatchNumber > 9999 {
		return errors.New("le numéro de lot doit être compris entre 0 et 9999")
	}

	var total int64
	for _, row := range request.Payroll.Rows {
		if len(compactDigits(row.Employee.RIB)) != 24 {
			return fmt.Errorf("RIB invalide pour %s", row.Employee.ID)
		}
		if row.NetPay <= 0 {
			return fmt.Errorf("net à payer invalide pour %s", row.Employee.ID)
		}
		total += row.NetPay
	}
	if total != request.Payroll.NetTotalCentimes {
		return fmt.Errorf(
			"total de contrôle incohérent: lignes=%d, lot=%d",
			total,
			request.Payroll.NetTotalCentimes,
		)
	}

	return nil
}

func generateSIMT500(request Request) (File, error) {
	issuerRIB := compactDigits(request.Company.RIB)
	participant := leftPadDigits(request.Company.Participant, 3)
	if participant == "000" {
		participant = issuerRIB[:3]
	}
	date := request.ExecutionAt.Format("20060102")
	timeValue := request.ExecutionAt.Format("150405")

	header := strings.Join([]string{
		"10",
		"00000",
		"000000",
		leftPadDigits(strconv.Itoa(request.BatchNumber), 4),
		participant,
		date,
		timeValue,
		"MAD2",
		"32",
		strings.Repeat(" ", 460),
	}, "")

	lines := []string{header}
	for index, row := range request.Payroll.Rows {
		beneficiaryRIB := compactDigits(row.Employee.RIB)
		beneficiaryName := row.Employee.FirstName + " " + row.Employee.LastName
		reference := fmt.Sprintf("%s-%04d", request.Period, index+1)
		line := strings.Join([]string{
			"04",
			"020",
			issuerRIB[:3],
			beneficiaryRIB[:3],
			issuerRIB[3:6],
			beneficiaryRIB[3:6],
			strings.Repeat(" ", 26),
			"MAD2",
			leftPadDigits(strconv.FormatInt(row.NetPay, 10), 16),
			strings.Repeat(" ", 37),
			"00",
			strings.Repeat(" ", 34),
			"1",
			fixedText(request.Company.Name, 35),
			fixedText(beneficiaryName, 35),
			issuerRIB,
			beneficiaryRIB,
			"00000000",
			fixedText("SALAIRE "+request.Period, 35),
			fixedText("PAIE "+request.Period, 35),
			strings.Repeat(" ", 140),
			fixedText(reference, 24),
			"MA",
			"N",
		}, "")
		if len(line) != 500 {
			return File{}, fmt.Errorf("erreur interne: ligne SIMT de %d caractères", len(line))
		}
		lines = append(lines, line)
	}

	footer := "11" +
		leftPadDigits(strconv.Itoa(len(request.Payroll.Rows)), 5) +
		leftPadDigits(strconv.FormatInt(request.Payroll.NetTotalCentimes, 10), 20) +
		strings.Repeat(" ", 473)
	lines = append(lines, footer)

	content := []byte(strings.Join(lines, "\r\n") + "\r\n")
	return File{
		Name:          "PAIE_SIMT500_" + request.Period + ".txt",
		MediaType:     "text/plain; charset=us-ascii",
		Content:       content,
		RecordCount:   len(request.Payroll.Rows),
		TotalCentimes: request.Payroll.NetTotalCentimes,
		Contract:      "SIMT500 — enregistrements fixes de 500 caractères",
	}, nil
}

func generateCSV(request Request) (File, error) {
	var buffer bytes.Buffer
	buffer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buffer)
	writer.Comma = ';'
	writer.UseCRLF = true
	if err := writer.Write([]string{"Matricule", "Bénéficiaire", "RIB", "Montant MAD", "Motif"}); err != nil {
		return File{}, fmt.Errorf("écriture entête CSV: %w", err)
	}
	for _, row := range request.Payroll.Rows {
		record := []string{
			safeSpreadsheetCell(row.Employee.ID),
			safeSpreadsheetCell(row.Employee.FirstName + " " + row.Employee.LastName),
			compactDigits(row.Employee.RIB),
			formatMAD(row.NetPay),
			"SALAIRE " + request.Period,
		}
		if err := writer.Write(record); err != nil {
			return File{}, fmt.Errorf("écriture ligne CSV: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return File{}, fmt.Errorf("finalisation CSV: %w", err)
	}

	return File{
		Name:          "PAIE_CSV_" + request.Period + ".csv",
		MediaType:     "text/csv; charset=utf-8",
		Content:       buffer.Bytes(),
		RecordCount:   len(request.Payroll.Rows),
		TotalCentimes: request.Payroll.NetTotalCentimes,
		Contract:      "CSV universel UTF-8, séparateur point-virgule",
	}, nil
}

type isoDocument struct {
	XMLName                xml.Name                  `xml:"Document"`
	Xmlns                  string                    `xml:"xmlns,attr"`
	CustomerCreditTransfer isoCustomerCreditTransfer `xml:"CstmrCdtTrfInitn"`
}

type isoCustomerCreditTransfer struct {
	GroupHeader isoGroupHeader `xml:"GrpHdr"`
	PaymentInfo isoPaymentInfo `xml:"PmtInf"`
}

type isoGroupHeader struct {
	MessageID       string  `xml:"MsgId"`
	CreatedAt       string  `xml:"CreDtTm"`
	Transactions    int     `xml:"NbOfTxs"`
	ControlSum      string  `xml:"CtrlSum"`
	InitiatingParty isoName `xml:"InitgPty"`
}

type isoPaymentInfo struct {
	PaymentInfoID string        `xml:"PmtInfId"`
	PaymentMethod string        `xml:"PmtMtd"`
	BatchBooking  bool          `xml:"BtchBookg"`
	Transactions  int           `xml:"NbOfTxs"`
	ControlSum    string        `xml:"CtrlSum"`
	ExecutionDate string        `xml:"ReqdExctnDt"`
	Debtor        isoName       `xml:"Dbtr"`
	DebtorAccount isoAccount    `xml:"DbtrAcct"`
	Transfers     []isoTransfer `xml:"CdtTrfTxInf"`
}

type isoName struct {
	Name string `xml:"Nm"`
}
type isoAccount struct {
	ID isoOtherID `xml:"Id"`
}
type isoOtherID struct {
	Other isoGenericID `xml:"Othr"`
}
type isoGenericID struct {
	ID string `xml:"Id"`
}
type isoAmount struct {
	Currency string `xml:"Ccy,attr"`
	Value    string `xml:",chardata"`
}
type isoTransfer struct {
	PaymentID       isoPaymentID `xml:"PmtId"`
	Amount          isoAmount    `xml:"Amt>InstdAmt"`
	Creditor        isoName      `xml:"Cdtr"`
	CreditorAccount isoAccount   `xml:"CdtrAcct"`
	Remittance      string       `xml:"RmtInf>Ustrd"`
}
type isoPaymentID struct {
	EndToEndID string `xml:"EndToEndId"`
}

func generateISO20022(request Request) (File, error) {
	total := formatMAD(request.Payroll.NetTotalCentimes)
	document := isoDocument{
		Xmlns: "urn:iso:std:iso:20022:tech:xsd:pain.001.001.03",
		CustomerCreditTransfer: isoCustomerCreditTransfer{
			GroupHeader: isoGroupHeader{
				MessageID:       "PAIE-" + request.Period + "-" + strconv.Itoa(request.BatchNumber),
				CreatedAt:       request.ExecutionAt.Format("2006-01-02T15:04:05"),
				Transactions:    len(request.Payroll.Rows),
				ControlSum:      total,
				InitiatingParty: isoName{Name: request.Company.Name},
			},
			PaymentInfo: isoPaymentInfo{
				PaymentInfoID: "LOT-" + request.Period,
				PaymentMethod: "TRF",
				BatchBooking:  true,
				Transactions:  len(request.Payroll.Rows),
				ControlSum:    total,
				ExecutionDate: request.ExecutionAt.Format("2006-01-02"),
				Debtor:        isoName{Name: request.Company.Name},
				DebtorAccount: account(request.Company.RIB),
				Transfers:     []isoTransfer{},
			},
		},
	}
	for index, row := range request.Payroll.Rows {
		document.CustomerCreditTransfer.PaymentInfo.Transfers = append(
			document.CustomerCreditTransfer.PaymentInfo.Transfers,
			isoTransfer{
				PaymentID:       isoPaymentID{EndToEndID: fmt.Sprintf("%s-%04d", request.Period, index+1)},
				Amount:          isoAmount{Currency: "MAD", Value: formatMAD(row.NetPay)},
				Creditor:        isoName{Name: row.Employee.FirstName + " " + row.Employee.LastName},
				CreditorAccount: account(row.Employee.RIB),
				Remittance:      "SALAIRE " + request.Period,
			},
		)
	}

	encoded, err := xml.MarshalIndent(document, "", "  ")
	if err != nil {
		return File{}, fmt.Errorf("encodage ISO 20022: %w", err)
	}
	content := append([]byte(xml.Header), encoded...)
	content = append(content, '\n')
	return File{
		Name:          "PAIE_ISO20022_" + request.Period + ".xml",
		MediaType:     "application/xml; charset=utf-8",
		Content:       content,
		RecordCount:   len(request.Payroll.Rows),
		TotalCentimes: request.Payroll.NetTotalCentimes,
		Contract:      "ISO 20022 pain.001.001.03 — validation bancaire requise",
	}, nil
}

func generateCNSSControl(request Request) (File, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	writer.Comma = ';'
	writer.UseCRLF = true
	if err := writer.Write([]string{
		"AFFILIE", "PERIODE", "MATRICULE_CNSS", "NOM", "PRENOM", "CIN", "JOURS", "SALAIRE_BRUT_MAD", "SITUATION",
	}); err != nil {
		return File{}, fmt.Errorf("écriture entête CNSS: %w", err)
	}
	for _, row := range request.Payroll.Rows {
		record := []string{
			compactDigits(request.Company.AffiliateCNSS),
			request.Period,
			compactDigits(row.Employee.CNSS),
			row.Employee.LastName,
			row.Employee.FirstName,
			row.Employee.CIN,
			strconv.Itoa(row.Employee.DaysWorked),
			formatMAD(row.Employee.GrossCentimes),
			"ACTIF",
		}
		if err := writer.Write(record); err != nil {
			return File{}, fmt.Errorf("écriture ligne CNSS: %w", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return File{}, fmt.Errorf("finalisation contrôle CNSS: %w", err)
	}
	return File{
		Name:          "CONTROLE_CNSS_" + request.Period + ".txt",
		MediaType:     "text/plain; charset=utf-8",
		Content:       buffer.Bytes(),
		RecordCount:   len(request.Payroll.Rows),
		TotalCentimes: request.Payroll.GrossTotalCentimes,
		Contract:      "Fichier de rapprochement CNSS — ne remplace pas le préétabli Damancom",
	}, nil
}

func account(rib string) isoAccount {
	return isoAccount{ID: isoOtherID{Other: isoGenericID{ID: compactDigits(rib)}}}
}

func fixedText(value string, width int) string {
	ascii := asciiText(value)
	if len(ascii) > width {
		return ascii[:width]
	}
	return ascii + strings.Repeat(" ", width-len(ascii))
}

func asciiText(value string) string {
	replacer := strings.NewReplacer(
		"À", "A", "Â", "A", "Ä", "A", "Ç", "C", "É", "E", "È", "E", "Ê", "E", "Ë", "E",
		"Î", "I", "Ï", "I", "Ô", "O", "Ö", "O", "Ù", "U", "Û", "U", "Ü", "U",
		"à", "A", "â", "A", "ä", "A", "ç", "C", "é", "E", "è", "E", "ê", "E", "ë", "E",
		"î", "I", "ï", "I", "ô", "O", "ö", "O", "ù", "U", "û", "U", "ü", "U",
	)
	value = strings.ToUpper(replacer.Replace(strings.TrimSpace(value)))
	var builder strings.Builder
	for _, character := range value {
		if character >= 32 && character <= 126 && !unicode.IsControl(character) {
			builder.WriteRune(character)
			continue
		}
		builder.WriteByte(' ')
	}
	return builder.String()
}

func leftPadDigits(value string, width int) string {
	digits := compactDigits(value)
	if len(digits) > width {
		return digits[len(digits)-width:]
	}
	return strings.Repeat("0", width-len(digits)) + digits
}

func compactDigits(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if character >= '0' && character <= '9' {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func formatMAD(centimes int64) string {
	return fmt.Sprintf("%d.%02d", centimes/100, centimes%100)
}

func safeSpreadsheetCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	if strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}
