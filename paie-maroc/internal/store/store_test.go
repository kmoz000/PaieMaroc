package store_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Niceblueman/PaieMaroc/paie-maroc/internal/payroll"
	"github.com/Niceblueman/PaieMaroc/paie-maroc/internal/store"
)

func TestStoreUpsertAndKeysetPagination(t *testing.T) {
	t.Parallel()

	database, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	ctx := context.Background()
	for index := 1; index <= 3; index++ {
		employee := validEmployee(fmt.Sprintf("EMP-%03d", index), fmt.Sprintf("%07d", index))
		if err := database.Upsert(ctx, employee); err != nil {
			t.Fatalf("Upsert() error = %v", err)
		}
	}
	first, err := database.List(ctx, "", "", 2)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(first.Items) != 2 || first.NextCursor != "EMP-002" || first.Total != 3 {
		t.Fatalf("first page = %+v", first)
	}
	second, err := database.List(ctx, first.NextCursor, "", 2)
	if err != nil {
		t.Fatalf("List(second) error = %v", err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != "EMP-003" {
		t.Fatalf("second page = %+v", second)
	}
}

func TestImportCSVStreamsValidRows(t *testing.T) {
	t.Parallel()

	database, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()
	csv := strings.Join([]string{
		"matricule;prenom;nom;cin;cnss;rib;jours;personnes_a_charge;salaire_brut_mad;exonere_mad;avance_mad",
		"EMP-001;Amine;Alaoui;AB123456;1234567;011780000012345678901234;26;2;8000,50;0;500",
		"EMP-002;Sara;Bennani;BAD;7654321;022780000098765432109876;26;0;9000;0;0",
	}, "\n")
	stats, err := database.ImportCSV(context.Background(), strings.NewReader(csv))
	if err != nil {
		t.Fatalf("ImportCSV() error = %v", err)
	}
	if stats.Imported != 1 || stats.Rejected != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func validEmployee(id, cnss string) payroll.Employee {
	return payroll.Employee{
		ID: id, FirstName: "Amine", LastName: "Alaoui", CIN: "AB123456", CNSS: cnss,
		RIB: "011780000012345678901234", DaysWorked: 26, GrossCentimes: 800_000,
	}
}
