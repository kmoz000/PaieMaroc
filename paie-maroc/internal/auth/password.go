// Package auth defines local identities and password handling for PaieMaroc.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	RoleAdmin   = "admin"
	RoleManager = "manager"

	argonMemory  = 64 * 1024
	argonTime    = 3
	argonThreads = 2
	argonKeyLen  = 32
	saltLength   = 16
)

var (
	ErrInvalidCredentials = errors.New("identifiant ou mot de passe incorrect")
	usernamePattern       = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,49}$`)
)

// User is the safe, public representation of a local account.
type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"createdAt"`
	LastLoginAt string `json:"lastLoginAt"`
}

type CreateInput struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Password    string `json:"password"`
	Role        string `json:"role"`
}

func NormalizeUsername(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func ValidateCreateInput(input CreateInput) (CreateInput, error) {
	input.Username = NormalizeUsername(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if !usernamePattern.MatchString(input.Username) {
		return CreateInput{}, errors.New("l’identifiant doit contenir 3 à 50 caractères: lettres minuscules, chiffres, point, tiret ou soulignement")
	}
	if length := utf8.RuneCountInString(input.DisplayName); length < 2 || length > 100 {
		return CreateInput{}, errors.New("le nom affiché doit contenir entre 2 et 100 caractères")
	}
	if input.Role != RoleAdmin && input.Role != RoleManager {
		return CreateInput{}, errors.New("rôle utilisateur invalide")
	}
	if err := ValidatePassword(input.Password); err != nil {
		return CreateInput{}, err
	}
	return input, nil
}

func ValidatePassword(password string) error {
	if len(password) > 128 {
		return errors.New("le mot de passe ne doit pas dépasser 128 octets")
	}
	if utf8.RuneCountInString(password) < 12 {
		return errors.New("le mot de passe doit contenir au moins 12 caractères")
	}
	return nil
}

// HashPassword creates a self-describing Argon2id hash with a random salt.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("génération du sel: %w", err)
	}
	digest := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonTime,
		argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	), nil
}

// VerifyPassword rejects malformed or unreasonably expensive stored parameters
// before allocating memory, then compares digests in constant time.
func VerifyPassword(encoded, password string) bool {
	params, salt, expected, err := parseHash(encoded)
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

// PerformDummyVerification keeps unknown-user logins close to the same cost as
// known-user logins without exposing or creating a real credential.
func PerformDummyVerification(password string) {
	salt := []byte("PaieMaroc-dummy!")
	digest := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	subtle.ConstantTimeCompare(digest, make([]byte, argonKeyLen))
}

type hashParams struct {
	memory  uint32
	time    uint32
	threads uint8
}

func parseHash(encoded string) (hashParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return hashParams{}, nil, nil, errors.New("format de hash invalide")
	}
	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return hashParams{}, nil, nil, errors.New("paramètres de hash invalides")
	}
	// These bounds prevent a modified local database from triggering an
	// excessive allocation or an unbounded CPU cost during login.
	if memory < 32*1024 || memory > 128*1024 || iterations < 1 || iterations > 5 || threads < 1 || threads > 4 {
		return hashParams{}, nil, nil, errors.New("paramètres de hash hors limites")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 16 || len(salt) > 32 {
		return hashParams{}, nil, nil, errors.New("sel de hash invalide")
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) < 16 || len(expected) > 64 {
		return hashParams{}, nil, nil, errors.New("empreinte de hash invalide")
	}
	return hashParams{memory: memory, time: iterations, threads: threads}, salt, expected, nil
}
