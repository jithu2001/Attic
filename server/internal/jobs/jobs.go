// Package jobs runs Attic's background work on River, which keeps its queue in
// the same Postgres database as everything else — no extra broker to operate.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/riverqueue/river/rivertype"

	"github.com/perleybrook/attic/server/internal/scanner"
)

// ScanLibraryArgs asks for a full pass over the music library: enumerate its
// directories, enqueue a job for each, then prune rows for vanished files.
type ScanLibraryArgs struct {
	// Reason is why the scan was triggered ("startup", "schedule", "manual").
	// It is excluded from the uniqueness hash so a manual scan does not queue
	// behind the hourly one.
	Reason string `json:"reason"`
}

// Kind implements river.JobArgs.
func (ScanLibraryArgs) Kind() string { return "music_scan_library" }

// InsertOpts keeps at most one library scan pending at a time.
func (ScanLibraryArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: 3,
		UniqueOpts: river.UniqueOpts{
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}

// ScanDirArgs scans exactly one directory. One job per directory is what makes
// a large library resumable: a restart re-runs at most one directory.
type ScanDirArgs struct {
	Dir string `json:"dir" river:"unique"`
}

// Kind implements river.JobArgs.
func (ScanDirArgs) Kind() string { return "music_scan_dir" }

// InsertOpts deduplicates queued scans of the same directory, which is what
// stops a burst of filesystem events becoming a burst of identical work.
func (ScanDirArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: 3,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		},
	}
}

// Runner owns the River client and the workers registered on it.
type Runner struct {
	client   *river.Client[pgx.Tx]
	log      *slog.Logger
	musicDir string
}

// Config configures the job runner.
type Config struct {
	Pool         *pgxpool.Pool
	Scanner      *scanner.Scanner
	MusicDir     string
	ScanInterval time.Duration
	Log          *slog.Logger
}

// New migrates River's own tables and builds a client with Attic's workers.
func New(ctx context.Context, cfg Config) (*Runner, error) {
	driver := riverpgxv5.New(cfg.Pool)

	migrator, err := rivermigrate.New(driver, nil)
	if err != nil {
		return nil, fmt.Errorf("jobs: build migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return nil, fmt.Errorf("jobs: migrate river schema: %w", err)
	}

	runner := &Runner{log: cfg.Log, musicDir: cfg.MusicDir}

	workers := river.NewWorkers()
	river.AddWorker(workers, &scanLibraryWorker{runner: runner, musicDir: cfg.MusicDir, scanner: cfg.Scanner, log: cfg.Log})
	river.AddWorker(workers, &scanDirWorker{scanner: cfg.Scanner, log: cfg.Log})

	client, err := river.NewClient(driver, &river.Config{
		Logger:  cfg.Log,
		Workers: workers,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 4},
		},
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(
				river.PeriodicInterval(cfg.ScanInterval),
				func() (river.JobArgs, *river.InsertOpts) {
					return ScanLibraryArgs{Reason: "schedule"}, nil
				},
				// Also scan once at startup, so a server that was down while
				// files were added catches up without anyone asking.
				&river.PeriodicJobOpts{RunOnStart: true},
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("jobs: build client: %w", err)
	}

	runner.client = client
	return runner, nil
}

// Start begins working the queue.
func (r *Runner) Start(ctx context.Context) error {
	if err := r.client.Start(ctx); err != nil {
		return fmt.Errorf("jobs: start: %w", err)
	}
	return nil
}

// Stop drains in-flight jobs and shuts the client down.
func (r *Runner) Stop(ctx context.Context) error { return r.client.Stop(ctx) }

// ScanLibrary enqueues a full library scan. Safe to call repeatedly: River
// deduplicates against any scan already queued or running.
func (r *Runner) ScanLibrary(ctx context.Context, reason string) error {
	_, err := r.client.Insert(ctx, ScanLibraryArgs{Reason: reason}, nil)
	if err != nil {
		return fmt.Errorf("jobs: enqueue library scan: %w", err)
	}
	return nil
}

// ScanDir enqueues a scan of one directory, used by the filesystem watcher.
func (r *Runner) ScanDir(ctx context.Context, dir string) error {
	_, err := r.client.Insert(ctx, ScanDirArgs{Dir: dir}, nil)
	if err != nil {
		return fmt.Errorf("jobs: enqueue directory scan: %w", err)
	}
	return nil
}
