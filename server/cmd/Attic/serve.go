package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/perleybrook/attic/server/internal/api"
	"github.com/perleybrook/attic/server/internal/config"
	"github.com/perleybrook/attic/server/internal/ffmpeg"
	"github.com/perleybrook/attic/server/internal/jobs"
	"github.com/perleybrook/attic/server/internal/scanner"
	"github.com/perleybrook/attic/server/internal/store"
	"github.com/perleybrook/attic/server/internal/stream"
	"github.com/perleybrook/attic/server/migrations"
)

func runServer(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := cfg.Logger()
	slog.SetDefault(log)

	log.Info("starting Attic",
		"version", api.Version,
		"listen_addr", cfg.ListenAddr,
		"music_dir", cfg.MusicDir,
		"library_roots", cfg.LibraryRoots,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Migrations run before anything opens a pool, so every later query sees
	// the schema it expects.
	if err := waitForDatabase(ctx, cfg.DatabaseURL, log); err != nil {
		return err
	}
	if err := migrations.Up(cfg.DatabaseURL, log); err != nil {
		return err
	}

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	guard, err := stream.NewGuard(cfg.LibraryRoots)
	if err != nil {
		return err
	}

	covers, err := scanner.NewCoverStore(cfg.CoversDir)
	if err != nil {
		return err
	}

	// Attic still serves originals without FFmpeg; it just cannot read track
	// durations or transcode. Say so loudly rather than failing to boot.
	tools := ffmpeg.New(cfg.FFmpegPath, cfg.FFprobePath)
	var prober scanner.Prober = tools
	if err := tools.Available(); err != nil {
		log.Warn("FFmpeg tools are unavailable: track durations and transcoding are disabled", "error", err)
		prober = nil
		tools = nil
	}

	musicScanner := scanner.New(db, prober, covers, log)

	runner, err := jobs.New(ctx, jobs.Config{
		Pool:         db.Pool(),
		Scanner:      musicScanner,
		MusicDir:     cfg.MusicDir,
		ScanInterval: cfg.ScanInterval,
		Log:          log,
	})
	if err != nil {
		return err
	}
	if err := runner.Start(ctx); err != nil {
		return err
	}

	watcher := jobs.NewWatcher(cfg.MusicDir, cfg.ScanDebounce, runner.ScanDir, log)
	go func() {
		if err := watcher.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			// The hourly scan still picks everything up, so this degrades
			// latency rather than breaking the feature.
			log.Warn("filesystem watcher stopped; falling back to scheduled scans", "error", err)
		}
	}()

	handler := api.NewServer(api.Deps{
		Config: cfg,
		Log:    log,
		Store:  db,
		Tokens: cfg.NewTokens(),
		Guard:  guard,
		Covers: covers,
		FFmpeg: tools,
		Jobs:   runner,
	})

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: media streaming responses are long-lived.
		IdleTimeout: 120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	log.Info("listening", "addr", cfg.ListenAddr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received, draining connections")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := runner.Stop(shutdownCtx); err != nil {
		log.Warn("job runner did not stop cleanly", "error", err)
	}
	log.Info("stopped cleanly")
	return nil
}

// waitForDatabase retries until Postgres accepts a connection. Compose starts
// both containers at once, and a healthcheck is not always enough on a cold
// machine where Postgres is still initialising its data directory.
func waitForDatabase(ctx context.Context, databaseURL string, log *slog.Logger) error {
	const timeout = 60 * time.Second

	deadline := time.Now().Add(timeout)
	for attempt := 1; ; attempt++ {
		db, err := store.Open(ctx, databaseURL)
		if err == nil {
			db.Close()
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("database unreachable after %s: %w", timeout, err)
		}
		log.Info("waiting for database", "attempt", attempt, "error", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
