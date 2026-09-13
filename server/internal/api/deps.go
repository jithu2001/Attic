package api

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/auth"
	"github.com/perleybrook/attic/server/internal/store"
)

// Store is the database surface the HTTP handlers use.
//
// It is declared here, at the consumer, rather than in the store package: that
// keeps the dependency one-way and lets the handler tests run against an
// in-memory fake instead of a live Postgres.
type Store interface {
	// accounts
	UserByUsername(ctx context.Context, username string) (*store.User, error)
	UserByID(ctx context.Context, id uuid.UUID) (*store.User, error)
	UpdatePasswordHash(ctx context.Context, id uuid.UUID, hash string) error

	// sessions
	CreateDevice(ctx context.Context, userID uuid.UUID, name string, refreshHash []byte, expiresAt time.Time) (*store.Device, error)
	RotateRefreshToken(ctx context.Context, oldHash, newHash []byte, expiresAt time.Time) (*store.Device, error)
	DeleteDeviceByRefreshToken(ctx context.Context, refreshHash []byte) error
	DevicesForUser(ctx context.Context, userID uuid.UUID) ([]store.Device, error)

	// music
	Artists(ctx context.Context) ([]store.Artist, error)
	AlbumsByArtist(ctx context.Context, albumArtist string) ([]store.Album, error)
	Album(ctx context.Context, albumArtist, album string) (*store.Album, error)
	AlbumTracks(ctx context.Context, albumArtist, album string) ([]store.Track, error)
	TrackByID(ctx context.Context, id uuid.UUID) (*store.Track, error)
	TracksByIDs(ctx context.Context, ids []uuid.UUID) ([]store.Track, error)
	CountTracks(ctx context.Context) (int, error)
	Search(ctx context.Context, q string, limit int) ([]store.SearchResult, error)

	// playlists
	CreatePlaylist(ctx context.Context, ownerID uuid.UUID, name string) (*store.Playlist, error)
	PlaylistsForUser(ctx context.Context, ownerID uuid.UUID) ([]store.Playlist, error)
	Playlist(ctx context.Context, id uuid.UUID) (*store.Playlist, error)
	RenamePlaylist(ctx context.Context, id uuid.UUID, name string) error
	DeletePlaylist(ctx context.Context, id uuid.UUID) error
	PlaylistTracks(ctx context.Context, id uuid.UUID) ([]store.Track, error)
	SetPlaylistTracks(ctx context.Context, id uuid.UUID, trackIDs []uuid.UUID) error
}

// Enqueuer triggers background work. Nil in tests and when the job runner is
// unavailable; handlers must cope.
type Enqueuer interface {
	ScanLibrary(ctx context.Context, reason string) error
}

// CoverLookup finds a stored cover image by hash.
type CoverLookup interface {
	Find(hash string) (string, error)
}

// Transcoder re-encodes audio on the fly.
type Transcoder interface {
	TranscodeAudio(ctx context.Context, path string, bitrateKbps int, w interface{ Write([]byte) (int, error) }) error
}

// identity pulls the authenticated caller out of a request context. Handlers
// behind the auth middleware can rely on it being present.
func identity(ctx context.Context) auth.Identity {
	id, _ := auth.IdentityFrom(ctx)
	return id
}
