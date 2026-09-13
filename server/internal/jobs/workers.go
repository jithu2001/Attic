package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"

	"github.com/perleybrook/attic/server/internal/scanner"
)

// scanLibraryWorker fans a library scan out into per-directory jobs, then
// prunes rows whose files are gone.
type scanLibraryWorker struct {
	river.WorkerDefaults[ScanLibraryArgs]

	runner   *Runner
	musicDir string
	scanner  *scanner.Scanner
	log      *slog.Logger
}

func (w *scanLibraryWorker) Work(ctx context.Context, job *river.Job[ScanLibraryArgs]) error {
	started := time.Now()

	dirs, err := scanner.Directories(w.musicDir)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		if err := w.runner.ScanDir(ctx, dir); err != nil {
			return err
		}
	}

	pruned, err := w.scanner.Prune(ctx, w.musicDir)
	if err != nil {
		return err
	}

	w.log.Info("library scan fanned out",
		"reason", job.Args.Reason,
		"directories", len(dirs),
		"pruned", pruned,
		"duration_ms", time.Since(started).Milliseconds(),
	)
	return nil
}

// Timeout bounds the enumeration pass. Walking a large library plus one stat
// per known file is minutes at worst, never hours.
func (w *scanLibraryWorker) Timeout(*river.Job[ScanLibraryArgs]) time.Duration {
	return 30 * time.Minute
}

// scanDirWorker ingests one directory.
type scanDirWorker struct {
	river.WorkerDefaults[ScanDirArgs]

	scanner *scanner.Scanner
	log     *slog.Logger
}

func (w *scanDirWorker) Work(ctx context.Context, job *river.Job[ScanDirArgs]) error {
	result, err := w.scanner.ScanDir(ctx, job.Args.Dir)
	if err != nil {
		return err
	}
	if result.Added > 0 || result.Removed > 0 || result.Failed > 0 {
		w.log.Info("scanned directory",
			"dir", result.Dir,
			"added", result.Added,
			"unchanged", result.Unchanged,
			"removed", result.Removed,
			"failed", result.Failed,
		)
	}
	return nil
}

func (w *scanDirWorker) Timeout(*river.Job[ScanDirArgs]) time.Duration {
	return 15 * time.Minute
}
