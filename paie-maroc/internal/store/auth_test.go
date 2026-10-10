package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/auth"
	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/store"
)

func TestStoreInitialAdminAndAuthentication(t *testing.T) {
	t.Parallel()
	database := openStore(t)
	ctx := context.Background()

	hasUsers, err := database.HasUsers(ctx)
	if err != nil || hasUsers {
		t.Fatalf("HasUsers() = %v, %v", hasUsers, err)
	}
	admin, err := database.CreateInitialAdmin(ctx, auth.CreateInput{
		Username: "admin.paie", DisplayName: "Admin Paie", Password: "mot de passe robuste", Role: auth.RoleManager,
	})
	if err != nil {
		t.Fatalf("CreateInitialAdmin() error = %v", err)
	}
	if admin.Role != auth.RoleAdmin {
		t.Fatalf("role = %q, want admin", admin.Role)
	}
	if _, err := database.CreateInitialAdmin(ctx, auth.CreateInput{
		Username: "second", DisplayName: "Second Admin", Password: "mot de passe robuste",
	}); !errors.Is(err, store.ErrSetupComplete) {
		t.Fatalf("second CreateInitialAdmin() error = %v", err)
	}
	loggedIn, err := database.Authenticate(ctx, "ADMIN.PAIE", "mot de passe robuste")
	if err != nil || loggedIn.ID != admin.ID {
		t.Fatalf("Authenticate() = %+v, %v", loggedIn, err)
	}
	if _, err := database.Authenticate(ctx, "admin.paie", "mauvais mot de passe"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("bad password error = %v", err)
	}
	if _, err := database.Authenticate(ctx, "inconnu", "mauvais mot de passe"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("unknown user error = %v", err)
	}
}

func TestStoreAdminManagesUsers(t *testing.T) {
	t.Parallel()
	database := openStore(t)
	ctx := context.Background()
	admin, err := database.CreateInitialAdmin(ctx, auth.CreateInput{
		Username: "admin", DisplayName: "Admin Paie", Password: "mot de passe robuste",
	})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := database.CreateUser(ctx, admin.ID, auth.CreateInput{
		Username: "gestionnaire", DisplayName: "Gestionnaire Paie", Password: "autre passe robuste", Role: auth.RoleManager,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if _, err := database.ListUsers(ctx, manager.ID); !errors.Is(err, store.ErrAdminRequired) {
		t.Fatalf("manager ListUsers() error = %v", err)
	}
	if err := database.SetUserEnabled(ctx, admin.ID, manager.ID, false); err != nil {
		t.Fatalf("SetUserEnabled() error = %v", err)
	}
	if _, err := database.Authenticate(ctx, manager.Username, "autre passe robuste"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("disabled Authenticate() error = %v", err)
	}
	if err := database.ResetUserPassword(ctx, admin.ID, manager.ID, "nouveau passe robuste"); err != nil {
		t.Fatalf("ResetUserPassword() error = %v", err)
	}
}

func TestStoreProtectsLastAndCurrentAdmin(t *testing.T) {
	t.Parallel()
	database := openStore(t)
	ctx := context.Background()
	admin, err := database.CreateInitialAdmin(ctx, auth.CreateInput{
		Username: "admin", DisplayName: "Admin Paie", Password: "mot de passe robuste",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.SetUserEnabled(ctx, admin.ID, admin.ID, false); !errors.Is(err, store.ErrSelfDisable) {
		t.Fatalf("self disable error = %v", err)
	}
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	database, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}
