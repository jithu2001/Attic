package jobs

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/perleybrook/attic/server/internal/media"
	"github.com/perleybrook/attic/server/internal/scanner"
)

// Watcher enqueues directory scans when the music library changes on disk.
//
// Events are debounced per directory: copying an album in drops dozens of
// events over several seconds, and scanning a half-written FLAC reads a
// truncated file. Waiting for the writes to settle costs a little latency and
// buys a correct result.
type Watcher struct {
	root     string
	debounce time.Duration
	enqueue  func(ctx context.Context, dir string) error
	log      *slog.Logger

	mu      sync.Mutex
	pending map[string]*time.Timer
}

// NewWatcher builds a watcher over root.
func NewWatcher(root string, debounce time.Duration, enqueue func(context.Context, string) error, log *slog.Logger) *Watcher {
	return &Watcher{
		root:     root,
		debounce: debounce,
		enqueue:  enqueue,
		log:      log,
		pending:  make(map[string]*time.Timer),
	}
}

// Run watches until ctx is cancelled. A watcher that cannot start is reported
// and then given up on: the hourly scan still catches everything, so a missing
// inotify quota degrades latency rather than breaking the feature.
func (w *Watcher) Run(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	if err := w.addTree(watcher, w.root); err != nil {
		return err
	}
	w.log.Info("watching music library", "root", w.root, "debounce", w.debounce)

	for {
		select {
		case <-ctx.Done():
			w.cancelPending()
			return ctx.Err()

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			w.handle(ctx, watcher, event)

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			w.log.Warn("filesystem watch error", "error", err)
		}
	}
}

func (w *Watcher) handle(ctx context.Context, watcher *fsnotify.Watcher, event fsnotify.Event) {
	// A new directory needs its own watch, and its contents need scanning.
	if event.Has(fsnotify.Create) {
		if stat, err := os.Stat(event.Name); err == nil && stat.IsDir() {
			w.adopt(ctx, watcher, event.Name)
			return
		}
	}

	// Chmod fires on every read on some filesystems; it never means new audio.
	if event.Op == fsnotify.Chmod {
		return
	}
	if !media.IsAudio(event.Name) {
		return
	}
	w.schedule(ctx, filepath.Dir(event.Name))
}

// adopt starts watching a directory that has just appeared, and schedules a
// scan of everything already inside it.
//
// Both halves are needed, in this order. Copying an album in creates the
// directory tree and its files faster than watches can be registered, so
// waiting for file events would miss everything written before the watch
// existed; walking afterwards catches those, and the watches catch whatever
// arrives later. Doing it the other way round would leave a gap between the
// walk and the watch.
func (w *Watcher) adopt(ctx context.Context, watcher *fsnotify.Watcher, dir string) {
	if err := w.addTree(watcher, dir); err != nil {
		w.log.Warn("could not watch new directory", "dir", dir, "error", err)
	}

	dirs, err := scanner.Directories(dir)
	if err != nil {
		w.log.Warn("could not enumerate new directory", "dir", dir, "error", err)
		w.schedule(ctx, dir)
		return
	}
	if len(dirs) == 0 {
		// Nothing in it yet; whatever lands later arrives as an event.
		return
	}
	for _, found := range dirs {
		w.schedule(ctx, found)
	}
}

// schedule (re)starts the debounce timer for dir.
func (w *Watcher) schedule(ctx context.Context, dir string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if timer, ok := w.pending[dir]; ok {
		timer.Reset(w.debounce)
		return
	}

	w.pending[dir] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		delete(w.pending, dir)
		w.mu.Unlock()

		if ctx.Err() != nil {
			return
		}
		if err := w.enqueue(ctx, dir); err != nil {
			w.log.Warn("could not enqueue scan for changed directory", "dir", dir, "error", err)
			return
		}
		w.log.Debug("enqueued scan for changed directory", "dir", dir)
	})
}

func (w *Watcher) cancelPending() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for dir, timer := range w.pending {
		timer.Stop()
		delete(w.pending, dir)
	}
}

// addTree watches root and every directory below it. fsnotify is not
// recursive, so subdirectories must be registered one by one.
func (w *Watcher) addTree(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable subtree: skip, the hourly scan still sees it
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
		}
		if err := watcher.Add(path); err != nil {
			w.log.Warn("could not watch directory", "dir", path, "error", err)
		}
		return nil
	})
}
