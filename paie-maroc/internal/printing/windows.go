package printing

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The Windows implementations shell out to PowerShell, which ships with
// every supported Windows version. They compile on every platform but are
// only called when runtime.GOOS == "windows".

type windowsPrinter struct {
	Name    string `json:"Name"`
	Default bool   `json:"Default"`
}

func listWindows() ([]Printer, error) {
	powershell, err := lookupPowerShell()
	if err != nil {
		return nil, err
	}
	command := "Get-CimInstance Win32_Printer | Select-Object Name,Default | ConvertTo-Json -Compress"
	output, err := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", command).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("liste des imprimantes impossible: %s", firstLine(strings.TrimSpace(string(output)), err))
	}
	text := strings.TrimSpace(string(output))
	if text == "" {
		return []Printer{}, nil
	}
	var entries []windowsPrinter
	if strings.HasPrefix(text, "{") {
		var single windowsPrinter
		if err := json.Unmarshal([]byte(text), &single); err != nil {
			return nil, fmt.Errorf("décodage de la liste des imprimantes: %w", err)
		}
		entries = []windowsPrinter{single}
	} else if err := json.Unmarshal([]byte(text), &entries); err != nil {
		return nil, fmt.Errorf("décodage de la liste des imprimantes: %w", err)
	}
	printers := make([]Printer, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		printers = append(printers, Printer{Name: name, SystemDefault: entry.Default})
	}
	return printers, nil
}

// windowsPrintScript prints a list of page images with System.Drawing so the
// job works with any installed driver, without requiring a PDF reader. The
// job parameters are read from a JSON file to avoid any quoting issues.
const windowsPrintScript = `
$ErrorActionPreference = 'Stop'
$job = Get-Content -LiteralPath $args[0] -Raw -Encoding UTF8 | ConvertFrom-Json
Add-Type -AssemblyName System.Drawing
$document = New-Object System.Drawing.Printing.PrintDocument
$document.PrinterSettings.PrinterName = $job.printer
if (-not $document.PrinterSettings.IsValid) {
  throw "Imprimante introuvable: $($job.printer)"
}
$document.DocumentName = $job.documentName
$document.PrinterSettings.Copies = [int]$job.copies
foreach ($size in $document.PrinterSettings.PaperSizes) {
  if ($size.Kind -eq [System.Drawing.Printing.PaperKind]::A4) {
    $document.DefaultPageSettings.PaperSize = $size
    break
  }
}
$script:pageIndex = 0
$pages = @($job.pages)
$document.add_PrintPage({
  param($sender, $event)
  $image = [System.Drawing.Image]::FromFile($pages[$script:pageIndex])
  try {
    $area = $event.PageSettings.PrintableArea
    $scale = [Math]::Min($area.Width / $image.Width, $area.Height / $image.Height)
    $width = $image.Width * $scale
    $height = $image.Height * $scale
    $left = $area.X + (($area.Width - $width) / 2) - $event.PageSettings.HardMarginX
    $top = $area.Y + (($area.Height - $height) / 2) - $event.PageSettings.HardMarginY
    $event.Graphics.DrawImage($image, $left, $top, $width, $height)
  } finally {
    $image.Dispose()
  }
  $script:pageIndex++
  $event.HasMorePages = $script:pageIndex -lt $pages.Count
})
$document.Print()
`

type windowsJob struct {
	Printer      string   `json:"printer"`
	DocumentName string   `json:"documentName"`
	Copies       int      `json:"copies"`
	Pages        []string `json:"pages"`
}

func printWindows(request Request, printer string) error {
	if len(request.PageImages) == 0 {
		return errors.New("aucune page à imprimer")
	}
	powershell, err := lookupPowerShell()
	if err != nil {
		return err
	}
	directory, err := os.MkdirTemp("", "paie-maroc-impression-*")
	if err != nil {
		return fmt.Errorf("préparation de l'impression: %w", err)
	}
	defer os.RemoveAll(directory)
	pages := make([]string, 0, len(request.PageImages))
	for index, encoded := range request.PageImages {
		content, decodeErr := base64.StdEncoding.DecodeString(encoded)
		if decodeErr != nil {
			return fmt.Errorf("décodage de la page %d à imprimer: %w", index+1, decodeErr)
		}
		path := filepath.Join(directory, fmt.Sprintf("page-%03d.png", index+1))
		if writeErr := os.WriteFile(path, content, 0o600); writeErr != nil {
			return fmt.Errorf("écriture de la page %d à imprimer: %w", index+1, writeErr)
		}
		pages = append(pages, path)
	}
	job := windowsJob{
		Printer:      printer,
		DocumentName: jobTitle(request.DocumentName),
		Copies:       request.Copies,
		Pages:        pages,
	}
	jobContent, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("préparation du travail d'impression: %w", err)
	}
	jobPath := filepath.Join(directory, "job.json")
	if err := os.WriteFile(jobPath, jobContent, 0o600); err != nil {
		return fmt.Errorf("écriture du travail d'impression: %w", err)
	}
	scriptPath := filepath.Join(directory, "print.ps1")
	if err := os.WriteFile(scriptPath, []byte(windowsPrintScript), 0o600); err != nil {
		return fmt.Errorf("écriture du script d'impression: %w", err)
	}
	output, err := exec.Command(
		powershell,
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-File", scriptPath, jobPath,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("impression refusée: %s", firstLine(strings.TrimSpace(string(output)), err))
	}
	return nil
}

func lookupPowerShell() (string, error) {
	if path, err := exec.LookPath("powershell.exe"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("pwsh.exe"); err == nil {
		return path, nil
	}
	return "", errors.New("PowerShell est introuvable sur ce poste; impossible de gérer les imprimantes")
}
