package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/kmoz000/PaieMaroc/paie-maroc/internal/auth"
)

var (
	ErrSetupComplete = errors.New("le compte administrateur initial existe déjà")
	ErrAdminRequired = errors.New("un compte administrateur actif est requis")
	ErrLastAdmin     = errors.New("le dernier administrateur actif ne peut pas être désactivé")
	ErrSelfDisable   = errors.New("vous ne pouvez pas désactiver votre propre compte")
)

func (store *Store) HasUsers(ctx context.Context) (bool, error) {
	var exists bool
	if err := store.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users)`).Scan(&exists); err != nil {
		return false, fmt.Errorf("vérification des comptes: %w", err)
	}
	return exists, nil
}

// CreateInitialAdmin is atomic: only the first caller can create the bootstrap
// account, even if two application processes start at the same time.
func (store *Store) CreateInitialAdmin(ctx context.Context, input auth.CreateInput) (auth.User, error) {
	input.Role = auth.RoleAdmin
	validated, err := auth.ValidateCreateInput(input)
	if err != nil {
		return auth.User{}, err
	}
	hash, err := auth.HashPassword(validated.Password)
	if err != nil {
		return auth.User{}, err
	}
	result, err := store.db.ExecContext(ctx, `INSERT INTO users (username, display_name, role, password_hash)
		SELECT ?, ?, ?, ? WHERE NOT EXISTS (SELECT 1 FROM users)`,
		validated.Username, validated.DisplayName, auth.RoleAdmin, hash)
	if err != nil {
		return auth.User{}, fmt.Errorf("création de l’administrateur: %w", err)
	}
	created, err := result.RowsAffected()
	if err != nil {
		return auth.User{}, fmt.Errorf("confirmation de l’administrateur: %w", err)
	}
	if created != 1 {
		return auth.User{}, ErrSetupComplete
	}
	return store.userByUsername(ctx, validated.Username)
}

func (store *Store) Authenticate(ctx context.Context, username, password string) (auth.User, error) {
	normalized := auth.NormalizeUsername(username)
	var user auth.User
	var hash string
	var enabled int
	var lastLogin sql.NullString
	err := store.db.QueryRowContext(ctx, `SELECT id, username, display_name, role, password_hash, enabled,
		created_at, last_login_at FROM users WHERE username = ?`, normalized).Scan(
		&user.ID, &user.Username, &user.DisplayName, &user.Role, &hash, &enabled,
		&user.CreatedAt, &lastLogin,
	)
	if errors.Is(err, sql.ErrNoRows) {
		auth.PerformDummyVerification(password)
		return auth.User{}, auth.ErrInvalidCredentials
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("lecture du compte: %w", err)
	}
	if !auth.VerifyPassword(hash, password) || enabled != 1 {
		return auth.User{}, auth.ErrInvalidCredentials
	}
	user.Enabled = true
	user.LastLoginAt = lastLogin.String
	if _, err := store.db.ExecContext(ctx, `UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = ?`, user.ID); err != nil {
		return auth.User{}, fmt.Errorf("mise à jour de la connexion: %w", err)
	}
	return user, nil
}

func (store *Store) GetUser(ctx context.Context, id int64) (auth.User, error) {
	return store.userByID(ctx, store.db, id)
}

func (store *Store) ListUsers(ctx context.Context, actorID int64) ([]auth.User, error) {
	if err := store.requireAdmin(ctx, store.db, actorID); err != nil {
		return nil, err
	}
	rows, err := store.db.QueryContext(ctx, `SELECT id, username, display_name, role, enabled,
		created_at, last_login_at FROM users ORDER BY display_name, id`)
	if err != nil {
		return nil, fmt.Errorf("lecture des utilisateurs: %w", err)
	}
	defer rows.Close()
	users := []auth.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("parcours des utilisateurs: %w", err)
	}
	return users, nil
}

func (store *Store) CreateUser(ctx context.Context, actorID int64, input auth.CreateInput) (auth.User, error) {
	validated, err := auth.ValidateCreateInput(input)
	if err != nil {
		return auth.User{}, err
	}
	hash, err := auth.HashPassword(validated.Password)
	if err != nil {
		return auth.User{}, err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return auth.User{}, fmt.Errorf("début création utilisateur: %w", err)
	}
	defer tx.Rollback()
	if err := store.requireAdmin(ctx, tx, actorID); err != nil {
		return auth.User{}, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO users (username, display_name, role, password_hash)
		VALUES (?, ?, ?, ?)`, validated.Username, validated.DisplayName, validated.Role, hash)
	if err != nil {
		return auth.User{}, fmt.Errorf("création utilisateur: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return auth.User{}, fmt.Errorf("identifiant utilisateur: %w", err)
	}
	user, err := store.userByID(ctx, tx, id)
	if err != nil {
		return auth.User{}, err
	}
	if err := tx.Commit(); err != nil {
		return auth.User{}, fmt.Errorf("validation utilisateur: %w", err)
	}
	return user, nil
}

func (store *Store) SetUserEnabled(ctx context.Context, actorID, targetID int64, enabled bool) error {
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("début modification utilisateur: %w", err)
	}
	defer tx.Rollback()
	if err := store.requireAdmin(ctx, tx, actorID); err != nil {
		return err
	}
	if actorID == targetID && !enabled {
		return ErrSelfDisable
	}
	target, err := store.userByID(ctx, tx, targetID)
	if err != nil {
		return err
	}
	if !enabled && target.Role == auth.RoleAdmin && target.Enabled {
		var activeAdmins int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = ? AND enabled = 1`, auth.RoleAdmin).Scan(&activeAdmins); err != nil {
			return fmt.Errorf("comptage des administrateurs: %w", err)
		}
		if activeAdmins <= 1 {
			return ErrLastAdmin
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, enabled, targetID); err != nil {
		return fmt.Errorf("modification utilisateur: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("validation utilisateur: %w", err)
	}
	return nil
}

func (store *Store) ResetUserPassword(ctx context.Context, actorID, targetID int64, password string) error {
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("début réinitialisation: %w", err)
	}
	defer tx.Rollback()
	if err := store.requireAdmin(ctx, tx, actorID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, hash, targetID)
	if err != nil {
		return fmt.Errorf("réinitialisation du mot de passe: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return errors.New("utilisateur introuvable")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("validation du mot de passe: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

type databaseQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (store *Store) requireAdmin(ctx context.Context, query databaseQuerier, id int64) error {
	var allowed bool
	if err := query.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM users WHERE id = ? AND role = ? AND enabled = 1
	)`, id, auth.RoleAdmin).Scan(&allowed); err != nil {
		return fmt.Errorf("vérification administrateur: %w", err)
	}
	if !allowed {
		return ErrAdminRequired
	}
	return nil
}

func (store *Store) userByUsername(ctx context.Context, username string) (auth.User, error) {
	return scanUser(store.db.QueryRowContext(ctx, `SELECT id, username, display_name, role, enabled,
		created_at, last_login_at FROM users WHERE username = ?`, username))
}

func (store *Store) userByID(ctx context.Context, query databaseQuerier, id int64) (auth.User, error) {
	return scanUser(query.QueryRowContext(ctx, `SELECT id, username, display_name, role, enabled,
		created_at, last_login_at FROM users WHERE id = ?`, id))
}

func scanUser(row rowScanner) (auth.User, error) {
	var user auth.User
	var enabled int
	var lastLogin sql.NullString
	if err := row.Scan(&user.ID, &user.Username, &user.DisplayName, &user.Role, &enabled, &user.CreatedAt, &lastLogin); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return auth.User{}, errors.New("utilisateur introuvable")
		}
		return auth.User{}, fmt.Errorf("décodage utilisateur: %w", err)
	}
	user.Enabled = enabled == 1
	user.LastLoginAt = lastLogin.String
	return user, nil
}
