package exporter

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/payroll"
)

const simtMaxRows = 99_999

// RowWalker streams calculated rows in stable employee order.
type RowWalker func(ctx context.Context, yield func(payroll.Result) error) error

type StreamRequest struct {
	Format      Format
	Company     Company
	Period      string
	ExecutionAt time.Time
	BatchNumber int
}

type StreamResult struct {
	RecordCount   int64  `json:"recordCount"`
	TotalCentimes int64  `json:"totalCentimes"`
	FileCount     int    `json:"fileCount"`
	Contract      string `json:"contract"`
}

// Stream writes large exports without accumulating rows or file contents.
func Stream(ctx context.Context, request StreamRequest, walk RowWalker, output io.Writer) (StreamResult, error) {
	if walk == nil {
		return StreamResult{}, errors.New("source de lignes absente")
	}
	if len(compactDigits(request.Company.RIB)) != 24 {
		return StreamResult{}, errors.New("le RIB de l'entreprise doit contenir 24 chiffres")
	}
	if request.Period == "" || request.ExecutionAt.IsZero() {
		return StreamResult{}, errors.New("période et date d'exécution obligatoires")
	}
	switch request.Format {
	case FormatCSV:
		return streamCSV(ctx, request, walk, output)
	case FormatSIMT500:
		return streamSIMTZip(ctx, request, walk, output)
	default:
		return StreamResult{}, fmt.Errorf("le format %q n'est pas disponible en mode massif", request.Format)
	}
}

func streamCSV(ctx context.Context, request StreamRequest, walk RowWalker, output io.Writer) (StreamResult, error) {
	buffered := bufio.NewWriterSize(output, 256*1024)
	if _, err := buffered.WriteString("\xEF\xBB\xBF"); err != nil {
		return StreamResult{}, err
	}
	writer := csv.NewWriter(buffered)
	writer.Comma = ';'
	writer.UseCRLF = true
	if err := writer.Write([]string{"Matricule", "Bénéficiaire", "RIB", "Montant MAD", "Motif"}); err != nil {
		return StreamResult{}, err
	}
	result := StreamResult{FileCount: 1, Contract: "CSV universel UTF-8, séparateur point-virgule"}
	err := walk(ctx, func(row payroll.Result) error {
		if row.NetPay <= 0 {
			return fmt.Errorf("net à payer invalide pour %s", row.Employee.ID)
		}
		if err := writer.Write([]string{
			safeSpreadsheetCell(row.Employee.ID),
			safeSpreadsheetCell(row.Employee.FirstName + " " + row.Employee.LastName),
			compactDigits(row.Employee.RIB),
			formatMAD(row.NetPay),
			"SALAIRE " + request.Period,
		}); err != nil {
			return err
		}
		result.RecordCount++
		result.TotalCentimes += row.NetPay
		return nil
	})
	if err != nil {
		return StreamResult{}, err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return StreamResult{}, err
	}
	if err := buffered.Flush(); err != nil {
		return StreamResult{}, err
	}
	return result, nil
}

func streamSIMTZip(ctx context.Context, request StreamRequest, walk RowWalker, output io.Writer) (StreamResult, error) {
	archive := zip.NewWriter(output)
	result := StreamResult{Contract: "SIMT500 segmenté à 99 999 opérations par fichier"}
	var current io.Writer
	var chunkCount int
	var chunkTotal int64
	var chunkIndex int

	openChunk := func() error {
		chunkIndex++
		entry, err := archive.Create(fmt.Sprintf("PAIE_SIMT500_%s_%03d.txt", request.Period, chunkIndex))
		if err != nil {
			return err
		}
		current = entry
		chunkCount = 0
		chunkTotal = 0
		header := simtHeader(request, request.BatchNumber+chunkIndex-1)
		_, err = io.WriteString(current, header+"\r\n")
		return err
	}
	closeChunk := func() error {
		if current == nil {
			return nil
		}
		_, err := io.WriteString(current, simtFooter(chunkCount, chunkTotal)+"\r\n")
		current = nil
		return err
	}

	err := walk(ctx, func(row payroll.Result) error {
		if current == nil || chunkCount == simtMaxRows {
			if err := closeChunk(); err != nil {
				return err
			}
			if err := openChunk(); err != nil {
				return err
			}
		}
		line, err := simtDetail(request, row, int(result.RecordCount)+1)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(current, line+"\r\n"); err != nil {
			return err
		}
		chunkCount++
		chunkTotal += row.NetPay
		result.RecordCount++
		result.TotalCentimes += row.NetPay
		return nil
	})
	if err != nil {
		_ = archive.Close()
		return StreamResult{}, err
	}
	if result.RecordCount == 0 {
		_ = archive.Close()
		return StreamResult{}, errors.New("aucun salarié à exporter")
	}
	if err := closeChunk(); err != nil {
		_ = archive.Close()
		return StreamResult{}, err
	}
	if err := archive.Close(); err != nil {
		return StreamResult{}, err
	}
	result.FileCount = chunkIndex
	return result, nil
}

func simtHeader(request StreamRequest, batchNumber int) string {
	rib := compactDigits(request.Company.RIB)
	participant := leftPadDigits(request.Company.Participant, 3)
	if participant == "000" {
		participant = rib[:3]
	}
	return "10" + "00000" + "000000" + leftPadDigits(strconv.Itoa(batchNumber), 4) + participant +
		request.ExecutionAt.Format("20060102") + request.ExecutionAt.Format("150405") + "MAD2" + "32" + strings.Repeat(" ", 460)
}

func simtDetail(request StreamRequest, row payroll.Result, sequence int) (string, error) {
	issuer := compactDigits(request.Company.RIB)
	beneficiary := compactDigits(row.Employee.RIB)
	if len(beneficiary) != 24 || row.NetPay <= 0 {
		return "", fmt.Errorf("RIB ou net invalide pour %s", row.Employee.ID)
	}
	reference := fmt.Sprintf("%s-%08d", request.Period, sequence)
	line := "04" + "020" + issuer[:3] + beneficiary[:3] + issuer[3:6] + beneficiary[3:6] + strings.Repeat(" ", 26) +
		"MAD2" + leftPadDigits(strconv.FormatInt(row.NetPay, 10), 16) + strings.Repeat(" ", 37) + "00" + strings.Repeat(" ", 34) + "1" +
		fixedText(request.Company.Name, 35) + fixedText(row.Employee.FirstName+" "+row.Employee.LastName, 35) + issuer + beneficiary + "00000000" +
		fixedText("SALAIRE "+request.Period, 35) + fixedText("PAIE "+request.Period, 35) + strings.Repeat(" ", 140) + fixedText(reference, 24) + "MA" + "N"
	if len(line) != 500 {
		return "", errors.New("erreur interne de longueur SIMT500")
	}
	return line, nil
}

func simtFooter(count int, total int64) string {
	return "11" + leftPadDigits(strconv.Itoa(count), 5) + leftPadDigits(strconv.FormatInt(total, 10), 20) + strings.Repeat(" ", 473)
}
