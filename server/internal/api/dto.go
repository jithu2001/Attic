package api

import (
	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/media"
	"github.com/perleybrook/attic/server/internal/store"
)

// The JSON shapes below are the API contract. Fields are snake_case, ids are
// strings, timestamps are UTC ISO-8601, and URLs are server-relative so the
// client can prefix whichever address it reached us on.

type artistDTO struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	AlbumCount int     `json:"album_count"`
	TrackCount int     `json:"track_count"`
	CoverURL   *string `json:"cover_url"`
}

type albumDTO struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	AlbumArtist string  `json:"album_artist"`
	ArtistID    string  `json:"artist_id"`
	Year        *int    `json:"year"`
	TrackCount  int     `json:"track_count"`
	DurationS   float64 `json:"duration_s"`
	CoverURL    *string `json:"cover_url"`
}

type albumDetailDTO struct {
	albumDTO
	Tracks []trackDTO `json:"tracks"`
}

type trackDTO struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Artist      string   `json:"artist"`
	Album       string   `json:"album"`
	AlbumArtist string   `json:"album_artist"`
	AlbumID     string   `json:"album_id"`
	ArtistID    string   `json:"artist_id"`
	TrackNo     *int     `json:"track_no"`
	DiscNo      *int     `json:"disc_no"`
	Year        *int     `json:"year"`
	DurationS   *float64 `json:"duration_s"`
	SizeBytes   int64    `json:"size_bytes"`
	MIMEType    string   `json:"mime_type"`
	CoverURL    *string  `json:"cover_url"`
	AudioURL    string   `json:"audio_url"`
}

type searchResultDTO struct {
	Kind     string `json:"kind"` // artist | album | track
	ID       string `json:"id"`
	Name     string `json:"name"`
	Subtitle string `json:"subtitle"`

	// AlbumID lets a track hit open in its album, which is where it can be
	// played in context rather than on its own.
	AlbumID   string   `json:"album_id"`
	CoverURL  *string  `json:"cover_url"`
	AudioURL  *string  `json:"audio_url"`
	DurationS *float64 `json:"duration_s"`
}

type playlistDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	TrackCount int    `json:"track_count"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type playlistDetailDTO struct {
	playlistDTO
	Tracks []trackDTO `json:"tracks"`
}

type deviceDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	LastSeen string `json:"last_seen"`
	Current  bool   `json:"current"`
}

type userDTO struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
}

// ------------------------------------------------------------- conversions --

func coverURL(hash *string) *string {
	if hash == nil || *hash == "" {
		return nil
	}
	url := "/api/v1/covers/" + *hash
	return &url
}

func audioURL(id uuid.UUID) string {
	return "/api/v1/music/tracks/" + id.String() + "/audio"
}

func toArtistDTO(a store.Artist) artistDTO {
	return artistDTO{
		ID:         store.EncodeArtistID(a.Name),
		Name:       a.Name,
		AlbumCount: a.AlbumCount,
		TrackCount: a.TrackCount,
		CoverURL:   coverURL(a.CoverHash),
	}
}

func toAlbumDTO(a store.Album) albumDTO {
	return albumDTO{
		ID:          store.EncodeAlbumID(a.AlbumArtist, a.Name),
		Name:        a.Name,
		AlbumArtist: a.AlbumArtist,
		ArtistID:    store.EncodeArtistID(a.AlbumArtist),
		Year:        a.Year,
		TrackCount:  a.TrackCount,
		DurationS:   a.DurationS,
		CoverURL:    coverURL(a.CoverHash),
	}
}

func toTrackDTO(t store.Track) trackDTO {
	return trackDTO{
		ID:          t.ID.String(),
		Title:       t.Title,
		Artist:      t.Artist,
		Album:       t.Album,
		AlbumArtist: t.AlbumArtist,
		AlbumID:     store.EncodeAlbumID(t.AlbumArtist, t.Album),
		ArtistID:    store.EncodeArtistID(t.AlbumArtist),
		TrackNo:     t.TrackNo,
		DiscNo:      t.DiscNo,
		Year:        t.Year,
		DurationS:   t.DurationS,
		SizeBytes:   t.SizeBytes,
		MIMEType:    media.AudioMIME(t.Path),
		CoverURL:    coverURL(t.CoverHash),
		AudioURL:    audioURL(t.ID),
	}
}

func toTrackDTOs(tracks []store.Track) []trackDTO {
	out := make([]trackDTO, 0, len(tracks))
	for _, t := range tracks {
		out = append(out, toTrackDTO(t))
	}
	return out
}

func toPlaylistDTO(p store.Playlist) playlistDTO {
	return playlistDTO{
		ID:         p.ID.String(),
		Name:       p.Name,
		TrackCount: p.TrackCount,
		CreatedAt:  p.CreatedAt.UTC().Format(timeFormat),
		UpdatedAt:  p.UpdatedAt.UTC().Format(timeFormat),
	}
}

func toUserDTO(u store.User) userDTO {
	return userDTO{
		ID:        u.ID.String(),
		Username:  u.Username,
		Role:      string(u.Role),
		CreatedAt: u.CreatedAt.UTC().Format(timeFormat),
	}
}

// timeFormat is RFC 3339 in UTC, as the API conventions require.
const timeFormat = "2006-01-02T15:04:05Z07:00"
