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
	if len(cfg.LibraryRoots) != 1 || cfg.LibraryRoots[0] != "/data/originals" {
		t.Errorf("LibraryRoots = %v", cfg.LibraryRoots)
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
