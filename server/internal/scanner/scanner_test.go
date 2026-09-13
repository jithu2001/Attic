package scanner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/perleybrook/attic/server/internal/ffmpeg"
	"github.com/perleybrook/attic/server/internal/store"
)

// memStore is an in-memory Store for the ingest tests.
type memStore struct {
	mu      sync.Mutex
	files   map[string]store.FileStamp
	tracks  map[uuid.UUID]store.Track
	touched []uuid.UUID
	deleted []uuid.UUID
}

func newMemStore() *memStore {
	return &memStore{
		files:  make(map[string]store.FileStamp),
		tracks: make(map[uuid.UUID]store.Track),
	}
}

func (m *memStore) FileStampsInDir(_ context.Context, dir string) (map[string]store.FileStamp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]store.FileStamp)
	for path, stamp := range m.files {
		if filepath.Dir(path) == filepath.Clean(dir) {
			out[path] = stamp
		}
	}
	return out, nil
}

func (m *memStore) KnownFilesUnder(_ context.Context, dir string) ([]store.KnownFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]store.KnownFile, 0, len(m.files))
	for path, stamp := range m.files {
		if rel, err := filepath.Rel(dir, path); err == nil && !filepath.IsAbs(rel) && rel != ".." {
			out = append(out, store.KnownFile{ID: stamp.ID, Path: path})
		}
	}
	return out, nil
}

func (m *memStore) UpsertMediaFile(_ context.Context, f store.MediaFile) (uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stamp, ok := m.files[f.Path]
	if !ok {
		stamp = store.FileStamp{ID: uuid.New()}
	}
	stamp.SizeBytes = f.SizeBytes
	stamp.MTime = f.MTime
	m.files[f.Path] = stamp
	return stamp.ID, nil
}

func (m *memStore) UpsertTrack(_ context.Context, t store.Track) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tracks[t.FileID] = t
	return nil
}

func (m *memStore) TouchMediaFiles(_ context.Context, ids []uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.touched = append(m.touched, ids...)
	return nil
}

func (m *memStore) DeleteMediaFiles(_ context.Context, ids []uuid.UUID) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleted = append(m.deleted, ids...)
	for _, id := range ids {
		for path, stamp := range m.files {
			if stamp.ID == id {
				delete(m.files, path)
			}
		}
		delete(m.tracks, id)
	}
	return int64(len(ids)), nil
}

func (m *memStore) trackByTitle(title string) (store.Track, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tracks {
		if t.Title == title {
			return t, true
		}
	}
	return store.Track{}, false
}

func (m *memStore) trackCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.tracks)
}

// stubProber stands in for ffprobe.
type stubProber struct {
	duration float64
	err      error
}

func (s stubProber) ProbeFile(context.Context, string) (*ffmpeg.Probe, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &ffmpeg.Probe{DurationS: s.duration, Raw: []byte(`{"format":{"duration":"1"}}`), Codec: "mp3"}, nil
}

func newTestScanner(t *testing.T, prober Prober) (*Scanner, *memStore) {
	t.Helper()
	covers, err := NewCoverStore(filepath.Join(t.TempDir(), "covers"))
	if err != nil {
		t.Fatalf("NewCoverStore: %v", err)
	}
	db := newMemStore()
	return New(db, prober, covers, slog.New(slog.NewTextHandler(io.Discard, nil))), db
}

func TestScanDirReadsTagsAndCoverArt(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Miles Davis", "Kind of Blue")

	image := []byte("\xff\xd8\xff\xe0 pretend jpeg bytes")
	writeMP3(t, filepath.Join(dir, "01 So What.mp3"),
		textFrame("TIT2", "So What"),
		textFrame("TPE1", "Miles Davis Quintet"),
		textFrame("TPE2", "Miles Davis"),
		textFrame("TALB", "Kind of Blue"),
		textFrame("TRCK", "1/5"),
		textFrame("TPOS", "1/1"),
		textFrame("TDRC", "1959"),
		textFrame("TCON", "Jazz"),
		pictureFrame("image/jpeg", image),
	)

	scanner, db := newTestScanner(t, stubProber{duration: 545.5})

	result, err := scanner.ScanDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("ScanDir: %v", err)
	}
	if result.Added != 1 || result.Failed != 0 {
		t.Fatalf("result = %+v, want one added", result)
	}

	track, ok := db.trackByTitle("So What")
	if !ok {
		t.Fatal("the track was not stored")
	}
	if track.Artist != "Miles Davis Quintet" {
		t.Errorf("artist = %q", track.Artist)
	}
	// Browsing groups by album artist, which is the whole point of reading
	// TPE2 separately from TPE1.
	if track.AlbumArtist != "Miles Davis" {
		t.Errorf("album_artist = %q, want the TPE2 value", track.AlbumArtist)
	}
	if track.Album != "Kind of Blue" {
		t.Errorf("album = %q", track.Album)
	}
	if track.TrackNo == nil || *track.TrackNo != 1 {
		t.Errorf("track_no = %v", track.TrackNo)
	}
	if track.DiscNo == nil || *track.DiscNo != 1 {
		t.Errorf("disc_no = %v", track.DiscNo)
	}
	if track.Year == nil || *track.Year != 1959 {
		t.Errorf("year = %v", track.Year)
	}
	if track.Genre == nil || *track.Genre != "Jazz" {
		t.Errorf("genre = %v", track.Genre)
	}
	if track.DurationS == nil || *track.DurationS != 545.5 {
		t.Errorf("duration = %v, want the probed value", track.DurationS)
	}
	if track.CoverHash == nil {
		t.Fatal("no cover art was extracted")
	}
	if _, err := scanner.covers.Find(*track.CoverHash); err != nil {
		t.Errorf("the cover hash does not resolve to a file: %v", err)
	}
}

func TestScanDirFallsBackForUntaggedFiles(t *testing.T) {
	dir := t.TempDir()
	// A bare .wav carries no tags dhowden/tag can read.
	if err := os.WriteFile(filepath.Join(dir, "07 Mystery Recording.wav"), []byte("RIFFxxxxWAVE"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	scanner, db := newTestScanner(t, stubProber{duration: 12})
	if _, err := scanner.ScanDir(context.Background(), dir); err != nil {
		t.Fatalf("ScanDir: %v", err)
	}

	// An untagged file still belongs in the library; dropping it would lose
	// music silently.
	track, ok := db.trackByTitle("07 Mystery Recording")
	if !ok {
		t.Fatal("an untagged file was dropped")
	}
	if track.AlbumArtist != "" {
		t.Errorf("album_artist = %q, want it left empty for the fallback path", track.AlbumArtist)
	}
}

func TestScanDirIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeMP3(t, filepath.Join(dir, "a.mp3"), textFrame("TIT2", "A"), textFrame("TPE2", "X"), textFrame("TALB", "Y"))

	scanner, db := newTestScanner(t, stubProber{duration: 1})

	first, err := scanner.ScanDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("ScanDir: %v", err)
	}
	second, err := scanner.ScanDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("ScanDir: %v", err)
	}

	if first.Added != 1 {
		t.Errorf("first scan added %d, want 1", first.Added)
	}
	// The second pass must not re-hash anything: that is what keeps the hourly
	// scan of a large library cheap.
	if second.Added != 0 || second.Unchanged != 1 {
		t.Errorf("second scan = %+v, want nothing added and one unchanged", second)
	}
	if db.trackCount() != 1 {
		t.Errorf("track count = %d, want 1", db.trackCount())
	}
}

func TestScanDirPicksUpNewAndDeletedFiles(t *testing.T) {
	dir := t.TempDir()
	first := writeMP3(t, filepath.Join(dir, "a.mp3"), textFrame("TIT2", "A"))

	scanner, db := newTestScanner(t, stubProber{duration: 1})
	if _, err := scanner.ScanDir(context.Background(), dir); err != nil {
		t.Fatalf("ScanDir: %v", err)
	}

	// Drop a new album in and take the old file away.
	writeMP3(t, filepath.Join(dir, "b.mp3"), textFrame("TIT2", "B"))
	if err := os.Remove(first); err != nil {
		t.Fatalf("remove: %v", err)
	}

	result, err := scanner.ScanDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("ScanDir: %v", err)
	}
	if result.Added != 1 {
		t.Errorf("added = %d, want 1", result.Added)
	}
	if result.Removed != 1 {
		t.Errorf("removed = %d, want 1", result.Removed)
	}
	if _, ok := db.trackByTitle("B"); !ok {
		t.Error("the new file was not ingested")
	}
}

func TestScanDirSkipsUnreadableFilesWithoutFailingTheDirectory(t *testing.T) {
	dir := t.TempDir()
	writeMP3(t, filepath.Join(dir, "good.mp3"), textFrame("TIT2", "Good"))
	writeMP3(t, filepath.Join(dir, "corrupt.mp3"), textFrame("TIT2", "Corrupt"))

	// A prober that rejects one specific file, the way ffprobe rejects a
	// truncated download.
	scanner, db := newTestScanner(t, proberFailingOn{"corrupt.mp3"})

	result, err := scanner.ScanDir(context.Background(), dir)
	if err != nil {
		t.Fatalf("ScanDir returned an error for one bad file: %v", err)
	}
	if result.Added != 1 || result.Failed != 1 {
		t.Errorf("result = %+v, want one added and one failed", result)
	}
	if _, ok := db.trackByTitle("Good"); !ok {
		t.Error("one corrupt file blocked its neighbours")
	}
}

type proberFailingOn struct{ name string }

func (p proberFailingOn) ProbeFile(_ context.Context, path string) (*ffmpeg.Probe, error) {
	if filepath.Base(path) == p.name {
		return nil, errors.New("ffprobe: invalid data found when processing input")
	}
	return &ffmpeg.Probe{DurationS: 1}, nil
}

func TestScanDirWorksWithoutAProber(t *testing.T) {
	dir := t.TempDir()
	writeMP3(t, filepath.Join(dir, "a.mp3"), textFrame("TIT2", "A"))

	// No FFmpeg on the box: the library still scans, it just has no durations.
	scanner, db := newTestScanner(t, nil)
	if _, err := scanner.ScanDir(context.Background(), dir); err != nil {
		t.Fatalf("ScanDir: %v", err)
	}

	track, ok := db.trackByTitle("A")
	if !ok {
		t.Fatal("the track was not stored")
	}
	if track.DurationS != nil {
		t.Errorf("duration = %v, want nil without a prober", track.DurationS)
	}
}

func TestDirectoriesFindsOnlyDirectoriesHoldingAudio(t *testing.T) {
	root := t.TempDir()
	writeMP3(t, filepath.Join(root, "Artist", "Album", "01.mp3"), textFrame("TIT2", "A"))
	writeMP3(t, filepath.Join(root, "Artist", "Album", "02.mp3"), textFrame("TIT2", "B"))
	writeMP3(t, filepath.Join(root, "Other", "01.mp3"), textFrame("TIT2", "C"))

	// Neither of these should produce a scan job.
	if err := os.MkdirAll(filepath.Join(root, "Artwork"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Artwork", "front.jpg"), []byte("jpeg"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeMP3(t, filepath.Join(root, ".trash", "deleted.mp3"), textFrame("TIT2", "D"))

	dirs, err := Directories(root)
	if err != nil {
		t.Fatalf("Directories: %v", err)
	}

	want := []string{filepath.Join(root, "Artist", "Album"), filepath.Join(root, "Other")}
	if len(dirs) != len(want) {
		t.Fatalf("dirs = %v, want %v", dirs, want)
	}
	for i := range want {
		if dirs[i] != want[i] {
			t.Fatalf("dirs = %v, want %v", dirs, want)
		}
	}
}

func TestPruneDropsVanishedDirectories(t *testing.T) {
	root := t.TempDir()
	album := filepath.Join(root, "Artist", "Album")
	writeMP3(t, filepath.Join(album, "01.mp3"), textFrame("TIT2", "A"))

	scanner, db := newTestScanner(t, stubProber{duration: 1})
	if _, err := scanner.ScanDir(context.Background(), album); err != nil {
		t.Fatalf("ScanDir: %v", err)
	}
	if db.trackCount() != 1 {
		t.Fatalf("track count = %d, want 1", db.trackCount())
	}

	// A whole directory disappearing produces no per-directory job, so the
	// prune pass is the only thing that notices.
	if err := os.RemoveAll(album); err != nil {
		t.Fatalf("remove: %v", err)
	}

	pruned, err := scanner.Prune(context.Background(), root)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if pruned != 1 {
		t.Errorf("pruned = %d, want 1", pruned)
	}
	if db.trackCount() != 0 {
		t.Errorf("track count = %d, want 0", db.trackCount())
	}
}
