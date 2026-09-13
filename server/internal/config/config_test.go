package config

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "0123456789abcdef0123456789abcdef"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("ATTIC_JWT_SECRET", testSecret)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want :8080", cfg.ListenAddr)
	}
	if cfg.OriginalsDir != "/data/originals" {
		t.Errorf("OriginalsDir = %q", cfg.OriginalsDir)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("AccessTokenTTL = %v, want 15m", cfg.AccessTokenTTL)
	}
	if len(cfg.LibraryRoots) != 2 || cfg.LibraryRoots[0] != "/data/originals" || cfg.LibraryRoots[1] != "/data/music" {
		t.Errorf("LibraryRoots = %v, want [/data/originals /data/music]", cfg.LibraryRoots)
	}
	if cfg.MusicDir != "/data/music" {
		t.Errorf("MusicDir = %q", cfg.MusicDir)
	}
}

func TestLoadDataDirDerivesPaths(t *testing.T) {
	t.Setenv("ATTIC_JWT_SECRET", testSecret)
	t.Setenv("ATTIC_DATA_DIR", "/srv/attic")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DerivedDir != "/srv/attic/derived" {
		t.Errorf("DerivedDir = %q", cfg.DerivedDir)
	}
}

func TestLoadRequiresStrongSecret(t *testing.T) {
	tests := map[string]string{
		"empty": "",
		"short": "tooshort",
	}
	for name, secret := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("ATTIC_JWT_SECRET", secret)
			if _, err := Load(); err == nil {
				t.Fatal("Load() succeeded, want error")
			} else if !strings.Contains(err.Error(), "ATTIC_JWT_SECRET") {
				t.Errorf("error = %v, want it to mention the secret", err)
			}
		})
	}
}

func TestEnvListSplitsAndTrims(t *testing.T) {
	t.Setenv("ATTIC_JWT_SECRET", testSecret)
	t.Setenv("ATTIC_LIBRARY_ROOTS", " /mnt/photos , /mnt/video ,, ")
	t.Setenv("ATTIC_MUSIC_DIR", "/mnt/video")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := []string{"/mnt/photos", "/mnt/video"}
	if len(cfg.LibraryRoots) != len(want) {
		t.Fatalf("LibraryRoots = %v, want %v", cfg.LibraryRoots, want)
	}
	for i := range want {
		if cfg.LibraryRoots[i] != want[i] {
			t.Errorf("LibraryRoots[%d] = %q, want %q", i, cfg.LibraryRoots[i], want[i])
		}
	}
}

func TestEnvDurationFallsBackOnGarbage(t *testing.T) {
	t.Setenv("ATTIC_JWT_SECRET", testSecret)
	t.Setenv("ATTIC_ACCESS_TOKEN_TTL", "not-a-duration")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("AccessTokenTTL = %v, want the 15m default", cfg.AccessTokenTTL)
	}
}

func TestMusicDirIsAlwaysALibraryRoot(t *testing.T) {
	t.Setenv("ATTIC_JWT_SECRET", testSecret)
	t.Setenv("ATTIC_LIBRARY_ROOTS", "/mnt/photos")
	t.Setenv("ATTIC_MUSIC_DIR", "/mnt/music")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Media handlers refuse to serve anything outside a library root, so a
	// music directory that is not one would scan but never play.
	var found bool
	for _, root := range cfg.LibraryRoots {
		if root == "/mnt/music" {
			found = true
		}
	}
	if !found {
		t.Errorf("LibraryRoots = %v, want it to contain the music dir", cfg.LibraryRoots)
	}
}

func TestCoversDirFollowsDerivedDir(t *testing.T) {
	t.Setenv("ATTIC_JWT_SECRET", testSecret)
	t.Setenv("ATTIC_DERIVED_DIR", "/cache")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CoversDir != "/cache/covers" {
		t.Errorf("CoversDir = %q, want /cache/covers", cfg.CoversDir)
	}
}
