package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/media"
	"github.com/perleybrook/attic/server/internal/store"
)

// Artists lists album artists.
//
// The library's artist count is bounded by human patience rather than by data
// volume — a 100k-track library still has a few thousand artists — so this is
// deliberately unpaginated. The endpoints that grow without bound (the photo
// and video timelines) use cursors.
func (s *Server) Artists(w http.ResponseWriter, r *http.Request) {
	artists, err := s.store.Artists(r.Context())
	if err != nil {
		s.internalError(w, r, "list artists", err)
		return
	}

	out := make([]artistDTO, 0, len(artists))
	for _, a := range artists {
		out = append(out, toArtistDTO(a))
	}
	WriteJSON(w, http.StatusOK, map[string]any{"artists": out})
}

// ArtistAlbums lists one artist's albums.
func (s *Server) ArtistAlbums(w http.ResponseWriter, r *http.Request) {
	albumArtist, err := store.DecodeArtistID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "That is not a valid artist id.")
		return
	}

	albums, err := s.store.AlbumsByArtist(r.Context(), albumArtist)
	if err != nil {
		s.internalError(w, r, "list albums", err)
		return
	}
	if len(albums) == 0 {
		WriteError(w, http.StatusNotFound, CodeNotFound, "No such artist.")
		return
	}

	out := make([]albumDTO, 0, len(albums))
	for _, a := range albums {
		out = append(out, toAlbumDTO(a))
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"artist": artistDTO{ID: store.EncodeArtistID(albumArtist), Name: albumArtist, AlbumCount: len(albums)},
		"albums": out,
	})
}

// AlbumDetail returns an album with its tracks in play order.
func (s *Server) AlbumDetail(w http.ResponseWriter, r *http.Request) {
	albumArtist, albumName, err := store.DecodeAlbumID(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "That is not a valid album id.")
		return
	}

	album, err := s.store.Album(r.Context(), albumArtist, albumName)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			WriteError(w, http.StatusNotFound, CodeNotFound, "No such album.")
			return
		}
		s.internalError(w, r, "load album", err)
		return
	}

	tracks, err := s.store.AlbumTracks(r.Context(), albumArtist, albumName)
	if err != nil {
		s.internalError(w, r, "load album tracks", err)
		return
	}

	WriteJSON(w, http.StatusOK, albumDetailDTO{
		albumDTO: toAlbumDTO(*album),
		Tracks:   toTrackDTOs(tracks),
	})
}

// TrackDetail returns one track's metadata.
func (s *Server) TrackDetail(w http.ResponseWriter, r *http.Request) {
	track, ok := s.loadTrack(w, r)
	if !ok {
		return
	}
	WriteJSON(w, http.StatusOK, toTrackDTO(*track))
}

// TrackAudio streams a track's audio.
//
// Two modes. By default the original file is served through http.ServeContent,
// which answers Range requests with 206 and a Content-Range — that is what
// makes seeking in a long FLAC instant. With ?transcode=opus128 the file is
// piped through a single FFmpeg process instead, for links too slow for
// lossless; that stream is sequential and cannot be seeked server-side.
func (s *Server) TrackAudio(w http.ResponseWriter, r *http.Request) {
	track, ok := s.loadTrack(w, r)
	if !ok {
		return
	}

	if profile := r.URL.Query().Get("transcode"); profile != "" {
		s.transcodeAudio(w, r, track, profile)
		return
	}

	file, err := s.guard.Open(track.Path)
	if err != nil {
		s.missingMedia(w, r, track.Path, err)
		return
	}
	defer file.Close()

	serveOriginal(w, r, file, media.AudioMIME(track.Path))
}

func (s *Server) transcodeAudio(w http.ResponseWriter, r *http.Request, track *store.Track, profile string) {
	bitrate, ok := transcodeProfiles[profile]
	if !ok {
		WriteError(w, http.StatusBadRequest, CodeBadRequest,
			"Unknown transcode profile. Supported: "+strings.Join(transcodeProfileNames(), ", ")+".")
		return
	}
	if s.ffmpeg == nil {
		WriteError(w, http.StatusServiceUnavailable, CodeInternal, "Transcoding is not available on this server.")
		return
	}

	path, err := s.guard.Resolve(track.Path)
	if err != nil {
		s.missingMedia(w, r, track.Path, err)
		return
	}

	// Length is unknown up front, so the response is chunked and unseekable.
	// Say so, rather than letting the client think it can range over it.
	w.Header().Set("Content-Type", "audio/webm")
	w.Header().Set("Accept-Ranges", "none")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)

	if err := s.ffmpeg.TranscodeAudio(r.Context(), path, bitrate, &flushWriter{w: w}); err != nil {
		if r.Context().Err() != nil {
			return // client hung up; nothing to report
		}
		// Headers are already out, so the only honest signal left is to cut
		// the response short.
		s.log.Error("transcode failed", "path", path, "error", err)
	}
}

// transcodeProfiles maps a profile name to an Opus bitrate in kbps.
var transcodeProfiles = map[string]int{
	"opus128": 128,
	"opus96":  96,
	"opus64":  64,
}

func transcodeProfileNames() []string {
	names := make([]string, 0, len(transcodeProfiles))
	for name := range transcodeProfiles {
		names = append(names, name)
	}
	sortStrings(names)
	return names
}

// loadTrack parses the {id} parameter and loads the track behind it.
func (s *Server) loadTrack(w http.ResponseWriter, r *http.Request) (*store.Track, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, CodeBadRequest, "That is not a valid track id.")
		return nil, false
	}

	track, err := s.store.TrackByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			WriteError(w, http.StatusNotFound, CodeNotFound, "No such track.")
			return nil, false
		}
		s.internalError(w, r, "load track", err)
		return nil, false
	}
	return track, true
}

// missingMedia reports a row whose file cannot be served. That is a 404 to the
// client — the track is not playable — but a warning in the log, because it
// means the library and the database have drifted apart.
func (s *Server) missingMedia(w http.ResponseWriter, r *http.Request, path string, err error) {
	s.log.Warn("media file is not servable", "path", path, "error", err)
	WriteError(w, http.StatusNotFound, CodeNotFound, "That file is no longer available.")
}

// parseLimit reads a bounded ?limit= parameter.
func parseLimit(r *http.Request, def, max int) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
