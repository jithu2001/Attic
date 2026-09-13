package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Playlist is a user's ordered list of tracks.
type Playlist struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	Name       string
	TrackCount int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CreatePlaylist makes an empty playlist owned by ownerID.
func (s *Store) CreatePlaylist(ctx context.Context, ownerID uuid.UUID, name string) (*Playlist, error) {
	const query = `
		INSERT INTO playlists (owner_id, name)
		VALUES ($1, $2)
		RETURNING id, owner_id, name, created_at, updated_at`

	var p Playlist
	err := s.pool.QueryRow(ctx, query, ownerID, name).
		Scan(&p.ID, &p.OwnerID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, mapError(err)
	}
	return &p, nil
}

// PlaylistsForUser lists a user's playlists, most recently updated first.
func (s *Store) PlaylistsForUser(ctx context.Context, ownerID uuid.UUID) ([]Playlist, error) {
	const query = `
		SELECT p.id, p.owner_id, p.name, p.created_at, p.updated_at,
		       (SELECT count(*) FROM playlist_tracks pt WHERE pt.playlist_id = p.id)
		FROM playlists p
		WHERE p.owner_id = $1
		ORDER BY p.updated_at DESC`

	rows, err := s.pool.Query(ctx, query, ownerID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	playlists := make([]Playlist, 0, 8)
	for rows.Next() {
		var p Playlist
		if err := rows.Scan(&p.ID, &p.OwnerID, &p.Name, &p.CreatedAt, &p.UpdatedAt, &p.TrackCount); err != nil {
			return nil, mapError(err)
		}
		playlists = append(playlists, p)
	}
	return playlists, mapError(rows.Err())
}

// Playlist returns one playlist. Ownership is the caller's to check.
func (s *Store) Playlist(ctx context.Context, id uuid.UUID) (*Playlist, error) {
	const query = `
		SELECT p.id, p.owner_id, p.name, p.created_at, p.updated_at,
		       (SELECT count(*) FROM playlist_tracks pt WHERE pt.playlist_id = p.id)
		FROM playlists p
		WHERE p.id = $1`

	var p Playlist
	err := s.pool.QueryRow(ctx, query, id).
		Scan(&p.ID, &p.OwnerID, &p.Name, &p.CreatedAt, &p.UpdatedAt, &p.TrackCount)
	if err != nil {
		return nil, mapError(err)
	}
	return &p, nil
}

// RenamePlaylist updates a playlist's name.
func (s *Store) RenamePlaylist(ctx context.Context, id uuid.UUID, name string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE playlists SET name = $2, updated_at = now() WHERE id = $1`, id, name)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeletePlaylist removes a playlist and its entries.
func (s *Store) DeletePlaylist(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM playlists WHERE id = $1`, id)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PlaylistTracks returns a playlist's tracks in playlist order.
func (s *Store) PlaylistTracks(ctx context.Context, id uuid.UUID) ([]Track, error) {
	query := `
		SELECT ` + trackColumns + `
		FROM playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id
		JOIN media_files f ON f.id = t.file_id
		WHERE pt.playlist_id = $1
		ORDER BY pt.position`

	rows, err := s.pool.Query(ctx, query, id)
	if err != nil {
		return nil, mapError(err)
	}
	return scanTracks(rows)
}

// SetPlaylistTracks replaces a playlist's contents with trackIDs, in order.
//
// Replace rather than patch: the client owns the ordering (it has a
// ReorderableListView), and a whole-list PUT is the only way to make reorder,
// insert and remove a single atomic operation.
func (s *Store) SetPlaylistTracks(ctx context.Context, id uuid.UUID, trackIDs []uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return mapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM playlist_tracks WHERE playlist_id = $1`, id); err != nil {
		return mapError(err)
	}

	if len(trackIDs) > 0 {
		rows := make([][]any, len(trackIDs))
		for i, trackID := range trackIDs {
			rows[i] = []any{id, trackID, i}
		}
		_, err := tx.CopyFrom(ctx,
			pgx.Identifier{"playlist_tracks"},
			[]string{"playlist_id", "track_id", "position"},
			pgx.CopyFromRows(rows),
		)
		if err != nil {
			return mapError(err)
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE playlists SET updated_at = now() WHERE id = $1`, id); err != nil {
		return mapError(err)
	}

	return mapError(tx.Commit(ctx))
}
