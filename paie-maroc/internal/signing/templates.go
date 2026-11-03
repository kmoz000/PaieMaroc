// Package signing persists the reusable signature templates applied on top
// of documents before saving or printing them. Templates are stored as a
// JSON file in the application configuration directory so they survive
// application restarts and remain local to this machine.
package signing

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Template is one reusable signature: the cleaned signature image plus its
// default placement expressed as fractions of an A4 page, so the same
// template applies to any page size.
type Template struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	PNGBase64 string  `json:"pngBase64"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Width     float64 `json:"width"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}

const maxTemplates = 50
const maxImageBytes = 4 << 20

// Load reads every stored template; a missing file returns an empty list.
func Load(path string) ([]Template, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Template{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lecture des modèles de signature: %w", err)
	}
	var templates []Template
	if err := json.Unmarshal(content, &templates); err != nil {
		return nil, fmt.Errorf("décodage des modèles de signature: %w", err)
	}
	if templates == nil {
		templates = []Template{}
	}
	return templates, nil
}

// Save inserts or updates one template and persists the whole list
// atomically. It returns the stored template with its identifier set.
func Save(path string, template Template) (Template, error) {
	template.Name = strings.TrimSpace(template.Name)
	if template.Name == "" {
		return Template{}, errors.New("le nom du modèle de signature est obligatoire")
	}
	if len(template.Name) > 80 {
		return Template{}, errors.New("le nom du modèle de signature est limité à 80 caractères")
	}
	image, err := base64.StdEncoding.DecodeString(template.PNGBase64)
	if err != nil {
		return Template{}, errors.New("l'image de signature est invalide")
	}
	if len(image) == 0 {
		return Template{}, errors.New("l'image de signature est vide")
	}
	if len(image) > maxImageBytes {
		return Template{}, errors.New("l'image de signature dépasse 4 Mo; réduisez sa taille")
	}
	if !strings.HasPrefix(string(image[:min(8, len(image))]), "\x89PNG\r\n\x1a\n") {
		return Template{}, errors.New("l'image de signature doit être au format PNG")
	}
	template.X = clampFraction(template.X, 0.72)
	template.Y = clampFraction(template.Y, 0.88)
	template.Width = clampWidth(template.Width)
	now := time.Now().Format(time.RFC3339)
	templates, err := Load(path)
	if err != nil {
		return Template{}, err
	}
	if template.ID == "" {
		if len(templates) >= maxTemplates {
			return Template{}, fmt.Errorf("limite de %d modèles de signature atteinte; supprimez-en un d'abord", maxTemplates)
		}
		template.ID = newID()
		template.CreatedAt = now
		template.UpdatedAt = now
		templates = append(templates, template)
	} else {
		found := false
		for index := range templates {
			if templates[index].ID == template.ID {
				template.CreatedAt = templates[index].CreatedAt
				template.UpdatedAt = now
				templates[index] = template
				found = true
				break
			}
		}
		if !found {
			return Template{}, errors.New("modèle de signature introuvable")
		}
	}
	if err := persist(path, templates); err != nil {
		return Template{}, err
	}
	return template, nil
}

// Delete removes one template by identifier.
func Delete(path string, id string) error {
	templates, err := Load(path)
	if err != nil {
		return err
	}
	kept := templates[:0]
	found := false
	for _, template := range templates {
		if template.ID == id {
			found = true
			continue
		}
		kept = append(kept, template)
	}
	if !found {
		return errors.New("modèle de signature introuvable")
	}
	return persist(path, kept)
}

func persist(path string, templates []Template) error {
	content, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return fmt.Errorf("encodage des modèles de signature: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("création du dossier des modèles de signature: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".signature-templates-*.tmp")
	if err != nil {
		return fmt.Errorf("création des modèles de signature: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("écriture des modèles de signature: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("fermeture des modèles de signature: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("enregistrement des modèles de signature: %w", err)
	}
	return nil
}

func clampFraction(value float64, fallback float64) float64 {
	if value < 0 || value >= 1 {
		return fallback
	}
	return value
}

func clampWidth(value float64) float64 {
	if value < 0.05 || value > 0.9 {
		return 0.25
	}
	return value
}

func newID() string {
	return fmt.Sprintf("sig-%d", time.Now().UnixNano())
}
