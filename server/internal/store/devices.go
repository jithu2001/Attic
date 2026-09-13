package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Device is one signed-in client. The refresh token is only ever stored as a
// hash, and is rotated on every refresh.
type Device struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	ExpiresAt time.Time
	CreatedAt time.Time
	LastSeen  time.Time
}

// CreateDevice records a new signed-in device.
func (s *Store) CreateDevice(ctx context.Context, userID uuid.UUID, name string, refreshHash []byte, expiresAt time.Time) (*Device, error) {
	const query = `
		INSERT INTO devices (user_id, name, refresh_token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, expires_at, created_at, last_seen`

	var d Device
	err := s.pool.QueryRow(ctx, query, userID, name, refreshHash, expiresAt).
		Scan(&d.ID, &d.UserID, &d.Name, &d.ExpiresAt, &d.CreatedAt, &d.LastSeen)
	if err != nil {
		return nil, mapError(err)
	}
	return &d, nil
}

// RotateRefreshToken atomically swaps a device's refresh token for a new one.
//
// The swap is a single conditional UPDATE so two concurrent refreshes with the
// same token cannot both succeed: the loser matches no row and gets
// ErrNotFound, which the API reports as a revoked session.
func (s *Store) RotateRefreshToken(ctx context.Context, oldHash, newHash []byte, expiresAt time.Time) (*Device, error) {
	const query = `
		UPDATE devices
		SET refresh_token_hash = $2, expires_at = $3, last_seen = now()
		WHERE refresh_token_hash = $1 AND expires_at > now()
		RETURNING id, user_id, name, expires_at, created_at, last_seen`

	var d Device
	err := s.pool.QueryRow(ctx, query, oldHash, newHash, expiresAt).
		Scan(&d.ID, &d.UserID, &d.Name, &d.ExpiresAt, &d.CreatedAt, &d.LastSeen)
	if err != nil {
		return nil, mapError(err)
	}
	return &d, nil
}

// DeleteDeviceByRefreshToken revokes the session holding refreshHash.
func (s *Store) DeleteDeviceByRefreshToken(ctx context.Context, refreshHash []byte) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM devices WHERE refresh_token_hash = $1`, refreshHash)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DevicesForUser lists a user's signed-in devices, most recently seen first.
func (s *Store) DevicesForUser(ctx context.Context, userID uuid.UUID) ([]Device, error) {
	const query = `
		SELECT id, user_id, name, expires_at, created_at, last_seen
		FROM devices WHERE user_id = $1
		ORDER BY last_seen DESC`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	devices := make([]Device, 0, 4)
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.ExpiresAt, &d.CreatedAt, &d.LastSeen); err != nil {
			return nil, mapError(err)
		}
		devices = append(devices, d)
	}
	return devices, mapError(rows.Err())
}

// DeleteExpiredDevices clears sessions whose refresh tokens have lapsed.
func (s *Store) DeleteExpiredDevices(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM devices WHERE expires_at <= now()`)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}
