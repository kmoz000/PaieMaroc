package printing

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSettingsMissingFile(t *testing.T) {
	settings, err := LoadSettings(filepath.Join(t.TempDir(), "printing.json"))
	if err != nil {
		t.Fatalf("un fichier absent ne doit pas être une erreur: %v", err)
	}
	if settings.DefaultPrinter != "" {
		t.Fatalf("réglages vides attendus, obtenu %q", settings.DefaultPrinter)
	}
}

func TestSaveAndLoadSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "printing.json")
	if err := SaveSettings(path, Settings{DefaultPrinter: "  HP LaserJet  "}); err != nil {
		t.Fatalf("enregistrement: %v", err)
	}
	settings, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("relecture: %v", err)
	}
	if settings.DefaultPrinter != "HP LaserJet" {
		t.Fatalf("imprimante par défaut inattendue: %q", settings.DefaultPrinter)
	}
}

func TestLoadSettingsRejectsCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "printing.json")
	if err := os.WriteFile(path, []byte("{pas du json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(path); err == nil {
		t.Fatal("un JSON corrompu doit être signalé")
	}
}

func TestPrintRequiresPrinter(t *testing.T) {
	err := Print(Request{})
	if err == nil || !strings.Contains(err.Error(), "imprimante") {
		t.Fatalf("une imprimante manquante doit être signalée, obtenu %v", err)
	}
}

func TestPrintCUPSRequiresDocument(t *testing.T) {
	err := printCUPS(Request{}, "Imprimante")
	if err == nil || !strings.Contains(err.Error(), "PDF") {
		t.Fatalf("un document manquant doit être signalé, obtenu %v", err)
	}
}

func TestPrintWindowsRequiresPages(t *testing.T) {
	err := printWindows(Request{}, "Imprimante")
	if err == nil {
		t.Fatal("des pages manquantes doivent être signalées")
	}
	if !strings.Contains(err.Error(), "page") {
		t.Fatalf("message inattendu: %v", err)
	}
}

func TestJobTitleFallsBack(t *testing.T) {
	if jobTitle("   ") != "PaieMaroc" {
		t.Fatal("un titre vide doit retomber sur PaieMaroc")
	}
	long := strings.Repeat("a", 200)
	if len(jobTitle(long)) != 120 {
		t.Fatal("un titre trop long doit être tronqué à 120 caractères")
	}
}

func TestFirstLineFallsBackToError(t *testing.T) {
	if firstLine("", errors.New("cause")) != "cause" {
		t.Fatal("sans sortie, l'erreur d'origine doit être renvoyée")
	}
	if firstLine("\n  détail  \nsuite", errors.New("cause")) != "détail" {
		t.Fatal("la première ligne non vide doit être renvoyée")
	}
}
