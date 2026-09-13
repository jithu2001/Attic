package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/perleybrook/attic/server/internal/media"
)

// CoverStore writes embedded cover art into the derived directory, named by the
// SHA-256 of the image bytes.
//
// Content addressing means an album's twelve tracks embedding the same artwork
// produce one file, and a cover can be cached forever by clients because its
// URL can never point at different bytes.
type CoverStore struct {
	dir string
}

// NewCoverStore creates dir if needed and returns a store over it.
func NewCoverStore(dir string) (*CoverStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("scanner: create covers dir: %w", err)
	}
	return &CoverStore{dir: dir}, nil
}

// Dir is the directory covers are written to.
func (c *CoverStore) Dir() string { return c.dir }

// Put stores image bytes and returns their hash. Writing an image that is
// already stored is a no-op.
func (c *CoverStore) Put(data []byte, mimeType string) (string, error) {
	if len(data) == 0 {
		return "", errors.New("scanner: empty cover image")
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	path := filepath.Join(c.dir, hash+media.ImageExtensionForMIME(mimeType))

	if _, err := os.Stat(path); err == nil {
		return hash, nil
	}

	// Write to a temporary name and rename into place, so a crash mid-write
	// cannot leave a truncated image behind a valid-looking hash.
	tmp, err := os.CreateTemp(c.dir, ".cover-*")
	if err != nil {
		return "", fmt.Errorf("scanner: create temp cover: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("scanner: write cover: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("scanner: close cover: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return "", fmt.Errorf("scanner: chmod cover: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return "", fmt.Errorf("scanner: place cover: %w", err)
	}
	return hash, nil
}

// Find returns the path of a stored cover, or ErrNoCover.
func (c *CoverStore) Find(hash string) (string, error) {
	if !isHexHash(hash) {
		return "", ErrNoCover
	}
	for _, ext := range media.ImageExtensions() {
		path := filepath.Join(c.dir, hash+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", ErrNoCover
}

// ErrNoCover is returned when no image is stored under a hash.
var ErrNoCover = errors.New("scanner: no such cover")

// isHexHash guards the covers directory against traversal: a hash is the only
// thing that may ever be joined onto it.
func isHexHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
