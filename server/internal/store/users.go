package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/auth"
)

// User is an Attic account.
type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	Role         auth.Role
	CreatedAt    time.Time
}

// CreateUser inserts a new account. It returns ErrConflict if the username is
// taken.
func (s *Store) CreateUser(ctx context.Context, username, passwordHash string, role auth.Role) (*User, error) {
	const query = `
		INSERT INTO users (username, password_hash, role)
		VALUES ($1, $2, $3)
		RETURNING id, username, password_hash, role, created_at`

	var u User
	err := s.pool.QueryRow(ctx, query, username, passwordHash, string(role)).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &u, nil
}

// UserByUsername looks an account up for login.
func (s *Store) UserByUsername(ctx context.Context, username string) (*User, error) {
	const query = `
		SELECT id, username, password_hash, role, created_at
		FROM users WHERE username = $1`

	var u User
	err := s.pool.QueryRow(ctx, query, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &u, nil
}

// UserByID looks an account up by primary key.
func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	const query = `
		SELECT id, username, password_hash, role, created_at
		FROM users WHERE id = $1`

	var u User
	err := s.pool.QueryRow(ctx, query, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &u, nil
}

// UpdatePasswordHash rewrites a user's stored hash, used to transparently
// upgrade argon2 parameters after a successful login.
func (s *Store) UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error {
	_, err := s.pool.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, hash)
	return mapError(err)
}

// CountUsers reports how many accounts exist. The adduser CLI uses it to make
// the first account an admin.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		return 0, mapError(err)
	}
	return n, nil
}
