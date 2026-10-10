package cnss

import (
	"context"
	"strings"
	"testing"
)

func TestCSVSourceSupportsMultipleUnregisteredEntrants(t *testing.T) {
	t.Parallel()
	csv := "cnss;nom_complet;cin;jours;salaire_reel_mad;salaire_plafonne_mad;situation;af_a_reverser_mad;entrant\n" +
		"168764721;AMINE ALAOUI;AB123456;26;8000;6000;;;non\n" +
		";SARA BENNANI;BE123456;20;5000;;;;oui\n" +
		"000000000;OMAR IDRISSI;CD456789;15;4200;;;;oui\n"
	path := t.TempDir() + "/cnss.db"
	source, stats, err := OpenCSVSource(context.Background(), strings.NewReader(csv), path)
	if err != nil {
		t.Fatalf("OpenCSVSource() error = %v", err)
	}
	defer source.Close()
	if stats.Imported != 3 {
		t.Fatalf("Imported = %d", stats.Imported)
	}
	if _, found, err := source.Existing("168764721"); err != nil || !found {
		t.Fatalf("Existing() found=%v error=%v", found, err)
	}
	var entrants int
	if err := source.WalkEntrants(func(declaration Declaration) error {
		entrants++
		if declaration.CNSS != "000000000" {
			t.Errorf("entrant CNSS = %q", declaration.CNSS)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if entrants != 2 {
		t.Fatalf("entrants = %d", entrants)
	}
}

func TestCNSSCheckDigit(t *testing.T) {
	t.Parallel()
	if !validCNSS("123456784") {
		t.Fatal("known valid number rejected")
	}
	if validCNSS("123456789") {
		t.Fatal("invalid check digit accepted")
	}
}
