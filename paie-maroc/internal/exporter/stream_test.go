package exporter_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/Niceblueman/PaieMaroc/paie-maroc/internal/exporter"
	"github.com/Niceblueman/PaieMaroc/paie-maroc/internal/payroll"
)

func TestStreamCSV(t *testing.T) {
	t.Parallel()
	rows := validRequest(exporter.FormatCSV).Payroll.Rows
	walk := func(_ context.Context, yield func(payroll.Result) error) error {
		for _, row := range rows {
			if err := yield(row); err != nil {
				return err
			}
		}
		return nil
	}
	var output bytes.Buffer
	result, err := exporter.Stream(context.Background(), streamRequest(exporter.FormatCSV), walk, &output)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if result.RecordCount != 2 || result.TotalCentimes != 1_211_500 {
		t.Fatalf("result = %+v", result)
	}
	if output.Len() == 0 {
		t.Fatal("empty output")
	}
}

func TestStreamSIMTCreatesZip(t *testing.T) {
	t.Parallel()
	rows := validRequest(exporter.FormatSIMT500).Payroll.Rows
	walk := func(_ context.Context, yield func(payroll.Result) error) error {
		for _, row := range rows {
			if err := yield(row); err != nil {
				return err
			}
		}
		return nil
	}
	var output bytes.Buffer
	result, err := exporter.Stream(context.Background(), streamRequest(exporter.FormatSIMT500), walk, &output)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if result.FileCount != 1 {
		t.Fatalf("FileCount = %d", result.FileCount)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	content, err := io.ReadAll(mustOpen(t, archive.File[0]))
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes.Split(bytes.TrimSpace(content), []byte("\r\n"))) != 4 {
		t.Fatal("unexpected SIMT record count")
	}
}

func TestStreamSIMTSplitsAtContractLimit(t *testing.T) {
	t.Parallel()
	row := validRequest(exporter.FormatSIMT500).Payroll.Rows[0]
	walk := func(_ context.Context, yield func(payroll.Result) error) error {
		for index := 0; index < 100_000; index++ {
			if err := yield(row); err != nil {
				return err
			}
		}
		return nil
	}
	var output bytes.Buffer
	result, err := exporter.Stream(context.Background(), streamRequest(exporter.FormatSIMT500), walk, &output)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	if result.RecordCount != 100_000 || result.FileCount != 2 {
		t.Fatalf("result = %+v, want 100000 records in 2 files", result)
	}
	archive, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 2 {
		t.Fatalf("zip entries = %d, want 2", len(archive.File))
	}
}

func BenchmarkStreamMillionCSV(b *testing.B) {
	row := validRequest(exporter.FormatCSV).Payroll.Rows[0]
	request := streamRequest(exporter.FormatCSV)
	for iteration := 0; iteration < b.N; iteration++ {
		walk := func(_ context.Context, yield func(payroll.Result) error) error {
			for index := 0; index < 1_000_000; index++ {
				if err := yield(row); err != nil {
					return err
				}
			}
			return nil
		}
		if _, err := exporter.Stream(context.Background(), request, walk, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

func streamRequest(format exporter.Format) exporter.StreamRequest {
	r := validRequest(format)
	return exporter.StreamRequest{Format: r.Format, Company: r.Company, Period: r.Period, ExecutionAt: r.ExecutionAt, BatchNumber: r.BatchNumber}
}

func mustOpen(t *testing.T, file *zip.File) io.ReadCloser {
	t.Helper()
	reader, err := file.Open()
	if err != nil {
		t.Fatal(err)
	}
	return reader
}
