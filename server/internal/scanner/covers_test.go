package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverStoreIsContentAddressed(t *testing.T) {
	store, err := NewCoverStore(filepath.Join(t.TempDir(), "covers"))
	if err != nil {
		t.Fatalf("NewCoverStore() error = %v", err)
	}

	data := []byte("pretend this is a jpeg")

	first, err := store.Put(data, "image/jpeg")
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	second, err := store.Put(data, "image/jpeg")
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	// The same artwork on twelve tracks must produce one file.
	if first != second {
		t.Errorf("identical images hashed differently: %q and %q", first, second)
	}
	entries, err := os.ReadDir(store.Dir())
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("stored %d files, want 1", len(entries))
	}

	path, err := store.Find(first)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != string(data) {
		t.Error("stored bytes do not round-trip")
	}
}

func TestCoverStoreKeepsTheSourceFormat(t *testing.T) {
	store, err := NewCoverStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewCoverStore() error = %v", err)
	}

	hash, err := store.Put([]byte("\x89PNG..."), "image/png")
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	path, err := store.Find(hash)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if !strings.HasSuffix(path, ".png") {
		t.Errorf("stored at %q, want a .png", path)
	}
}

func TestCoverStoreRejectsNonHashes(t *testing.T) {
	dir := t.TempDir()
	store, err := NewCoverStore(dir)
	if err != nil {
		t.Fatalf("NewCoverStore() error = %v", err)
	}

	// The hash goes straight into a filesystem path, so anything that is not a
	// hash must be refused before it gets there.
	for _, bad := range []string{
		"",
		"../../../etc/passwd",
		"..",
		strings.Repeat("g", 64),
		strings.Repeat("a", 63),
		strings.Repeat("A", 64),
	} {
		if _, err := store.Find(bad); err == nil {
			t.Errorf("Find(%q) succeeded, want a rejection", bad)
		}
	}
}

func TestCoverStoreRejectsEmptyImages(t *testing.T) {
	store, err := NewCoverStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewCoverStore() error = %v", err)
	}
	if _, err := store.Put(nil, "image/jpeg"); err == nil {
		t.Error("Put(nil) succeeded, want an error")
	}
}
