package store_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/auth"
	"github.com/perleybrook/attic/server/internal/store"
	"github.com/perleybrook/attic/server/migrations"
)

// These tests exercise the real SQL — migrations, generated columns, the
// full-text index, the ordering the app depends on — against a live Postgres.
//
// They are skipped unless ATTIC_TEST_DATABASE_URL points at a database the
// test may freely wipe, so `make server-test` stays runnable anywhere. Run them
// with: make server-test-integration
func testStore(t *testing.T) *store.Store {
	t.Helper()

	url := os.Getenv("ATTIC_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set ATTIC_TEST_DATABASE_URL to run store integration tests")
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := migrations.Up(url, log); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := store.Open(context.Background(), url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(db.Close)

	// Every test starts from an empty library. Cascades take the rest.
	if _, err := db.Pool().Exec(context.Background(),
		`TRUNCATE users, media_files RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return c
}

func mustUser(t *testing.T, db *store.Store, username string, role auth.Role) *store.User {
	t.Helper()
	user, err := db.CreateUser(ctx(t), username, "$argon2id$fake", role)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return user
}

// addTrack inserts a media file and its track, returning the track id.
func addTrack(t *testing.T, db *store.Store, path string, track store.Track) uuid.UUID {
	t.Helper()
	c := ctx(t)

	fileID, err := db.UpsertMediaFile(c, store.MediaFile{
		Path:      path,
		SizeBytes: 1024,
		SHA256:    "0000000000000000000000000000000000000000000000000000000000000000",
		MTime:     time.Now().UTC().Truncate(time.Microsecond),
	})
	if err != nil {
		t.Fatalf("UpsertMediaFile: %v", err)
	}

	track.FileID = fileID
	if err := db.UpsertTrack(c, track); err != nil {
		t.Fatalf("UpsertTrack: %v", err)
	}

	stored, err := db.AlbumTracks(c, track.AlbumArtist, track.Album)
	if err != nil {
		t.Fatalf("AlbumTracks: %v", err)
	}
	for _, s := range stored {
		if s.Path == path {
			return s.ID
		}
	}
	t.Fatalf("track %s was not stored", path)
	return uuid.Nil
}

func intPtr(v int) *int { return &v }

func TestMigrationsAndUserLifecycle(t *testing.T) {
	db := testStore(t)
	c := ctx(t)

	count, err := db.CountUsers(c)
	if err != nil {
		t.Fatalf("CountUsers: %v", err)
	}
	if count != 0 {
		t.Fatalf("CountUsers = %d on a fresh database, want 0", count)
	}

	user := mustUser(t, db, "ada", auth.RoleAdmin)
	if user.Role != auth.RoleAdmin {
		t.Errorf("role = %q", user.Role)
	}

	// The username unique constraint must surface as ErrConflict, which is
	// what the adduser CLI reports as "already taken".
	if _, err := db.CreateUser(c, "ada", "x", auth.RoleMember); err != store.ErrConflict {
		t.Errorf("duplicate username error = %v, want ErrConflict", err)
	}

	// The role check constraint must actually be enforced.
	if _, err := db.CreateUser(c, "bob", "x", auth.Role("superuser")); err == nil {
		t.Error("an invalid role was accepted")
	}

	found, err := db.UserByUsername(c, "ada")
	if err != nil || found.ID != user.ID {
		t.Fatalf("UserByUsername = %v, %v", found, err)
	}
	if _, err := db.UserByUsername(c, "nobody"); err != store.ErrNotFound {
		t.Errorf("UserByUsername for an unknown user = %v, want ErrNotFound", err)
	}
}

func TestRefreshTokenRotationIsAtomic(t *testing.T) {
	db := testStore(t)
	c := ctx(t)
	user := mustUser(t, db, "ada", auth.RoleMember)

	expires := time.Now().Add(time.Hour)
	device, err := db.CreateDevice(c, user.ID, "Pixel 8", []byte("hash-one"), expires)
	if err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}

	rotated, err := db.RotateRefreshToken(c, []byte("hash-one"), []byte("hash-two"), expires)
	if err != nil {
		t.Fatalf("RotateRefreshToken: %v", err)
	}
	if rotated.ID != device.ID {
		t.Error("rotation created a new device instead of updating one")
	}

	// The consumed token is gone: a second use of it, from anywhere, fails.
	if _, err := db.RotateRefreshToken(c, []byte("hash-one"), []byte("hash-three"), expires); err != store.ErrNotFound {
		t.Errorf("replayed rotation = %v, want ErrNotFound", err)
	}

	// An expired session cannot be refreshed back to life.
	if _, err := db.CreateDevice(c, user.ID, "Old TV", []byte("hash-old"), time.Now().Add(-time.Hour)); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	if _, err := db.RotateRefreshToken(c, []byte("hash-old"), []byte("hash-new"), expires); err != store.ErrNotFound {
		t.Errorf("rotating an expired session = %v, want ErrNotFound", err)
	}

	devices, err := db.DevicesForUser(c, user.ID)
	if err != nil {
		t.Fatalf("DevicesForUser: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("device count = %d, want 2", len(devices))
	}

	if err := db.DeleteDeviceByRefreshToken(c, []byte("hash-two")); err != nil {
		t.Fatalf("DeleteDeviceByRefreshToken: %v", err)
	}
	if err := db.DeleteDeviceByRefreshToken(c, []byte("hash-two")); err != store.ErrNotFound {
		t.Errorf("second delete = %v, want ErrNotFound", err)
	}

	purged, err := db.DeleteExpiredDevices(c)
	if err != nil {
		t.Fatalf("DeleteExpiredDevices: %v", err)
	}
	if purged != 1 {
		t.Errorf("purged %d expired devices, want 1", purged)
	}
}

func TestBrowseGroupsAndOrders(t *testing.T) {
	db := testStore(t)
	c := ctx(t)

	cover := "aa" + "00000000000000000000000000000000000000000000000000000000000000"
	duration := 300.0
	addTrack(t, db, "/music/miles/kob/02.flac", store.Track{
		Title: "Freddie Freeloader", AlbumArtist: "Miles Davis", Artist: "Miles Davis",
		Album: "Kind of Blue", TrackNo: intPtr(2), DiscNo: intPtr(1), Year: intPtr(1959),
		DurationS: &duration, CoverHash: &cover,
	})
	addTrack(t, db, "/music/miles/kob/01.flac", store.Track{
		Title: "So What", AlbumArtist: "Miles Davis", Artist: "Miles Davis",
		Album: "Kind of Blue", TrackNo: intPtr(1), DiscNo: intPtr(1), Year: intPtr(1959),
		DurationS: &duration,
	})
	addTrack(t, db, "/music/miles/bb/01.flac", store.Track{
		Title: "Pharaoh's Dance", AlbumArtist: "Miles Davis", Artist: "Miles Davis",
		Album: "Bitches Brew", TrackNo: intPtr(1), Year: intPtr(1970), DurationS: &duration,
	})
	addTrack(t, db, "/music/coltrane/01.flac", store.Track{
		Title: "Acknowledgement", AlbumArtist: "John Coltrane", Artist: "John Coltrane",
		Album: "A Love Supreme", TrackNo: intPtr(1), Year: intPtr(1965), DurationS: &duration,
	})

	artists, err := db.Artists(c)
	if err != nil {
		t.Fatalf("Artists: %v", err)
	}
	if len(artists) != 2 {
		t.Fatalf("artist count = %d, want 2", len(artists))
	}
	// Alphabetical, case-insensitive.
	if artists[0].Name != "John Coltrane" || artists[1].Name != "Miles Davis" {
		t.Errorf("artists = %q, %q", artists[0].Name, artists[1].Name)
	}
	if artists[1].AlbumCount != 2 || artists[1].TrackCount != 3 {
		t.Errorf("Miles: %d albums, %d tracks; want 2 and 3", artists[1].AlbumCount, artists[1].TrackCount)
	}
	// The album-art hash is picked up from whichever track carries one.
	if artists[1].CoverHash == nil {
		t.Error("Miles Davis has no cover hash, though one of his tracks does")
	}

	albums, err := db.AlbumsByArtist(c, "Miles Davis")
	if err != nil {
		t.Fatalf("AlbumsByArtist: %v", err)
	}
	if len(albums) != 2 {
		t.Fatalf("album count = %d, want 2", len(albums))
	}
	// Chronological: 1959 before 1970.
	if albums[0].Name != "Kind of Blue" || albums[1].Name != "Bitches Brew" {
		t.Errorf("albums = %q, %q; want them oldest first", albums[0].Name, albums[1].Name)
	}
	if albums[0].TrackCount != 2 || albums[0].DurationS != 600 {
		t.Errorf("Kind of Blue: %d tracks, %.0f seconds", albums[0].TrackCount, albums[0].DurationS)
	}

	tracks, err := db.AlbumTracks(c, "Miles Davis", "Kind of Blue")
	if err != nil {
		t.Fatalf("AlbumTracks: %v", err)
	}
	// Play order, not insertion order: disc then track number.
	if len(tracks) != 2 || tracks[0].Title != "So What" || tracks[1].Title != "Freddie Freeloader" {
		t.Errorf("track order = %v", tracks)
	}
	if tracks[0].Path != "/music/miles/kob/01.flac" {
		t.Errorf("path was not joined from media_files: %q", tracks[0].Path)
	}

	album, err := db.Album(c, "Miles Davis", "Kind of Blue")
	if err != nil {
		t.Fatalf("Album: %v", err)
	}
	if album.TrackCount != 2 {
		t.Errorf("track count = %d", album.TrackCount)
	}
	if _, err := db.Album(c, "Miles Davis", "No Such Album"); err != store.ErrNotFound {
		t.Errorf("Album for a missing album = %v, want ErrNotFound", err)
	}
}

func TestSearchFindsArtistsAlbumsAndTracks(t *testing.T) {
	db := testStore(t)
	c := ctx(t)

	addTrack(t, db, "/music/a.flac", store.Track{
		Title: "So What", AlbumArtist: "Miles Davis", Artist: "Miles Davis", Album: "Kind of Blue",
	})
	addTrack(t, db, "/music/b.flac", store.Track{
		Title: "Blue in Green", AlbumArtist: "Miles Davis", Artist: "Miles Davis", Album: "Kind of Blue",
	})
	addTrack(t, db, "/music/c.flac", store.Track{
		Title: "Acknowledgement", AlbumArtist: "John Coltrane", Artist: "John Coltrane", Album: "A Love Supreme",
	})

	results, err := db.Search(c, "blue", 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("searching for 'blue' found nothing")
	}

	kinds := map[string]int{}
	var sawAlbum, sawTrack bool
	for _, r := range results {
		kinds[r.Kind]++
		if r.Kind == "album" && r.Name == "Kind of Blue" {
			sawAlbum = true
		}
		if r.Kind == "track" && r.Name == "Blue in Green" {
			sawTrack = true
		}
	}
	if !sawAlbum {
		t.Error("the album 'Kind of Blue' did not come back")
	}
	if !sawTrack {
		t.Error("the track 'Blue in Green' did not come back")
	}

	// An artist name matches through the C-weighted column.
	results, err = db.Search(c, "coltrane", 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	var sawArtist bool
	for _, r := range results {
		if r.Kind == "artist" && r.Name == "John Coltrane" {
			sawArtist = true
		}
	}
	if !sawArtist {
		t.Error("searching for 'coltrane' did not return the artist")
	}

	// Nothing matching is an empty result, not an error.
	results, err = db.Search(c, "zzzzznotamatch", 20)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("result count = %d, want 0", len(results))
	}

	// The limit is honoured.
	results, err = db.Search(c, "blue", 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("result count = %d, want 1", len(results))
	}
}

func TestScannerStoreDiffSupport(t *testing.T) {
	db := testStore(t)
	c := ctx(t)

	mtime := time.Now().UTC().Truncate(time.Microsecond)
	for _, path := range []string{
		"/music/artist/album/01.flac",
		"/music/artist/album/02.flac",
		"/music/artist/album/deeper/03.flac",
		"/music/other/01.flac",
	} {
		if _, err := db.UpsertMediaFile(c, store.MediaFile{
			Path: path, SizeBytes: 10, SHA256: "abc", MTime: mtime,
		}); err != nil {
			t.Fatalf("UpsertMediaFile: %v", err)
		}
	}

	// A directory scan sees its own files, and not the ones nested below it.
	stamps, err := db.FileStampsInDir(c, "/music/artist/album")
	if err != nil {
		t.Fatalf("FileStampsInDir: %v", err)
	}
	if len(stamps) != 2 {
		t.Fatalf("direct children = %d, want 2: %v", len(stamps), stamps)
	}
	if _, ok := stamps["/music/artist/album/deeper/03.flac"]; ok {
		t.Error("FileStampsInDir descended into a subdirectory")
	}

	// Round-tripping an mtime through Postgres must not make a file look
	// changed, or every scan would re-hash the whole library.
	stamp := stamps["/music/artist/album/01.flac"]
	if !stamp.MTime.Equal(mtime) {
		t.Errorf("mtime round-tripped as %v, want %v", stamp.MTime, mtime)
	}

	// The prune pass sees the whole subtree.
	known, err := db.KnownFilesUnder(c, "/music")
	if err != nil {
		t.Fatalf("KnownFilesUnder: %v", err)
	}
	if len(known) != 4 {
		t.Fatalf("known files = %d, want 4", len(known))
	}

	// Upserting the same path updates rather than duplicating.
	if _, err := db.UpsertMediaFile(c, store.MediaFile{
		Path: "/music/other/01.flac", SizeBytes: 99, SHA256: "def", MTime: mtime,
	}); err != nil {
		t.Fatalf("UpsertMediaFile: %v", err)
	}
	again, err := db.KnownFilesUnder(c, "/music")
	if err != nil {
		t.Fatalf("KnownFilesUnder: %v", err)
	}
	if len(again) != 4 {
		t.Errorf("known files after re-upsert = %d, want 4", len(again))
	}

	deleted, err := db.DeleteMediaFiles(c, []uuid.UUID{known[0].ID})
	if err != nil {
		t.Fatalf("DeleteMediaFiles: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
}

func TestTrackUpsertReplacesTagsWithoutDuplicating(t *testing.T) {
	db := testStore(t)
	c := ctx(t)

	id := addTrack(t, db, "/music/a.flac", store.Track{
		Title: "Old Title", AlbumArtist: "Artist", Artist: "Artist", Album: "Album",
	})

	// Re-tagging a file must update its track, not create a second one.
	addTrack(t, db, "/music/a.flac", store.Track{
		Title: "New Title", AlbumArtist: "Artist", Artist: "Artist", Album: "Album",
	})

	count, err := db.CountTracks(c)
	if err != nil {
		t.Fatalf("CountTracks: %v", err)
	}
	if count != 1 {
		t.Fatalf("track count = %d, want 1", count)
	}

	track, err := db.TrackByID(c, id)
	if err != nil {
		t.Fatalf("TrackByID: %v", err)
	}
	if track.Title != "New Title" {
		t.Errorf("title = %q, want the re-read tag", track.Title)
	}

	// Deleting the file cascades the track away.
	known, _ := db.KnownFilesUnder(c, "/music")
	if _, err := db.DeleteMediaFiles(c, []uuid.UUID{known[0].ID}); err != nil {
		t.Fatalf("DeleteMediaFiles: %v", err)
	}
	if count, _ := db.CountTracks(c); count != 0 {
		t.Errorf("track count after deleting the file = %d, want 0", count)
	}
}

func TestPlaylistOrderingAndReplacement(t *testing.T) {
	db := testStore(t)
	c := ctx(t)
	user := mustUser(t, db, "ada", auth.RoleMember)

	first := addTrack(t, db, "/music/1.flac", store.Track{Title: "One", AlbumArtist: "A", Album: "X"})
	second := addTrack(t, db, "/music/2.flac", store.Track{Title: "Two", AlbumArtist: "A", Album: "X"})
	third := addTrack(t, db, "/music/3.flac", store.Track{Title: "Three", AlbumArtist: "A", Album: "X"})

	playlist, err := db.CreatePlaylist(c, user.ID, "Late night")
	if err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	if err := db.SetPlaylistTracks(c, playlist.ID, []uuid.UUID{third, first, second}); err != nil {
		t.Fatalf("SetPlaylistTracks: %v", err)
	}

	tracks, err := db.PlaylistTracks(c, playlist.ID)
	if err != nil {
		t.Fatalf("PlaylistTracks: %v", err)
	}
	if len(tracks) != 3 || tracks[0].ID != third || tracks[1].ID != first || tracks[2].ID != second {
		t.Fatalf("playlist order was not preserved: %v", tracks)
	}

	// A reorder is a whole-list replacement, and must not accumulate rows.
	if err := db.SetPlaylistTracks(c, playlist.ID, []uuid.UUID{first, first}); err != nil {
		t.Fatalf("SetPlaylistTracks: %v", err)
	}
	tracks, _ = db.PlaylistTracks(c, playlist.ID)
	// The same track twice is legal: position, not track id, is the key.
	if len(tracks) != 2 || tracks[0].ID != first || tracks[1].ID != first {
		t.Fatalf("duplicate entries = %v", tracks)
	}

	// Emptying works too.
	if err := db.SetPlaylistTracks(c, playlist.ID, nil); err != nil {
		t.Fatalf("SetPlaylistTracks(nil): %v", err)
	}
	if tracks, _ := db.PlaylistTracks(c, playlist.ID); len(tracks) != 0 {
		t.Errorf("emptied playlist still has %d tracks", len(tracks))
	}

	reloaded, err := db.Playlist(c, playlist.ID)
	if err != nil {
		t.Fatalf("Playlist: %v", err)
	}
	if reloaded.TrackCount != 0 {
		t.Errorf("track count = %d, want 0", reloaded.TrackCount)
	}
	if !reloaded.UpdatedAt.After(playlist.UpdatedAt) && !reloaded.UpdatedAt.Equal(playlist.UpdatedAt) {
		t.Error("updated_at went backwards")
	}

	if err := db.RenamePlaylist(c, playlist.ID, "Early morning"); err != nil {
		t.Fatalf("RenamePlaylist: %v", err)
	}
	if err := db.DeletePlaylist(c, playlist.ID); err != nil {
		t.Fatalf("DeletePlaylist: %v", err)
	}
	if err := db.DeletePlaylist(c, playlist.ID); err != store.ErrNotFound {
		t.Errorf("second delete = %v, want ErrNotFound", err)
	}
	_ = third
}

func TestTracksByIDsPreservesRequestOrder(t *testing.T) {
	db := testStore(t)
	c := ctx(t)

	a := addTrack(t, db, "/music/a.flac", store.Track{Title: "A", AlbumArtist: "X", Album: "Y"})
	b := addTrack(t, db, "/music/b.flac", store.Track{Title: "B", AlbumArtist: "X", Album: "Y"})

	tracks, err := db.TracksByIDs(c, []uuid.UUID{b, a})
	if err != nil {
		t.Fatalf("TracksByIDs: %v", err)
	}
	if len(tracks) != 2 || tracks[0].ID != b || tracks[1].ID != a {
		t.Fatalf("order was not preserved: %v", tracks)
	}

	// Unknown ids are dropped rather than erroring, so the caller can compare
	// lengths to detect them.
	tracks, err = db.TracksByIDs(c, []uuid.UUID{a, uuid.New()})
	if err != nil {
		t.Fatalf("TracksByIDs: %v", err)
	}
	if len(tracks) != 1 {
		t.Errorf("track count = %d, want 1", len(tracks))
	}
}
