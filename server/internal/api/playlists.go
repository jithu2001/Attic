package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/store"
)

type playlistRequest struct {
	Name string `json:"name"`
}

type playlistTracksRequest struct {
	TrackIDs []string `json:"track_ids"`
}

// ListPlaylists returns the caller's playlists.
func (s *Server) ListPlaylists(w http.ResponseWriter, r *http.Request) {
	playlists, err := s.store.PlaylistsForUser(r.Context(), identity(r.Context()).UserID)
	if err != nil {
		s.internalError(w, r, "list playlists", err)
		return
	}

	out := make([]playlistDTO, 0, len(playlists))
	for _, p := range playlists {
		out = append(out, toPlaylistDTO(p))
	}
	WriteJSON(w, http.StatusOK, map[string]any{"playlists": out})
}

// CreatePlaylist makes a new empty playlist owned by the caller.
func (s *Server) CreatePlaylist(w http.ResponseWriter, r *http.Request) {
	var req playlistRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	name, ok := validPlaylistName(w, req.Name)
	if !ok {
		return
	}

	playlist, err := s.store.CreatePlaylist(r.Context(), identity(r.Context()).UserID, name)
	if err != nil {
		s.internalError(w, r, "create playlist", err)
		return
	}
	WriteJSON(w, http.StatusCreated, toPlaylistDTO(*playlist))
}

// GetPlaylist returns one playlist with its tracks in order.
func (s *Server) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	playlist, ok := s.loadOwnedPlaylist(w, r)
	if !ok {
		return
	}

	tracks, err := s.store.PlaylistTracks(r.Context(), playlist.ID)
	if err != nil {
		s.internalError(w, r, "load playlist tracks", err)
		return
	}

	WriteJSON(w, http.StatusOK, playlistDetailDTO{
		playlistDTO: toPlaylistDTO(*playlist),
		Tracks:      toTrackDTOs(tracks),
	})
}

// UpdatePlaylist renames a playlist.
func (s *Server) UpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	playlist, ok := s.loadOwnedPlaylist(w, r)
	if !ok {
		return
	}

	var req playlistRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	name, ok := validPlaylistName(w, req.Name)
	if !ok {
		return
	}

	if err := s.store.RenamePlaylist(r.Context(), playlist.ID, name); err != nil {
		s.internalError(w, r, "rename playlist", err)
		return
	}

	updated, err := s.store.Playlist(r.Context(), playlist.ID)
	if err != nil {
		s.internalError(w, r, "reload playlist", err)
		return
	}
	WriteJSON(w, http.StatusOK, toPlaylistDTO(*updated))
}

// DeletePlaylist removes a playlist.
func (s *Server) DeletePlaylist(w http.ResponseWriter, r *http.Request) {
	playlist, ok := s.loadOwnedPlaylist(w, r)
	if !ok {
		return
	}
	if err := s.store.DeletePlaylist(r.Context(), playlist.ID); err != nil {
		s.internalError(w, r, "delete playlist", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// SetPlaylistTracks replaces a playlist's contents with an ordered list of
// track ids. Replace rather than patch: reorder, insert and remove are one
// atomic operation, which is what a drag-to-reorder list actually needs.
func (s *Server) SetPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	playlist, ok := s.loadOwnedPlaylist(w, r)
	if !ok {
		return
	}

	var req playlistTracksRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	const maxTracks = 10000
	if len(req.TrackIDs) > maxTracks {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "That is more tracks than a playlist can hold.")
		return
	}

	ids := make([]uuid.UUID, 0, len(req.TrackIDs))
	for _, raw := range req.TrackIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			WriteError(w, http.StatusBadRequest, CodeBadRequest, "One of the track ids is not valid.")
			return
		}
		ids = append(ids, id)
	}

	// Verify every id exists before writing, so a stale client cannot leave a
	// playlist referencing tracks that were removed from the library.
	found, err := s.store.TracksByIDs(r.Context(), ids)
	if err != nil {
		s.internalError(w, r, "verify tracks", err)
		return
	}
	if len(found) != len(ids) {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "Some of those tracks are no longer in the library.")
		return
	}

	if err := s.store.SetPlaylistTracks(r.Context(), playlist.ID, ids); err != nil {
		s.internalError(w, r, "set playlist tracks", err)
		return
	}

	WriteJSON(w, http.StatusOK, playlistDetailDTO{
		playlistDTO: toPlaylistDTO(store.Playlist{
			ID:         playlist.ID,
			OwnerID:    playlist.OwnerID,
			Name:       playlist.Name,
			TrackCount: len(found),
			CreatedAt:  playlist.CreatedAt,
			UpdatedAt:  playlist.UpdatedAt,
		}),
		Tracks: toTrackDTOs(found),
	})
}

// loadOwnedPlaylist loads the {id} playlist and checks the caller owns it.
//
// A playlist belonging to someone else is reported as missing rather than
// forbidden: "403" would confirm that a given id exists.
func (s *Server) loadOwnedPlaylist(w http.ResponseWriter, r *http.Request) (*store.Playlist, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "That is not a valid playlist id.")
		return nil, false
	}

	playlist, err := s.store.Playlist(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			WriteError(w, http.StatusNotFound, CodeNotFound, "No such playlist.")
			return nil, false
		}
		s.internalError(w, r, "load playlist", err)
		return nil, false
	}

	if playlist.OwnerID != identity(r.Context()).UserID {
		WriteError(w, http.StatusNotFound, CodeNotFound, "No such playlist.")
		return nil, false
	}
	return playlist, true
}

func validPlaylistName(w http.ResponseWriter, raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	if name == "" {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "A playlist needs a name.")
		return "", false
	}
	if len([]rune(name)) > 200 {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "That name is too long.")
		return "", false
	}
	return name, true
}
