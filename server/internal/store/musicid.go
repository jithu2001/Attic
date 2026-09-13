package store

import (
	"encoding/base64"
	"errors"
	"strings"
)

// Artists and albums have no rows of their own: they are groupings of tracks
// by `album_artist` and `album`, exactly as the schema describes. Their API ids
// are therefore derived from those names rather than stored, which keeps them
// stable across rescans (a rescan that rewrites every track row must not
// invalidate a client's cached album id).
//
// The separator is unit-separator rather than NUL because Postgres text columns
// cannot hold NUL, and these names come straight out of the database.
const albumIDSeparator = "\x1f"

// ErrBadID is returned when an artist or album id is not decodable.
var ErrBadID = errors.New("store: malformed id")

// EncodeArtistID derives the API id for an album artist.
func EncodeArtistID(albumArtist string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(albumArtist))
}

// DecodeArtistID recovers the album artist from an API id.
func DecodeArtistID(id string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return "", ErrBadID
	}
	name := string(raw)
	if name == "" {
		return "", ErrBadID
	}
	return name, nil
}

// EncodeAlbumID derives the API id for one album by one album artist.
func EncodeAlbumID(albumArtist, album string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(albumArtist + albumIDSeparator + album))
}

// DecodeAlbumID recovers the (album artist, album) pair from an API id.
func DecodeAlbumID(id string) (albumArtist, album string, err error) {
	raw, decodeErr := base64.RawURLEncoding.DecodeString(id)
	if decodeErr != nil {
		return "", "", ErrBadID
	}
	artist, name, found := strings.Cut(string(raw), albumIDSeparator)
	if !found {
		return "", "", ErrBadID
	}
	return artist, name, nil
}
