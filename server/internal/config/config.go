// Package config loads Attic server configuration from the environment.
//
// Every setting has a sane default so the binary starts with zero config in
// development; production deployments override via environment variables
// (docker-compose passes them through).
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/perleybrook/attic/server/internal/auth"
)

// Config is the fully resolved server configuration.
type Config struct {
	// HTTP
	ListenAddr string

	// Postgres connection string (pgx compatible).
	DatabaseURL string

	// Storage roots. Originals are immutable and content-addressed;
	// derived holds regenerable caches (thumbnails, transcodes).
	DataDir      string
	OriginalsDir string
	DerivedDir   string

	// LibraryRoots are the directories the scanner is allowed to read from.
	// Media handlers resolve every path against these roots, so nothing
	// outside them can ever be served.
	LibraryRoots []string

	// MusicDir is the library root the music scanner walks. It is always
	// included in LibraryRoots.
	MusicDir string

	// CoversDir holds cover art extracted from audio tags, named by content
	// hash. Regenerable: it lives under DerivedDir.
	CoversDir string

	// ScanInterval is how often the periodic music scan runs.
	ScanInterval time.Duration

	// ScanDebounce is how long the filesystem watcher waits for writes to
	// settle before enqueuing a scan of a changed directory.
	ScanDebounce time.Duration

	// Auth
	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	// MediaTokenTTL bounds the signed `?token=` URLs handed to the audio and
	// video players, which cannot refresh a bearer header mid-stream.
	MediaTokenTTL time.Duration

	// External tools
	FFmpegPath  string
	FFprobePath string

	// Observability
	LogLevel  slog.Level
	LogFormat string // "text" or "json"
}

// Load reads configuration from the environment and validates it.
func Load() (*Config, error) {
	dataDir := env("ATTIC_DATA_DIR", "/data")
	derivedDir := env("ATTIC_DERIVED_DIR", dataDir+"/derived")
	musicDir := env("ATTIC_MUSIC_DIR", dataDir+"/music")

	c := &Config{
		ListenAddr:      env("ATTIC_LISTEN_ADDR", ":8080"),
		DatabaseURL:     env("ATTIC_DATABASE_URL", "postgres://attic:attic@localhost:5432/attic?sslmode=disable"),
		DataDir:         dataDir,
		OriginalsDir:    env("ATTIC_ORIGINALS_DIR", dataDir+"/originals"),
		DerivedDir:      derivedDir,
		MusicDir:        musicDir,
		CoversDir:       env("ATTIC_COVERS_DIR", derivedDir+"/covers"),
		ScanInterval:    envDuration("ATTIC_SCAN_INTERVAL", time.Hour),
		ScanDebounce:    envDuration("ATTIC_SCAN_DEBOUNCE", 30*time.Second),
		LibraryRoots:    envList("ATTIC_LIBRARY_ROOTS", []string{dataDir + "/originals", musicDir}),
		JWTSecret:       []byte(env("ATTIC_JWT_SECRET", "")),
		AccessTokenTTL:  envDuration("ATTIC_ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL: envDuration("ATTIC_REFRESH_TOKEN_TTL", 90*24*time.Hour),
		MediaTokenTTL:   envDuration("ATTIC_MEDIA_TOKEN_TTL", 6*time.Hour),
		FFmpegPath:      env("ATTIC_FFMPEG_PATH", "ffmpeg"),
		FFprobePath:     env("ATTIC_FFPROBE_PATH", "ffprobe"),
		LogLevel:        envLevel("ATTIC_LOG_LEVEL", slog.LevelInfo),
		LogFormat:       env("ATTIC_LOG_FORMAT", "text"),
	}

	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	if c.ListenAddr == "" {
		return fmt.Errorf("config: ATTIC_LISTEN_ADDR must not be empty")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("config: ATTIC_DATABASE_URL must not be empty")
	}
	if len(c.JWTSecret) == 0 {
		return fmt.Errorf("config: ATTIC_JWT_SECRET must be set")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("config: ATTIC_JWT_SECRET must be at least 32 bytes")
	}
	if len(c.LibraryRoots) == 0 {
		return fmt.Errorf("config: ATTIC_LIBRARY_ROOTS must list at least one directory")
	}
	if c.AccessTokenTTL <= 0 || c.RefreshTokenTTL <= 0 || c.MediaTokenTTL <= 0 {
		return fmt.Errorf("config: token TTLs must be positive")
	}
	if c.MusicDir == "" {
		return fmt.Errorf("config: ATTIC_MUSIC_DIR must not be empty")
	}
	// The music scanner reads through the same guard as every media handler,
	// so its directory has to be a library root.
	if !c.isLibraryRoot(c.MusicDir) {
		c.LibraryRoots = append(c.LibraryRoots, c.MusicDir)
	}
	return nil
}

func (c *Config) isLibraryRoot(dir string) bool {
	for _, root := range c.LibraryRoots {
		if root == dir {
			return true
		}
	}
	return false
}

// NewTokens builds the token issuer this configuration describes.
func (c *Config) NewTokens() *auth.Tokens {
	return auth.NewTokens(c.JWTSecret, c.AccessTokenTTL, c.RefreshTokenTTL, c.MediaTokenTTL)
}

// Logger builds the slog logger described by the configuration.
func (c *Config) Logger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: c.LogLevel}
	if strings.EqualFold(c.LogFormat, "json") {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envList(key string, def []string) []string {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return def
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func envDuration(key string, def time.Duration) time.Duration {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func envLevel(key string, def slog.Level) slog.Level {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return def
	}
	var l slog.Level
	if err := l.UnmarshalText([]byte(raw)); err != nil {
		return def
	}
	return l
}
