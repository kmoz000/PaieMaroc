package signing

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pngSample is the base64 of a 1x1 transparent PNG.
const pngSample = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

func TestLoadMissingFileReturnsEmptyList(t *testing.T) {
	templates, err := Load(filepath.Join(t.TempDir(), "signature-templates.json"))
	if err != nil {
		t.Fatalf("un fichier absent ne doit pas être une erreur: %v", err)
	}
	if len(templates) != 0 {
		t.Fatalf("liste vide attendue, obtenu %d modèles", len(templates))
	}
}

func TestSaveLoadDeleteRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dossier", "signature-templates.json")
	saved, err := Save(path, Template{Name: "  Directeur RH  ", PNGBase64: pngSample, X: 0.6, Y: 0.8, Width: 0.3})
	if err != nil {
		t.Fatalf("enregistrement: %v", err)
	}
	if saved.ID == "" || saved.Name != "Directeur RH" || saved.CreatedAt == "" {
		t.Fatalf("modèle normalisé attendu, obtenu %+v", saved)
	}
	updated, err := Save(path, Template{ID: saved.ID, Name: "Directrice RH", PNGBase64: pngSample, X: 0.5, Y: 0.7, Width: 0.2})
	if err != nil {
		t.Fatalf("mise à jour: %v", err)
	}
	if updated.CreatedAt != saved.CreatedAt {
		t.Fatal("la date de création doit être conservée lors d'une mise à jour")
	}
	templates, err := Load(path)
	if err != nil {
		t.Fatalf("relecture: %v", err)
	}
	if len(templates) != 1 || templates[0].Name != "Directrice RH" {
		t.Fatalf("un seul modèle mis à jour attendu, obtenu %+v", templates)
	}
	if err := Delete(path, saved.ID); err != nil {
		t.Fatalf("suppression: %v", err)
	}
	templates, err = Load(path)
	if err != nil {
		t.Fatalf("relecture après suppression: %v", err)
	}
	if len(templates) != 0 {
		t.Fatal("la liste doit être vide après suppression")
	}
}

func TestSaveValidatesInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signature-templates.json")
	if _, err := Save(path, Template{Name: "", PNGBase64: pngSample}); err == nil {
		t.Fatal("un nom vide doit être refusé")
	}
	if _, err := Save(path, Template{Name: "Test", PNGBase64: "%%%"}); err == nil {
		t.Fatal("un base64 invalide doit être refusé")
	}
	notPNG := base64.StdEncoding.EncodeToString([]byte("GIF89a..."))
	if _, err := Save(path, Template{Name: "Test", PNGBase64: notPNG}); err == nil {
		t.Fatal("une image non PNG doit être refusée")
	}
	if _, err := Save(path, Template{ID: "sig-inconnu", Name: "Test", PNGBase64: pngSample}); err == nil {
		t.Fatal("la mise à jour d'un modèle inconnu doit être refusée")
	}
}

func TestSaveClampsPlacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signature-templates.json")
	saved, err := Save(path, Template{Name: "Position", PNGBase64: pngSample, X: 4, Y: -1, Width: 7})
	if err != nil {
		t.Fatalf("enregistrement: %v", err)
	}
	if saved.X != 0.72 || saved.Y != 0.88 || saved.Width != 0.25 {
		t.Fatalf("placement par défaut attendu, obtenu %+v", saved)
	}
}

func TestLoadRejectsCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signature-templates.json")
	if err := os.WriteFile(path, []byte("[pas du json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "décodage") {
		t.Fatalf("un JSON corrompu doit être signalé, obtenu %v", err)
	}
}

func TestDeleteUnknownTemplate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signature-templates.json")
	if err := Delete(path, "sig-inconnu"); err == nil {
		t.Fatal("la suppression d'un modèle inconnu doit être refusée")
	}
}
