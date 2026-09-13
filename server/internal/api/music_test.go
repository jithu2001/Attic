package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/auth"
	"github.com/perleybrook/attic/server/internal/store"
)

// addTrackFile writes a file into the library root and registers a track for it.
func (ts *testServer) addTrackFile(t *testing.T, name string, content []byte) *store.Track {
	t.Helper()

	path := filepath.Join(ts.root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	track := &store.Track{
		ID:          uuid.New(),
		FileID:      uuid.New(),
		Path:        path,
		Title:       "Blue in Green",
		Artist:      "Miles Davis",
		Album:       "Kind of Blue",
		AlbumArtist: "Miles Davis",
		SizeBytes:   int64(len(content)),
	}
	ts.store.addTrack(track)
	return track
}

// mediaToken mints a media token for a signed-in user.
func (ts *testServer) mediaToken(t *testing.T, access string) string {
	t.Helper()
	rec := ts.do(t, http.MethodPost, "/api/v1/auth/media-token", nil, bearer(access))
	if rec.Code != http.StatusOK {
		t.Fatalf("media-token status = %d", rec.Code)
	}
	return decode[mediaTokenResponse](t, rec).Token
}

// audioFixture is 4 KB of recognisable bytes: every byte equals its index mod
// 251, so any served range can be checked against the offset it claims.
func audioFixture(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 251)
	}
	return data
}

func TestTrackAudioServesTheWholeFile(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	content := audioFixture(4096)
	track := ts.addTrackFile(t, "Miles Davis/Kind of Blue/02 Blue in Green.flac", content)

	rec := ts.do(t, http.MethodGet, "/api/v1/music/tracks/"+track.ID.String()+"/audio", nil, bearer(tokens.AccessToken))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "audio/flac" {
		t.Errorf("Content-Type = %q, want audio/flac", got)
	}
	// Accept-Ranges is what tells the player it may seek at all.
	if got := rec.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Errorf("Accept-Ranges = %q, want bytes", got)
	}
	if rec.Body.Len() != len(content) {
		t.Fatalf("body length = %d, want %d", rec.Body.Len(), len(content))
	}
	if !equalBytes(rec.Body.Bytes(), content) {
		t.Error("served bytes do not match the file")
	}
}

func TestTrackAudioServesByteRanges(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	content := audioFixture(4096)
	track := ts.addTrackFile(t, "Miles Davis/Kind of Blue/01 So What.flac", content)
	url := "/api/v1/music/tracks/" + track.ID.String() + "/audio"

	// This is the mechanism behind instant in-track seeking: the player asks
	// for the bytes at the position the user dropped the scrubber on, and gets
	// a 206 with exactly those bytes instead of the whole file.
	tests := []struct {
		name       string
		header     string
		wantStatus int
		wantFirst  int
		wantLast   int
	}{
		{"opening range", "bytes=0-1023", http.StatusPartialContent, 0, 1023},
		{"middle range", "bytes=2048-3071", http.StatusPartialContent, 2048, 3071},
		{"open-ended range", "bytes=4000-", http.StatusPartialContent, 4000, 4095},
		{"suffix range", "bytes=-96", http.StatusPartialContent, 4000, 4095},
		{"whole file", "bytes=0-4095", http.StatusPartialContent, 0, 4095},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
			req.Header.Set("Range", tc.header)
			rec := httptest.NewRecorder()
			ts.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			wantLen := tc.wantLast - tc.wantFirst + 1
			if rec.Body.Len() != wantLen {
				t.Fatalf("body length = %d, want %d", rec.Body.Len(), wantLen)
			}
			wantRange := fmt.Sprintf("bytes %d-%d/%d", tc.wantFirst, tc.wantLast, len(content))
			if got := rec.Header().Get("Content-Range"); got != wantRange {
				t.Errorf("Content-Range = %q, want %q", got, wantRange)
			}
			if got := rec.Header().Get("Content-Length"); got != strconv.Itoa(wantLen) {
				t.Errorf("Content-Length = %q, want %d", got, wantLen)
			}
			if !equalBytes(rec.Body.Bytes(), content[tc.wantFirst:tc.wantLast+1]) {
				t.Error("served bytes are not the requested range")
			}
		})
	}
}

func TestTrackAudioRejectsAnUnsatisfiableRange(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	content := audioFixture(1024)
	track := ts.addTrackFile(t, "a.flac", content)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/music/tracks/"+track.ID.String()+"/audio", nil)
	req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	req.Header.Set("Range", "bytes=99999-")
	rec := httptest.NewRecorder()
	ts.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("status = %d, want 416", rec.Code)
	}
}

func TestTrackAudioAcceptsAMediaTokenInTheQuery(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")
	token := ts.mediaToken(t, tokens.AccessToken)

	track := ts.addTrackFile(t, "b.mp3", audioFixture(512))

	rec := ts.do(t, http.MethodGet, "/api/v1/music/tracks/"+track.ID.String()+"/audio?token="+token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "audio/mpeg" {
		t.Errorf("Content-Type = %q, want audio/mpeg", got)
	}
}

func TestTrackAudioRequiresCredentials(t *testing.T) {
	ts := newTestServer(t)
	track := ts.addTrackFile(t, "c.mp3", audioFixture(16))

	rec := ts.do(t, http.MethodGet, "/api/v1/music/tracks/"+track.ID.String()+"/audio", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestTrackAudioRefusesPathsOutsideTheLibrary(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	// A row whose path escapes the library must not be servable, however it
	// got there.
	outside := filepath.Join(t.TempDir(), "secret.mp3")
	if err := os.WriteFile(outside, []byte("not yours"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	track := &store.Track{ID: uuid.New(), FileID: uuid.New(), Path: outside, Title: "x"}
	ts.store.addTrack(track)

	rec := ts.do(t, http.MethodGet, "/api/v1/music/tracks/"+track.ID.String()+"/audio", nil, bearer(tokens.AccessToken))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestTrackAudioRejectsAnUnknownTranscodeProfile(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")
	track := ts.addTrackFile(t, "d.mp3", audioFixture(16))

	rec := ts.do(t, http.MethodGet,
		"/api/v1/music/tracks/"+track.ID.String()+"/audio?transcode=mp3-320", nil, bearer(tokens.AccessToken))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestArtistsAndAlbums(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	hash := "ab12"
	ts.store.artists = []store.Artist{{Name: "Miles Davis", AlbumCount: 2, TrackCount: 12, CoverHash: &hash}}
	year := 1959
	ts.store.albums = []store.Album{{AlbumArtist: "Miles Davis", Name: "Kind of Blue", Year: &year, TrackCount: 5}}

	rec := ts.do(t, http.MethodGet, "/api/v1/music/artists", nil, bearer(tokens.AccessToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	artists := decode[struct {
		Artists []artistDTO `json:"artists"`
	}](t, rec).Artists
	if len(artists) != 1 {
		t.Fatalf("artist count = %d, want 1", len(artists))
	}
	if artists[0].CoverURL == nil || *artists[0].CoverURL != "/api/v1/covers/ab12" {
		t.Errorf("cover_url = %v", artists[0].CoverURL)
	}

	// The artist id round-trips into the albums endpoint.
	rec = ts.do(t, http.MethodGet, "/api/v1/music/artists/"+artists[0].ID+"/albums", nil, bearer(tokens.AccessToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("albums status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	albums := decode[struct {
		Albums []albumDTO `json:"albums"`
	}](t, rec).Albums
	if len(albums) != 1 || albums[0].Name != "Kind of Blue" {
		t.Fatalf("albums = %+v", albums)
	}
	if albums[0].Year == nil || *albums[0].Year != 1959 {
		t.Errorf("year = %v", albums[0].Year)
	}

	// And the album id round-trips into the detail endpoint.
	rec = ts.do(t, http.MethodGet, "/api/v1/music/albums/"+albums[0].ID, nil, bearer(tokens.AccessToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("album detail status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestUnknownArtistIDIsRejected(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	rec := ts.do(t, http.MethodGet, "/api/v1/music/artists/!!!not-base64!!!/albums", nil, bearer(tokens.AccessToken))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCoverIsImmutablyCacheable(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")

	coverPath := filepath.Join(t.TempDir(), "cover.jpg")
	if err := os.WriteFile(coverPath, []byte("\xff\xd8\xff\xe0jpeg"), 0o644); err != nil {
		t.Fatalf("write cover: %v", err)
	}
	ts.Server.covers = fakeCovers{"abc": coverPath}

	rec := ts.do(t, http.MethodGet, "/api/v1/covers/abc", nil, bearer(tokens.AccessToken))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control = %q", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("Content-Type = %q", got)
	}

	missing := ts.do(t, http.MethodGet, "/api/v1/covers/nope", nil, bearer(tokens.AccessToken))
	if missing.Code != http.StatusNotFound {
		t.Errorf("missing cover status = %d, want 404", missing.Code)
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
