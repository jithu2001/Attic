package api

import (
	"net/http"
	"strings"

	"github.com/perleybrook/attic/server/internal/store"
)

// Search runs one full-text query across the music library and returns a mixed
// list of artists, albums and tracks, best match first.
func (s *Server) Search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		WriteJSON(w, http.StatusOK, map[string]any{"results": []searchResultDTO{}})
		return
	}

	limit := parseLimit(r, 20, 50)

	results, err := s.store.Search(r.Context(), query, limit)
	if err != nil {
		s.internalError(w, r, "search", err)
		return
	}

	out := make([]searchResultDTO, 0, len(results))
	for _, res := range results {
		out = append(out, toSearchResultDTO(res))
	}
	WriteJSON(w, http.StatusOK, map[string]any{"results": out})
}

func toSearchResultDTO(r store.SearchResult) searchResultDTO {
	dto := searchResultDTO{
		Kind:      r.Kind,
		Name:      r.Name,
		Subtitle:  r.Subtitle,
		CoverURL:  coverURL(r.CoverHash),
		DurationS: r.DurationS,
	}

	if r.Album != "" {
		dto.AlbumID = store.EncodeAlbumID(r.AlbumArtist, r.Album)
	}

	switch r.Kind {
	case "artist":
		dto.ID = store.EncodeArtistID(r.AlbumArtist)
	case "album":
		dto.ID = store.EncodeAlbumID(r.AlbumArtist, r.Album)
	case "track":
		if r.TrackID != nil {
			dto.ID = r.TrackID.String()
			url := audioURL(*r.TrackID)
			dto.AudioURL = &url
		}
	}
	return dto
}
