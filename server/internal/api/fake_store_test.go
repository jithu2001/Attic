package api

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/store"
)

// fakeStore is an in-memory Store. It exists so the HTTP layer — auth flow,
// ownership checks, range serving — can be tested exhaustively without a
// Postgres to hand. Query behaviour itself is covered by the integration tests
// in the store package.
type fakeStore struct {
	mu sync.Mutex

	users     map[uuid.UUID]*store.User
	byName    map[string]uuid.UUID
	devices   map[string]*store.Device // keyed by hex of refresh hash
	tracks    map[uuid.UUID]*store.Track
	playlists map[uuid.UUID]*store.Playlist
	entries   map[uuid.UUID][]uuid.UUID

	artists []store.Artist
	albums  []store.Album
	results []store.SearchResult

	// failWith, when set, makes every method return it.
	failWith error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		users:     make(map[uuid.UUID]*store.User),
		byName:    make(map[string]uuid.UUID),
		devices:   make(map[string]*store.Device),
		tracks:    make(map[uuid.UUID]*store.Track),
		playlists: make(map[uuid.UUID]*store.Playlist),
		entries:   make(map[uuid.UUID][]uuid.UUID),
	}
}

func hexKey(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&0xf])
	}
	return string(out)
}

func (f *fakeStore) addUser(u *store.User) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.users[u.ID] = u
	f.byName[u.Username] = u.ID
}

func (f *fakeStore) addTrack(t *store.Track) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tracks[t.ID] = t
}

func (f *fakeStore) deviceCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.devices)
}

// ------------------------------------------------------------------ accounts --

func (f *fakeStore) UserByUsername(_ context.Context, username string) (*store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failWith != nil {
		return nil, f.failWith
	}
	id, ok := f.byName[username]
	if !ok {
		return nil, store.ErrNotFound
	}
	return f.users[id], nil
}

func (f *fakeStore) UserByID(_ context.Context, id uuid.UUID) (*store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failWith != nil {
		return nil, f.failWith
	}
	u, ok := f.users[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) UpdatePasswordHash(_ context.Context, id uuid.UUID, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return store.ErrNotFound
	}
	u.PasswordHash = hash
	return nil
}

// ------------------------------------------------------------------ sessions --

func (f *fakeStore) CreateDevice(_ context.Context, userID uuid.UUID, name string, refreshHash []byte, expiresAt time.Time) (*store.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failWith != nil {
		return nil, f.failWith
	}
	d := &store.Device{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
	}
	f.devices[hexKey(refreshHash)] = d
	return d, nil
}

func (f *fakeStore) RotateRefreshToken(_ context.Context, oldHash, newHash []byte, expiresAt time.Time) (*store.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failWith != nil {
		return nil, f.failWith
	}
	key := hexKey(oldHash)
	d, ok := f.devices[key]
	if !ok || !d.ExpiresAt.After(time.Now()) {
		return nil, store.ErrNotFound
	}
	delete(f.devices, key)
	d.ExpiresAt = expiresAt
	d.LastSeen = time.Now()
	f.devices[hexKey(newHash)] = d
	return d, nil
}

func (f *fakeStore) DeleteDeviceByRefreshToken(_ context.Context, refreshHash []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := hexKey(refreshHash)
	if _, ok := f.devices[key]; !ok {
		return store.ErrNotFound
	}
	delete(f.devices, key)
	return nil
}

func (f *fakeStore) DevicesForUser(_ context.Context, userID uuid.UUID) ([]store.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Device, 0, len(f.devices))
	for _, d := range f.devices {
		if d.UserID == userID {
			out = append(out, *d)
		}
	}
	return out, nil
}

// --------------------------------------------------------------------- music --

func (f *fakeStore) Artists(context.Context) ([]store.Artist, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.artists, f.failWith
}

func (f *fakeStore) AlbumsByArtist(_ context.Context, albumArtist string) ([]store.Album, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Album, 0, len(f.albums))
	for _, a := range f.albums {
		if a.AlbumArtist == albumArtist {
			out = append(out, a)
		}
	}
	return out, f.failWith
}

func (f *fakeStore) Album(_ context.Context, albumArtist, album string) (*store.Album, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.albums {
		if a.AlbumArtist == albumArtist && a.Name == album {
			found := a
			return &found, nil
		}
	}
	return nil, store.ErrNotFound
}

func (f *fakeStore) AlbumTracks(_ context.Context, albumArtist, album string) ([]store.Track, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Track, 0)
	for _, t := range f.tracks {
		if t.AlbumArtist == albumArtist && t.Album == album {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (f *fakeStore) TrackByID(_ context.Context, id uuid.UUID) (*store.Track, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.tracks[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return t, nil
}

func (f *fakeStore) TracksByIDs(_ context.Context, ids []uuid.UUID) ([]store.Track, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Track, 0, len(ids))
	for _, id := range ids {
		if t, ok := f.tracks[id]; ok {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (f *fakeStore) CountTracks(context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.tracks), nil
}

func (f *fakeStore) Search(_ context.Context, _ string, limit int) ([]store.SearchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.results) > limit {
		return f.results[:limit], f.failWith
	}
	return f.results, f.failWith
}

// ----------------------------------------------------------------- playlists --

func (f *fakeStore) CreatePlaylist(_ context.Context, ownerID uuid.UUID, name string) (*store.Playlist, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := &store.Playlist{
		ID:        uuid.New(),
		OwnerID:   ownerID,
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.playlists[p.ID] = p
	return p, nil
}

func (f *fakeStore) PlaylistsForUser(_ context.Context, ownerID uuid.UUID) ([]store.Playlist, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Playlist, 0, len(f.playlists))
	for _, p := range f.playlists {
		if p.OwnerID == ownerID {
			copy := *p
			copy.TrackCount = len(f.entries[p.ID])
			out = append(out, copy)
		}
	}
	return out, nil
}

func (f *fakeStore) Playlist(_ context.Context, id uuid.UUID) (*store.Playlist, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.playlists[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	copy := *p
	copy.TrackCount = len(f.entries[id])
	return &copy, nil
}

func (f *fakeStore) RenamePlaylist(_ context.Context, id uuid.UUID, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.playlists[id]
	if !ok {
		return store.ErrNotFound
	}
	p.Name = name
	p.UpdatedAt = time.Now()
	return nil
}

func (f *fakeStore) DeletePlaylist(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.playlists[id]; !ok {
		return store.ErrNotFound
	}
	delete(f.playlists, id)
	delete(f.entries, id)
	return nil
}

func (f *fakeStore) PlaylistTracks(_ context.Context, id uuid.UUID) ([]store.Track, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]store.Track, 0)
	for _, trackID := range f.entries[id] {
		if t, ok := f.tracks[trackID]; ok {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (f *fakeStore) SetPlaylistTracks(_ context.Context, id uuid.UUID, trackIDs []uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.playlists[id]; !ok {
		return store.ErrNotFound
	}
	f.entries[id] = append([]uuid.UUID(nil), trackIDs...)
	return nil
}

// fakeEnqueuer records scan requests.
type fakeEnqueuer struct {
	mu      sync.Mutex
	reasons []string
	err     error
}

func (f *fakeEnqueuer) ScanLibrary(_ context.Context, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.reasons = append(f.reasons, reason)
	return nil
}

func (f *fakeEnqueuer) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.reasons...)
}

// fakeCovers serves a fixed hash → path mapping.
type fakeCovers map[string]string

func (f fakeCovers) Find(hash string) (string, error) {
	if path, ok := f[hash]; ok {
		return path, nil
	}
	return "", store.ErrNotFound
}
