package scanner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/perleybrook/attic/server/internal/ffmpeg"
	"github.com/perleybrook/attic/server/internal/media"
	"github.com/perleybrook/attic/server/internal/store"
)

// Store is the slice of the database the scanner needs.
type Store interface {
	FileStampsInDir(ctx context.Context, dir string) (map[string]store.FileStamp, error)
	KnownFilesUnder(ctx context.Context, dir string) ([]store.KnownFile, error)
	UpsertMediaFile(ctx context.Context, f store.MediaFile) (uuid.UUID, error)
	UpsertTrack(ctx context.Context, t store.Track) error
	TouchMediaFiles(ctx context.Context, ids []uuid.UUID) error
	DeleteMediaFiles(ctx context.Context, ids []uuid.UUID) (int64, error)
}

// Prober reads technical metadata off a file. It may be nil: without ffprobe
// Attic still builds a browsable library, it just has no durations.
type Prober interface {
	ProbeFile(ctx context.Context, path string) (*ffmpeg.Probe, error)
}

// Scanner ingests music files.
type Scanner struct {
	store  Store
	prober Prober
	covers *CoverStore
	log    *slog.Logger

	// ingestWorkers bounds how many files are hashed and probed at once.
	// Hashing is IO-bound and probing spawns a process; four keeps a spinning
	// disk busy without thrashing it or forking a hundred ffprobes.
	ingestWorkers int
}

// New builds a Scanner.
func New(s Store, prober Prober, covers *CoverStore, log *slog.Logger) *Scanner {
	return &Scanner{store: s, prober: prober, covers: covers, log: log, ingestWorkers: 4}
}

// Result summarises one directory scan.
type Result struct {
	Dir       string
	Added     int
	Unchanged int
	Removed   int
	Failed    int
}

// Directories lists every directory at or below root that directly contains at
// least one audio file. Each becomes its own scan job, which is what makes a
// large library resumable: a crash loses at most one directory's progress.
func Directories(root string) ([]string, error) {
	withAudio := make(map[string]struct{})

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable subtree should not abort the whole walk.
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			// Skip the dot-directories music managers litter libraries with.
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if media.IsAudio(path) {
			withAudio[filepath.Dir(path)] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanner: walk %s: %w", root, err)
	}

	dirs := make([]string, 0, len(withAudio))
	for dir := range withAudio {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs, nil
}

// ScanDir brings one directory's rows in line with what is on disk.
func (s *Scanner) ScanDir(ctx context.Context, dir string) (Result, error) {
	result := Result{Dir: dir}

	found, err := listAudio(dir)
	if err != nil {
		return result, err
	}

	known, err := s.store.FileStampsInDir(ctx, dir)
	if err != nil {
		return result, fmt.Errorf("scanner: read known files: %w", err)
	}

	plan := Diff(known, found)
	if plan.Empty() {
		return result, nil
	}

	if err := s.store.TouchMediaFiles(ctx, plan.Unchanged); err != nil {
		return result, fmt.Errorf("scanner: touch files: %w", err)
	}
	result.Unchanged = len(plan.Unchanged)

	removed, err := s.store.DeleteMediaFiles(ctx, plan.Remove)
	if err != nil {
		return result, fmt.Errorf("scanner: delete files: %w", err)
	}
	result.Removed = int(removed)

	var (
		mu     sync.Mutex
		failed int
		added  int
	)

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(s.ingestWorkers)
	for _, file := range plan.Ingest {
		group.Go(func() error {
			if err := s.ingest(groupCtx, file); err != nil {
				// One unreadable file must not fail the directory: log it and
				// carry on, or a single corrupt MP3 blocks the whole library.
				if groupCtx.Err() != nil {
					return groupCtx.Err()
				}
				s.log.Warn("skipping unreadable music file", "path", file.Path, "error", err)
				mu.Lock()
				failed++
				mu.Unlock()
				return nil
			}
			mu.Lock()
			added++
			mu.Unlock()
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return result, err
	}

	result.Added = added
	result.Failed = failed
	return result, nil
}

// Prune deletes rows for files that have disappeared from under root,
// including whole directories that were removed between scans.
func (s *Scanner) Prune(ctx context.Context, root string) (int, error) {
	known, err := s.store.KnownFilesUnder(ctx, root)
	if err != nil {
		return 0, fmt.Errorf("scanner: list known files: %w", err)
	}

	missing := make([]uuid.UUID, 0, 16)
	for _, file := range known {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		if _, err := os.Stat(file.Path); err != nil && os.IsNotExist(err) {
			missing = append(missing, file.ID)
		}
	}

	deleted, err := s.store.DeleteMediaFiles(ctx, missing)
	if err != nil {
		return 0, fmt.Errorf("scanner: prune: %w", err)
	}
	return int(deleted), nil
}

// ingest hashes, probes and tags one file, then writes it to the database.
func (s *Scanner) ingest(ctx context.Context, file OnDisk) error {
	sum, err := hashFile(file.Path)
	if err != nil {
		return err
	}

	// A probe failure when ffprobe *is* available means the file is not
	// decodable, which is worth skipping over. No ffprobe at all just means no
	// durations.
	var probe *ffmpeg.Probe
	if s.prober != nil {
		probe, err = s.prober.ProbeFile(ctx, file.Path)
		if err != nil {
			return err
		}
	}

	tags, err := s.readTags(file.Path)
	if err != nil {
		return err
	}

	mediaFile := store.MediaFile{
		Path:      file.Path,
		SizeBytes: file.SizeBytes,
		SHA256:    sum,
		MTime:     file.MTime,
	}
	if probe != nil {
		mediaFile.Probe = probe.Raw
	}

	fileID, err := s.store.UpsertMediaFile(ctx, mediaFile)
	if err != nil {
		return fmt.Errorf("scanner: upsert media file: %w", err)
	}

	track := tags.toTrack(file.Path)
	track.FileID = fileID
	if probe != nil && probe.DurationS > 0 {
		duration := probe.DurationS
		track.DurationS = &duration
	}

	if err := s.store.UpsertTrack(ctx, track); err != nil {
		return fmt.Errorf("scanner: upsert track: %w", err)
	}
	return nil
}

// hashFile streams the file through SHA-256 rather than reading it into
// memory: music files run to hundreds of megabytes and scans run four wide.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("scanner: open %s: %w", path, err)
	}
	defer f.Close()

	digest := sha256.New()
	if _, err := io.Copy(digest, f); err != nil {
		return "", fmt.Errorf("scanner: hash %s: %w", path, err)
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// listAudio returns the audio files directly inside dir.
func listAudio(dir string) ([]OnDisk, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("scanner: read dir %s: %w", dir, err)
	}

	files := make([]OnDisk, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !media.IsAudio(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue // vanished between readdir and stat
		}
		files = append(files, OnDisk{
			Path:      filepath.Join(dir, entry.Name()),
			SizeBytes: info.Size(),
			MTime:     info.ModTime(),
		})
	}
	return files, nil
}
