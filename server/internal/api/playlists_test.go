package api

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/auth"
)

func TestPlaylistLifecycle(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	tokens := ts.login(t, "ada", testPassword, "Pixel 8")
	access := bearer(tokens.AccessToken)

	one := ts.addTrackFile(t, "one.mp3", audioFixture(8))
	two := ts.addTrackFile(t, "two.mp3", audioFixture(8))

	// create
	rec := ts.do(t, http.MethodPost, "/api/v1/playlists", playlistRequest{Name: "  Late night  "}, access)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	created := decode[playlistDTO](t, rec)
	if created.Name != "Late night" {
		t.Errorf("name = %q, want it trimmed", created.Name)
	}

	// set tracks
	rec = ts.do(t, http.MethodPut, "/api/v1/playlists/"+created.ID+"/tracks",
		playlistTracksRequest{TrackIDs: []string{two.ID.String(), one.ID.String()}}, access)
	if rec.Code != http.StatusOK {
		t.Fatalf("set tracks status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	detail := decode[playlistDetailDTO](t, rec)
	if len(detail.Tracks) != 2 {
		t.Fatalf("track count = %d, want 2", len(detail.Tracks))
	}
	// Order is the client's, and must survive the round trip exactly.
	if detail.Tracks[0].ID != two.ID.String() || detail.Tracks[1].ID != one.ID.String() {
		t.Errorf("track order was not preserved: %s, %s", detail.Tracks[0].ID, detail.Tracks[1].ID)
	}

	// rename
	rec = ts.do(t, http.MethodPut, "/api/v1/playlists/"+created.ID, playlistRequest{Name: "Early morning"}, access)
	if rec.Code != http.StatusOK {
		t.Fatalf("rename status = %d, want 200", rec.Code)
	}
	if renamed := decode[playlistDTO](t, rec); renamed.Name != "Early morning" {
		t.Errorf("name = %q", renamed.Name)
	}

	// list
	rec = ts.do(t, http.MethodGet, "/api/v1/playlists", nil, access)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rec.Code)
	}
	list := decode[struct {
		Playlists []playlistDTO `json:"playlists"`
	}](t, rec).Playlists
	if len(list) != 1 || list[0].TrackCount != 2 {
		t.Fatalf("list = %+v", list)
	}

	// delete
	rec = ts.do(t, http.MethodDelete, "/api/v1/playlists/"+created.ID, nil, access)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", rec.Code)
	}
	rec = ts.do(t, http.MethodGet, "/api/v1/playlists/"+created.ID, nil, access)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", rec.Code)
	}
}

func TestPlaylistsAreNotVisibleToOtherUsers(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	ts.addUser(t, "bob", auth.RoleMember)

	ada := bearer(ts.login(t, "ada", testPassword, "Pixel").AccessToken)
	bob := bearer(ts.login(t, "bob", testPassword, "Tablet").AccessToken)

	rec := ts.do(t, http.MethodPost, "/api/v1/playlists", playlistRequest{Name: "Ada's mix"}, ada)
	created := decode[playlistDTO](t, rec)

	// Someone else's playlist reads as missing, not forbidden: a 403 would
	// confirm the id exists.
	for _, call := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/v1/playlists/" + created.ID, nil},
		{http.MethodPut, "/api/v1/playlists/" + created.ID, playlistRequest{Name: "hijacked"}},
		{http.MethodDelete, "/api/v1/playlists/" + created.ID, nil},
		{http.MethodPut, "/api/v1/playlists/" + created.ID + "/tracks", playlistTracksRequest{}},
	} {
		rec := ts.do(t, call.method, call.path, call.body, bob)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s: status = %d, want 404", call.method, call.path, rec.Code)
		}
	}

	// Bob's own list stays empty.
	rec = ts.do(t, http.MethodGet, "/api/v1/playlists", nil, bob)
	if list := decode[struct {
		Playlists []playlistDTO `json:"playlists"`
	}](t, rec).Playlists; len(list) != 0 {
		t.Errorf("bob sees %d playlists, want 0", len(list))
	}
}

func TestPlaylistRejectsUnknownTracks(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	access := bearer(ts.login(t, "ada", testPassword, "Pixel").AccessToken)

	created := decode[playlistDTO](t, ts.do(t, http.MethodPost, "/api/v1/playlists", playlistRequest{Name: "Mix"}, access))

	rec := ts.do(t, http.MethodPut, "/api/v1/playlists/"+created.ID+"/tracks",
		playlistTracksRequest{TrackIDs: []string{uuid.NewString()}}, access)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestPlaylistNeedsAName(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	access := bearer(ts.login(t, "ada", testPassword, "Pixel").AccessToken)

	rec := ts.do(t, http.MethodPost, "/api/v1/playlists", playlistRequest{Name: "   "}, access)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestUnknownJSONFieldsAreRejected(t *testing.T) {
	ts := newTestServer(t)
	ts.addUser(t, "ada", auth.RoleMember)
	access := bearer(ts.login(t, "ada", testPassword, "Pixel").AccessToken)

	rec := ts.do(t, http.MethodPost, "/api/v1/playlists", map[string]any{"name": "Mix", "owner_id": "someone-else"}, access)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
