package api

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/auth"
	"github.com/perleybrook/attic/server/internal/store"
)

func TestSearchReturnsMixedResults(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	access := bearer(ts.login(t, "ada", testPassword, "Pixel").AccessToken)

	trackID := uuid.New()
	cover := "cafe"
	ts.store.results = []store.SearchResult{
		{Kind: "artist", Name: "Miles Davis", AlbumArtist: "Miles Davis"},
		{Kind: "album", Name: "Kind of Blue", Subtitle: "Miles Davis", AlbumArtist: "Miles Davis", Album: "Kind of Blue", CoverHash: &cover},
		{Kind: "track", Name: "So What", Subtitle: "Miles Davis", TrackID: &trackID,
			AlbumArtist: "Miles Davis", Album: "Kind of Blue"},
	}

	rec := ts.do(t, http.MethodGet, "/api/v1/search?q=miles", nil, access)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	results := decode[struct {
		Results []searchResultDTO `json:"results"`
	}](t, rec).Results
	if len(results) != 3 {
		t.Fatalf("result count = %d, want 3", len(results))
	}

	// Each kind carries the id its own endpoint expects.
	if got, want := results[0].ID, store.EncodeArtistID("Miles Davis"); got != want {
		t.Errorf("artist id = %q, want %q", got, want)
	}
	if got, want := results[1].ID, store.EncodeAlbumID("Miles Davis", "Kind of Blue"); got != want {
		t.Errorf("album id = %q, want %q", got, want)
	}
	if results[2].ID != trackID.String() {
		t.Errorf("track id = %q", results[2].ID)
	}
	// A track hit also carries the album it belongs to, so tapping it can open
	// the album rather than stranding the track on its own.
	if want := store.EncodeAlbumID("Miles Davis", "Kind of Blue"); results[1].AlbumID != want {
		t.Errorf("album hit album_id = %q, want %q", results[1].AlbumID, want)
	}
	// Only tracks are playable, so only tracks carry an audio URL.
	if results[0].AudioURL != nil || results[1].AudioURL != nil {
		t.Error("a non-track result carries an audio_url")
	}
	if results[2].AudioURL == nil {
		t.Error("track result has no audio_url")
	}
}

func TestSearchWithoutAQueryReturnsNothing(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	access := bearer(ts.login(t, "ada", testPassword, "Pixel").AccessToken)

	ts.store.results = []store.SearchResult{{Kind: "artist", Name: "Miles Davis"}}

	rec := ts.do(t, http.MethodGet, "/api/v1/search?q=%20%20", nil, access)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if results := decode[struct {
		Results []searchResultDTO `json:"results"`
	}](t, rec).Results; len(results) != 0 {
		t.Errorf("result count = %d, want 0", len(results))
	}
}

func TestSearchLimitIsBounded(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	access := bearer(ts.login(t, "ada", testPassword, "Pixel").AccessToken)

	for i := 0; i < 100; i++ {
		ts.store.results = append(ts.store.results, store.SearchResult{Kind: "artist", Name: "a", AlbumArtist: "a"})
	}

	rec := ts.do(t, http.MethodGet, "/api/v1/search?q=a&limit=1000", nil, access)
	results := decode[struct {
		Results []searchResultDTO `json:"results"`
	}](t, rec).Results
	if len(results) != 50 {
		t.Errorf("result count = %d, want the 50 cap", len(results))
	}
}
