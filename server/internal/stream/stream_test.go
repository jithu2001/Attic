package stream

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGuardAllowsFilesInsideTheLibrary(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "artist", "album", "track.flac")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(inside, []byte("music"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	guard, err := NewGuard([]string{root})
	if err != nil {
		t.Fatalf("NewGuard() error = %v", err)
	}

	resolved, err := guard.Resolve(inside)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved == "" {
		t.Error("Resolve() returned an empty path")
	}

	file, err := guard.Open(inside)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()
	if file.Info.Size() != 5 {
		t.Errorf("size = %d, want 5", file.Info.Size())
	}
}

func TestGuardRefusesEverythingElse(t *testing.T) {
	root := t.TempDir()
	elsewhere := t.TempDir()

	secret := filepath.Join(elsewhere, "secret.txt")
	if err := os.WriteFile(secret, []byte("not yours"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// A symlink *inside* the library pointing out of it is the interesting
	// case: the path looks fine until it is resolved.
	link := filepath.Join(root, "escape.flac")
	if err := os.Symlink(secret, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks need privileges on Windows")
		}
		t.Fatalf("symlink: %v", err)
	}

	guard, err := NewGuard([]string{root})
	if err != nil {
		t.Fatalf("NewGuard() error = %v", err)
	}

	for _, name := range []string{
		secret,
		link,
		filepath.Join(root, "..", filepath.Base(elsewhere), "secret.txt"),
		filepath.Join(root, "../../etc/passwd"),
		"/etc/passwd",
		"",
	} {
		if _, err := guard.Resolve(name); !errors.Is(err, ErrOutsideLibrary) {
			t.Errorf("Resolve(%q) = %v, want ErrOutsideLibrary", name, err)
		}
	}
}

func TestGuardRefusesDirectories(t *testing.T) {
	root := t.TempDir()
	guard, err := NewGuard([]string{root})
	if err != nil {
		t.Fatalf("NewGuard() error = %v", err)
	}

	if _, err := guard.Open(root); err == nil {
		t.Error("Open() served a directory")
	}
}

func TestGuardAcceptsAnyConfiguredRoot(t *testing.T) {
	music := t.TempDir()
	photos := t.TempDir()

	track := filepath.Join(music, "a.flac")
	photo := filepath.Join(photos, "a.jpg")
	for _, path := range []string{track, photo} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	guard, err := NewGuard([]string{music, photos})
	if err != nil {
		t.Fatalf("NewGuard() error = %v", err)
	}
	for _, path := range []string{track, photo} {
		if _, err := guard.Resolve(path); err != nil {
			t.Errorf("Resolve(%q) = %v, want success", path, err)
		}
	}
}

func TestGuardToleratesARootThatDoesNotExistYet(t *testing.T) {
	// The music volume is often an empty mount on first boot; that must not
	// stop the server from starting.
	missing := filepath.Join(t.TempDir(), "not-created-yet")
	if _, err := NewGuard([]string{missing}); err != nil {
		t.Fatalf("NewGuard() error = %v", err)
	}
}
