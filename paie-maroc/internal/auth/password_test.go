package auth_test

import (
	"strings"
	"testing"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/auth"
)

func TestValidateCreateInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   auth.CreateInput
		wantErr bool
	}{
		{name: "valid admin", input: auth.CreateInput{Username: " Admin.Paie ", DisplayName: "Karim Benali", Password: "une phrase tres sure", Role: auth.RoleAdmin}},
		{name: "invalid username", input: auth.CreateInput{Username: "A B", DisplayName: "Karim Benali", Password: "une phrase tres sure", Role: auth.RoleAdmin}, wantErr: true},
		{name: "short password", input: auth.CreateInput{Username: "karim", DisplayName: "Karim Benali", Password: "court", Role: auth.RoleAdmin}, wantErr: true},
		{name: "invalid role", input: auth.CreateInput{Username: "karim", DisplayName: "Karim Benali", Password: "une phrase tres sure", Role: "owner"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := auth.ValidateCreateInput(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCreateInput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got.Username != "admin.paie" && tt.name == "valid admin" {
				t.Fatalf("username = %q", got.Username)
			}
		})
	}
}

func TestHashPassword(t *testing.T) {
	t.Parallel()

	const password = "correct horse maroc"
	encoded, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !auth.VerifyPassword(encoded, password) {
		t.Fatal("VerifyPassword() rejected the correct password")
	}
	if auth.VerifyPassword(encoded, "incorrect password") {
		t.Fatal("VerifyPassword() accepted an incorrect password")
	}
	if auth.VerifyPassword("$argon2id$v=19$m=999999999,t=3,p=2$bad$bad", password) {
		t.Fatal("VerifyPassword() accepted unsafe parameters")
	}
}

func TestValidatePasswordRejectsOversizedInput(t *testing.T) {
	t.Parallel()

	if err := auth.ValidatePassword(strings.Repeat("a", 129)); err == nil {
		t.Fatal("ValidatePassword() accepted more than 128 bytes")
	}
}
