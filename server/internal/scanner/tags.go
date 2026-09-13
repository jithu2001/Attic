package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"

	"github.com/perleybrook/attic/server/internal/store"
)

// trackTags is the metadata read off one file, already normalised.
type trackTags struct {
	Title         string
	Artist        string
	Album         string
	AlbumArtist   string
	TrackNo       *int
	DiscNo        *int
	Year          *int
	Genre         *string
	MusicBrainzID *string
	CoverHash     *string
}

// readTags reads embedded tags and extracts cover art.
func (s *Scanner) readTags(path string) (trackTags, error) {
	f, err := os.Open(path)
	if err != nil {
		return trackTags{}, fmt.Errorf("scanner: open %s: %w", path, err)
	}
	defer f.Close()

	meta, err := tag.ReadFrom(f)
	if err != nil {
		// An untagged or oddly tagged file still belongs in the library; fall
		// back to its filename rather than dropping it.
		s.log.Debug("no readable tags, using filename", "path", path, "error", err)
		return trackTags{Title: titleFromFilename(path)}, nil
	}

	tags := trackTags{
		Title:       strings.TrimSpace(meta.Title()),
		Artist:      strings.TrimSpace(meta.Artist()),
		Album:       strings.TrimSpace(meta.Album()),
		AlbumArtist: strings.TrimSpace(meta.AlbumArtist()),
	}

	if tags.Title == "" {
		tags.Title = titleFromFilename(path)
	}
	// Browsing groups by album_artist, so a file that only carries "artist"
	// must still land under a sensible heading.
	if tags.AlbumArtist == "" {
		tags.AlbumArtist = tags.Artist
	}
	if tags.AlbumArtist == "" {
		tags.AlbumArtist = "Unknown Artist"
	}
	if tags.Album == "" {
		tags.Album = "Unknown Album"
	}

	if trackNo, _ := meta.Track(); trackNo > 0 {
		tags.TrackNo = &trackNo
	}
	if discNo, _ := meta.Disc(); discNo > 0 {
		tags.DiscNo = &discNo
	}
	if year := meta.Year(); year > 0 {
		tags.Year = &year
	}
	if genre := strings.TrimSpace(meta.Genre()); genre != "" {
		tags.Genre = &genre
	}
	if mbid := musicBrainzID(meta); mbid != "" {
		tags.MusicBrainzID = &mbid
	}

	if picture := meta.Picture(); picture != nil && len(picture.Data) > 0 {
		hash, err := s.covers.Put(picture.Data, picture.MIMEType)
		if err != nil {
			s.log.Warn("could not store cover art", "path", path, "error", err)
		} else {
			tags.CoverHash = &hash
		}
	}

	return tags, nil
}

// musicBrainzID digs the recording id out of the format-specific raw tags.
func musicBrainzID(meta tag.Metadata) string {
	raw := meta.Raw()
	for _, key := range []string{
		"musicbrainz_trackid",
		"MusicBrainz Track Id",
		"MUSICBRAINZ_TRACKID",
		"----:com.apple.iTunes:MusicBrainz Track Id",
	} {
		if value, ok := raw[key]; ok {
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func titleFromFilename(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func (t trackTags) toTrack(path string) store.Track {
	return store.Track{
		Path:          path,
		Title:         t.Title,
		Artist:        t.Artist,
		Album:         t.Album,
		AlbumArtist:   t.AlbumArtist,
		TrackNo:       t.TrackNo,
		DiscNo:        t.DiscNo,
		Year:          t.Year,
		Genre:         t.Genre,
		MusicBrainzID: t.MusicBrainzID,
		CoverHash:     t.CoverHash,
	}
}
