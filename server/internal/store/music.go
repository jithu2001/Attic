package store

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MediaFile is one file on disk that Attic has ingested.
type MediaFile struct {
	ID         uuid.UUID
	Path       string
	SizeBytes  int64
	SHA256     string
	MTime      time.Time
	Probe      []byte
	VerifiedAt *time.Time
}

// FileStamp is the cheap half of a media file: enough to decide whether a file
// on disk has changed without reading its contents.
type FileStamp struct {
	ID        uuid.UUID
	SizeBytes int64
	MTime     time.Time
}

// Track is one audio track, joined to the file it came from.
type Track struct {
	ID            uuid.UUID
	FileID        uuid.UUID
	Path          string
	Title         string
	Artist        string
	Album         string
	AlbumArtist   string
	TrackNo       *int
	DiscNo        *int
	Year          *int
	Genre         *string
	DurationS     *float64
	MusicBrainzID *string
	CoverHash     *string
	SizeBytes     int64
}

// Artist is an album-artist grouping.
type Artist struct {
	Name       string
	AlbumCount int
	TrackCount int
	CoverHash  *string
}

// Album is an (album artist, album) grouping.
type Album struct {
	AlbumArtist string
	Name        string
	Year        *int
	TrackCount  int
	DurationS   float64
	CoverHash   *string
}

// ---------------------------------------------------------------- ingestion --

// FileStampsUnder returns the recorded size and mtime of every known file whose
// path sits under dir, keyed by absolute path. The scanner diffs this against
// the filesystem to decide what to (re)read.
func (s *Store) FileStampsUnder(ctx context.Context, dir string) (map[string]FileStamp, error) {
	prefix := strings.TrimSuffix(dir, "/") + "/"

	const query = `
		SELECT id, path, size_bytes, mtime
		FROM media_files
		WHERE path LIKE $1 ESCAPE '\'`

	return s.queryFileStamps(ctx, query, escapeLike(prefix)+"%")
}

func (s *Store) queryFileStamps(ctx context.Context, query string, args ...any) (map[string]FileStamp, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	stamps := make(map[string]FileStamp)
	for rows.Next() {
		var (
			path  string
			stamp FileStamp
		)
		if err := rows.Scan(&stamp.ID, &path, &stamp.SizeBytes, &stamp.MTime); err != nil {
			return nil, mapError(err)
		}
		stamps[path] = stamp
	}
	return stamps, mapError(rows.Err())
}

// escapeLike neutralises LIKE wildcards in a literal path prefix. Without it a
// directory named "100%" would match far more than itself.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// FileStampsInDir is FileStampsUnder restricted to dir's direct children, which
// is the unit one scan job works on.
func (s *Store) FileStampsInDir(ctx context.Context, dir string) (map[string]FileStamp, error) {
	prefix := escapeLike(strings.TrimSuffix(dir, "/") + "/")

	const query = `
		SELECT id, path, size_bytes, mtime
		FROM media_files
		WHERE path LIKE $1 || '%' ESCAPE '\'
		  AND path NOT LIKE $1 || '%/%' ESCAPE '\'`

	return s.queryFileStamps(ctx, query, prefix)
}

// KnownFile is a recorded path and its row id.
type KnownFile struct {
	ID   uuid.UUID
	Path string
}

// KnownFilesUnder lists every recorded file below dir, for the prune pass.
func (s *Store) KnownFilesUnder(ctx context.Context, dir string) ([]KnownFile, error) {
	prefix := escapeLike(strings.TrimSuffix(dir, "/") + "/")

	const query = `
		SELECT id, path FROM media_files
		WHERE path LIKE $1 || '%' ESCAPE '\'
		ORDER BY path`

	rows, err := s.pool.Query(ctx, query, prefix)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	files := make([]KnownFile, 0, 256)
	for rows.Next() {
		var f KnownFile
		if err := rows.Scan(&f.ID, &f.Path); err != nil {
			return nil, mapError(err)
		}
		files = append(files, f)
	}
	return files, mapError(rows.Err())
}

// UpsertMediaFile records a file by path, replacing the stamp and probe of an
// existing row. Returns the row id.
func (s *Store) UpsertMediaFile(ctx context.Context, f MediaFile) (uuid.UUID, error) {
	const query = `
		INSERT INTO media_files (path, size_bytes, sha256, mtime, probe, verified_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (path) DO UPDATE SET
			size_bytes  = EXCLUDED.size_bytes,
			sha256      = EXCLUDED.sha256,
			mtime       = EXCLUDED.mtime,
			probe       = EXCLUDED.probe,
			verified_at = now()
		RETURNING id`

	var id uuid.UUID
	err := s.pool.QueryRow(ctx, query, f.Path, f.SizeBytes, f.SHA256, f.MTime, f.Probe).Scan(&id)
	return id, mapError(err)
}

// TouchMediaFiles marks unchanged files as verified in this scan pass, so a
// later sweep can tell "still there" from "gone".
func (s *Store) TouchMediaFiles(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `UPDATE media_files SET verified_at = now() WHERE id = ANY($1)`, ids)
	return mapError(err)
}

// DeleteMediaFiles removes rows for files that no longer exist on disk.
// Tracks referencing them cascade away.
func (s *Store) DeleteMediaFiles(ctx context.Context, ids []uuid.UUID) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM media_files WHERE id = ANY($1)`, ids)
	if err != nil {
		return 0, mapError(err)
	}
	return tag.RowsAffected(), nil
}

// UpsertTrack writes the tags read from a file. One track per file, so the
// file id is the conflict target.
func (s *Store) UpsertTrack(ctx context.Context, t Track) error {
	const query = `
		INSERT INTO tracks (
			file_id, title, artist, album, album_artist,
			track_no, disc_no, year, genre, duration_s, musicbrainz_id, cover_hash
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (file_id) DO UPDATE SET
			title          = EXCLUDED.title,
			artist         = EXCLUDED.artist,
			album          = EXCLUDED.album,
			album_artist   = EXCLUDED.album_artist,
			track_no       = EXCLUDED.track_no,
			disc_no        = EXCLUDED.disc_no,
			year           = EXCLUDED.year,
			genre          = EXCLUDED.genre,
			duration_s     = EXCLUDED.duration_s,
			musicbrainz_id = EXCLUDED.musicbrainz_id,
			cover_hash     = EXCLUDED.cover_hash,
			updated_at     = now()`

	_, err := s.pool.Exec(ctx, query,
		t.FileID, t.Title, t.Artist, t.Album, t.AlbumArtist,
		t.TrackNo, t.DiscNo, t.Year, t.Genre, t.DurationS, t.MusicBrainzID, t.CoverHash,
	)
	return mapError(err)
}

// CountTracks reports the size of the music library.
func (s *Store) CountTracks(ctx context.Context) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM tracks`).Scan(&n); err != nil {
		return 0, mapError(err)
	}
	return n, nil
}

// ------------------------------------------------------------------ browse --

const trackColumns = `
	t.id, t.file_id, f.path, t.title, t.artist, t.album, t.album_artist,
	t.track_no, t.disc_no, t.year, t.genre, t.duration_s, t.musicbrainz_id,
	t.cover_hash, f.size_bytes`

func scanTracks(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
	Close()
},
) ([]Track, error) {
	defer rows.Close()

	tracks := make([]Track, 0, 32)
	for rows.Next() {
		var t Track
		if err := rows.Scan(
			&t.ID, &t.FileID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.AlbumArtist,
			&t.TrackNo, &t.DiscNo, &t.Year, &t.Genre, &t.DurationS, &t.MusicBrainzID,
			&t.CoverHash, &t.SizeBytes,
		); err != nil {
			return nil, mapError(err)
		}
		tracks = append(tracks, t)
	}
	return tracks, mapError(rows.Err())
}

// Artists lists album artists with their album and track counts.
func (s *Store) Artists(ctx context.Context) ([]Artist, error) {
	const query = `
		SELECT
			album_artist,
			count(DISTINCT album) AS album_count,
			count(*)              AS track_count,
			(array_agg(cover_hash) FILTER (WHERE cover_hash IS NOT NULL))[1] AS cover_hash
		FROM tracks
		WHERE album_artist <> ''
		GROUP BY album_artist
		ORDER BY lower(album_artist)`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	artists := make([]Artist, 0, 64)
	for rows.Next() {
		var a Artist
		if err := rows.Scan(&a.Name, &a.AlbumCount, &a.TrackCount, &a.CoverHash); err != nil {
			return nil, mapError(err)
		}
		artists = append(artists, a)
	}
	return artists, mapError(rows.Err())
}

// AlbumsByArtist lists one artist's albums, oldest first.
func (s *Store) AlbumsByArtist(ctx context.Context, albumArtist string) ([]Album, error) {
	const query = `
		SELECT
			album_artist,
			album,
			min(year)                  AS year,
			count(*)                   AS track_count,
			coalesce(sum(duration_s), 0) AS duration_s,
			(array_agg(cover_hash) FILTER (WHERE cover_hash IS NOT NULL))[1] AS cover_hash
		FROM tracks
		WHERE album_artist = $1
		GROUP BY album_artist, album
		ORDER BY min(year) NULLS LAST, lower(album)`

	return s.queryAlbums(ctx, query, albumArtist)
}

func (s *Store) queryAlbums(ctx context.Context, query string, args ...any) ([]Album, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	albums := make([]Album, 0, 16)
	for rows.Next() {
		var a Album
		if err := rows.Scan(&a.AlbumArtist, &a.Name, &a.Year, &a.TrackCount, &a.DurationS, &a.CoverHash); err != nil {
			return nil, mapError(err)
		}
		albums = append(albums, a)
	}
	return albums, mapError(rows.Err())
}

// Album returns one album's summary. ErrNotFound if it holds no tracks.
func (s *Store) Album(ctx context.Context, albumArtist, album string) (*Album, error) {
	const query = `
		SELECT
			album_artist,
			album,
			min(year),
			count(*),
			coalesce(sum(duration_s), 0),
			(array_agg(cover_hash) FILTER (WHERE cover_hash IS NOT NULL))[1]
		FROM tracks
		WHERE album_artist = $1 AND album = $2
		GROUP BY album_artist, album`

	albums, err := s.queryAlbums(ctx, query, albumArtist, album)
	if err != nil {
		return nil, err
	}
	if len(albums) == 0 {
		return nil, ErrNotFound
	}
	return &albums[0], nil
}

// AlbumTracks returns an album's tracks in disc then track order, which is the
// order they are meant to be played in.
func (s *Store) AlbumTracks(ctx context.Context, albumArtist, album string) ([]Track, error) {
	query := `
		SELECT ` + trackColumns + `
		FROM tracks t JOIN media_files f ON f.id = t.file_id
		WHERE t.album_artist = $1 AND t.album = $2
		ORDER BY t.disc_no NULLS FIRST, t.track_no NULLS FIRST, lower(t.title)`

	rows, err := s.pool.Query(ctx, query, albumArtist, album)
	if err != nil {
		return nil, mapError(err)
	}
	return scanTracks(rows)
}

// TrackByID returns one track, including the path needed to stream it.
func (s *Store) TrackByID(ctx context.Context, id uuid.UUID) (*Track, error) {
	query := `
		SELECT ` + trackColumns + `
		FROM tracks t JOIN media_files f ON f.id = t.file_id
		WHERE t.id = $1`

	rows, err := s.pool.Query(ctx, query, id)
	if err != nil {
		return nil, mapError(err)
	}
	tracks, err := scanTracks(rows)
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, ErrNotFound
	}
	return &tracks[0], nil
}

// TracksByIDs returns the given tracks in the order the ids were supplied,
// skipping any that no longer exist.
func (s *Store) TracksByIDs(ctx context.Context, ids []uuid.UUID) ([]Track, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := `
		SELECT ` + trackColumns + `
		FROM tracks t JOIN media_files f ON f.id = t.file_id
		WHERE t.id = ANY($1)`

	rows, err := s.pool.Query(ctx, query, ids)
	if err != nil {
		return nil, mapError(err)
	}
	found, err := scanTracks(rows)
	if err != nil {
		return nil, err
	}

	byID := make(map[uuid.UUID]Track, len(found))
	for _, t := range found {
		byID[t.ID] = t
	}
	ordered := make([]Track, 0, len(ids))
	for _, id := range ids {
		if t, ok := byID[id]; ok {
			ordered = append(ordered, t)
		}
	}
	return ordered, nil
}

// ------------------------------------------------------------------ search --

// SearchResult is one hit, which may be an artist, an album or a track.
type SearchResult struct {
	Kind        string // "artist", "album" or "track"
	Name        string
	Subtitle    string
	CoverHash   *string
	TrackID     *uuid.UUID
	AlbumArtist string
	Album       string
	DurationS   *float64
}

// Search runs a full-text query across tracks and rolls the matches up into
// artist, album and track hits, best first.
func (s *Store) Search(ctx context.Context, q string, limit int) ([]SearchResult, error) {
	const query = `
		WITH tsq AS (
			SELECT websearch_to_tsquery('simple', $1) AS q
		),
		matches AS (
			SELECT t.*, ts_rank(t.search_vector, tsq.q) AS rank
			FROM tracks t, tsq
			WHERE t.search_vector @@ tsq.q
		)
		SELECT kind, name, subtitle, cover_hash, track_id, album_artist, album, duration_s, rank
		FROM (
			SELECT
				'artist'::text AS kind,
				album_artist   AS name,
				''::text       AS subtitle,
				(array_agg(cover_hash) FILTER (WHERE cover_hash IS NOT NULL))[1] AS cover_hash,
				NULL::uuid     AS track_id,
				album_artist,
				''::text       AS album,
				NULL::double precision AS duration_s,
				-- An artist whose name matches many tracks is a better hit than
				-- one stray track, but not unboundedly so.
				max(rank) + least(count(*), 10) * 0.001 AS rank
			FROM matches
			WHERE album_artist <> ''
			GROUP BY album_artist

			UNION ALL

			SELECT
				'album', album, album_artist,
				(array_agg(cover_hash) FILTER (WHERE cover_hash IS NOT NULL))[1],
				NULL::uuid, album_artist, album, NULL::double precision,
				max(rank) + least(count(*), 10) * 0.001
			FROM matches
			WHERE album <> ''
			GROUP BY album_artist, album

			UNION ALL

			SELECT
				'track', title, artist, cover_hash, id, album_artist, album, duration_s, rank
			FROM matches
		) hits
		ORDER BY rank DESC, lower(name)
		LIMIT $2`

	rows, err := s.pool.Query(ctx, query, q, limit)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	results := make([]SearchResult, 0, limit)
	for rows.Next() {
		var (
			r    SearchResult
			rank float64
		)
		if err := rows.Scan(&r.Kind, &r.Name, &r.Subtitle, &r.CoverHash, &r.TrackID,
			&r.AlbumArtist, &r.Album, &r.DurationS, &rank); err != nil {
			return nil, mapError(err)
		}
		results = append(results, r)
	}
	return results, mapError(rows.Err())
}
