package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/printing"
	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/signing"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// PrinterState bundles the printers visible on this machine with the stored
// default so the frontend can run the first-use picker in one call.
type PrinterState struct {
	Printers       []printing.Printer `json:"printers"`
	DefaultPrinter string             `json:"defaultPrinter"`
	Configured     bool               `json:"configured"`
}

// SignedDocumentResult reports where a signed document was written.
type SignedDocumentResult struct {
	Path string `json:"path"`
}

// GetPrinterState lists the printers and returns the persisted default.
func (app *App) GetPrinterState() (PrinterState, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return PrinterState{}, err
	}
	settings, err := printing.LoadSettings(app.printingConfigPath)
	if err != nil {
		return PrinterState{}, err
	}
	printers, err := printing.List(settings.DefaultPrinter)
	if err != nil {
		return PrinterState{}, err
	}
	return PrinterState{
		Printers:       printers,
		DefaultPrinter: settings.DefaultPrinter,
		Configured:     settings.DefaultPrinter != "",
	}, nil
}

// SetDefaultPrinter persists the printer chosen by the user.
func (app *App) SetDefaultPrinter(name string) (PrinterState, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return PrinterState{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return PrinterState{}, errors.New("choisissez une imprimante")
	}
	if err := printing.SaveSettings(app.printingConfigPath, printing.Settings{DefaultPrinter: name}); err != nil {
		return PrinterState{}, err
	}
	return app.GetPrinterState()
}

// PrintSignedDocument prints the signed document on the requested printer,
// or on the stored default when none is given.
func (app *App) PrintSignedDocument(request printing.Request) error {
	if _, err := app.requireAuthenticated(); err != nil {
		return err
	}
	if strings.TrimSpace(request.Printer) == "" {
		settings, err := printing.LoadSettings(app.printingConfigPath)
		if err != nil {
			return err
		}
		request.Printer = settings.DefaultPrinter
	}
	return printing.Print(request)
}

// ListSignatureTemplates returns the reusable signature templates.
func (app *App) ListSignatureTemplates() ([]signing.Template, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return nil, err
	}
	return signing.Load(app.signatureTemplatesPath)
}

// SaveSignatureTemplate inserts or updates one reusable signature template.
func (app *App) SaveSignatureTemplate(template signing.Template) (signing.Template, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return signing.Template{}, err
	}
	return signing.Save(app.signatureTemplatesPath, template)
}

// DeleteSignatureTemplate removes one reusable signature template.
func (app *App) DeleteSignatureTemplate(id string) error {
	if _, err := app.requireAuthenticated(); err != nil {
		return err
	}
	return signing.Delete(app.signatureTemplatesPath, id)
}

// SaveSignedDocument asks where to store the signed PDF and writes it.
func (app *App) SaveSignedDocument(name string, contentBase64 string) (SignedDocumentResult, error) {
	if _, err := app.requireAuthenticated(); err != nil {
		return SignedDocumentResult{}, err
	}
	content, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil {
		return SignedDocumentResult{}, fmt.Errorf("décodage du document signé: %w", err)
	}
	if len(content) == 0 {
		return SignedDocumentResult{}, errors.New("le document signé est vide")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "document-signe.pdf"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
		name += ".pdf"
	}
	path, err := runtime.SaveFileDialog(app.ctx, runtime.SaveDialogOptions{
		Title:           "Enregistrer le document signé",
		DefaultFilename: name,
	})
	if err != nil {
		return SignedDocumentResult{}, fmt.Errorf("ouverture du dialogue d'enregistrement: %w", err)
	}
	if path == "" {
		return SignedDocumentResult{}, errors.New("enregistrement annulé")
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return SignedDocumentResult{}, fmt.Errorf("écriture du document signé: %w", err)
	}
	return SignedDocumentResult{Path: path}, nil
}
