// Command Attic is the single Attic server binary: API, scanner, background
// jobs and media streaming in one modular monolith.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/perleybrook/attic/server/internal/api"
	"github.com/perleybrook/attic/server/internal/config"
)

func main() {
	// -healthcheck lets the container image probe itself without shipping
	// curl in the runtime layer (see Dockerfile HEALTHCHECK).
	healthcheck := flag.Bool("healthcheck", false, "probe the local /healthz endpoint and exit")
	flag.Parse()

	if *healthcheck {
		if err := probeHealth(); err != nil {
			os.Stderr.WriteString("unhealthy: " + err.Error() + "\n")
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		// The logger may not exist yet if config failed, so use stderr.
		os.Stderr.WriteString("fatal: " + err.Error() + "\n")
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := cfg.Logger()
	log.Info("starting Attic",
		"version", api.Version,
		"listen_addr", cfg.ListenAddr,
		"data_dir", cfg.DataDir,
		"library_roots", cfg.LibraryRoots,
	)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.NewServer(cfg, log),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: media streaming responses are long-lived.
		IdleTimeout: 120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

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
	log.Info("stopped cleanly")
	return nil
}

// probeHealth performs a local GET /healthz, used by the container healthcheck.
func probeHealth() error {
	addr := os.Getenv("ATTIC_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET /healthz returned %s", resp.Status)
	}
	return nil
}
