// Package printing lists the printers available on the host and prints
// signed documents on them. Windows uses PowerShell with System.Drawing
// (page images), macOS and Linux use CUPS (lp with the final PDF), so the
// package never requires cgo or a third-party dependency.
package printing

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Printer describes one printer visible to the operating system.
type Printer struct {
	Name          string `json:"name"`
	SystemDefault bool   `json:"systemDefault"`
	StoredDefault bool   `json:"storedDefault"`
}

// Settings is the persisted printing configuration.
type Settings struct {
	DefaultPrinter string `json:"defaultPrinter"`
}

// Request carries one print job. PDFBase64 is used on macOS and Linux,
// PageImages (PNG, already rasterized in A4 proportions) on Windows.
type Request struct {
	DocumentName string   `json:"documentName"`
	Printer      string   `json:"printer"`
	Copies       int      `json:"copies"`
	PDFBase64    string   `json:"pdfBase64"`
	PageImages   []string `json:"pageImages"`
}

// LoadSettings reads the persisted printing settings; a missing file is not
// an error and returns empty settings.
func LoadSettings(path string) (Settings, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("lecture des réglages d'impression: %w", err)
	}
	var settings Settings
	if err := json.Unmarshal(content, &settings); err != nil {
		return Settings{}, fmt.Errorf("décodage des réglages d'impression: %w", err)
	}
	settings.DefaultPrinter = strings.TrimSpace(settings.DefaultPrinter)
	return settings, nil
}

// SaveSettings writes the printing settings atomically.
func SaveSettings(path string, settings Settings) error {
	settings.DefaultPrinter = strings.TrimSpace(settings.DefaultPrinter)
	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encodage des réglages d'impression: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("création du dossier des réglages d'impression: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".printing-*.tmp")
	if err != nil {
		return fmt.Errorf("création des réglages d'impression: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("écriture des réglages d'impression: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("fermeture des réglages d'impression: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("enregistrement des réglages d'impression: %w", err)
	}
	return nil
}

// List returns the printers known to the operating system, flagging the
// system default and the stored default.
func List(storedDefault string) ([]Printer, error) {
	var printers []Printer
	var err error
	if runtime.GOOS == "windows" {
		printers, err = listWindows()
	} else {
		printers, err = listCUPS()
	}
	if err != nil {
		return nil, err
	}
	storedDefault = strings.TrimSpace(storedDefault)
	for index := range printers {
		printers[index].StoredDefault = storedDefault != "" && printers[index].Name == storedDefault
	}
	return printers, nil
}

// Print sends the request to the selected printer.
func Print(request Request) error {
	printer := strings.TrimSpace(request.Printer)
	if printer == "" {
		return errors.New("aucune imprimante sélectionnée")
	}
	if request.Copies < 1 {
		request.Copies = 1
	}
	if request.Copies > 99 {
		request.Copies = 99
	}
	if runtime.GOOS == "windows" {
		return printWindows(request, printer)
	}
	return printCUPS(request, printer)
}

func listCUPS() ([]Printer, error) {
	lpstat, err := exec.LookPath("lpstat")
	if err != nil {
		return nil, errors.New("le service d'impression CUPS (lpstat) est introuvable sur ce poste")
	}
	output, err := exec.Command(lpstat, "-p").CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if strings.Contains(text, "No destinations") || strings.Contains(text, "no destinations") {
			return []Printer{}, nil
		}
		return nil, fmt.Errorf("liste des imprimantes impossible: %s", firstLine(text, err))
	}
	var printers []Printer
	for line := range strings.SplitSeq(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "printer" {
			printers = append(printers, Printer{Name: fields[1]})
		}
	}
	defaultOutput, defaultErr := exec.Command(lpstat, "-d").CombinedOutput()
	if defaultErr == nil {
		text := strings.TrimSpace(string(defaultOutput))
		if colon := strings.LastIndex(text, ":"); colon >= 0 {
			name := strings.TrimSpace(text[colon+1:])
			for index := range printers {
				if printers[index].Name == name {
					printers[index].SystemDefault = true
				}
			}
		}
	}
	if printers == nil {
		printers = []Printer{}
	}
	return printers, nil
}

func printCUPS(request Request, printer string) error {
	if strings.TrimSpace(request.PDFBase64) == "" {
		return errors.New("document PDF manquant pour l'impression")
	}
	content, err := base64.StdEncoding.DecodeString(request.PDFBase64)
	if err != nil {
		return fmt.Errorf("décodage du document à imprimer: %w", err)
	}
	lp, err := exec.LookPath("lp")
	if err != nil {
		return errors.New("la commande d'impression CUPS (lp) est introuvable sur ce poste")
	}
	directory, err := os.MkdirTemp("", "paie-maroc-impression-*")
	if err != nil {
		return fmt.Errorf("préparation de l'impression: %w", err)
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "document.pdf")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("écriture du document à imprimer: %w", err)
	}
	arguments := []string{
		"-d", printer,
		"-n", fmt.Sprintf("%d", request.Copies),
		"-o", "media=A4",
		"-t", jobTitle(request.DocumentName),
		path,
	}
	output, err := exec.Command(lp, arguments...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("impression refusée: %s", firstLine(strings.TrimSpace(string(output)), err))
	}
	return nil
}

func jobTitle(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "PaieMaroc"
	}
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

func firstLine(text string, fallback error) string {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return fallback.Error()
}
