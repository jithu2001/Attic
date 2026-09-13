// Package stream serves media files to clients.
package stream

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrOutsideLibrary is returned when a path does not resolve inside any
// configured library root.
var ErrOutsideLibrary = errors.New("stream: path is outside the library")

// Guard resolves file paths against the configured library roots. Every media
// handler goes through it, so a row with a doctored path — or a symlink
// pointing at /etc/shadow — cannot be served.
type Guard struct {
	roots []string
}

// NewGuard resolves each root to its real absolute path.
func NewGuard(roots []string) (*Guard, error) {
	resolved := make([]string, 0, len(roots))
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("stream: resolve library root %s: %w", root, err)
		}
		// A root that does not exist yet is kept as-is: the music directory is
		// often an empty mount on first boot.
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			abs = real
		}
		resolved = append(resolved, filepath.Clean(abs))
	}
	return &Guard{roots: resolved}, nil
}

// Resolve returns the real path of name, or ErrOutsideLibrary.
//
// Symlinks are followed before the check, so a link inside the library that
// points outside it is rejected rather than followed.
func (g *Guard) Resolve(name string) (string, error) {
	abs, err := filepath.Abs(name)
	if err != nil {
		return "", ErrOutsideLibrary
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", ErrOutsideLibrary
	}
	real = filepath.Clean(real)

	for _, root := range g.roots {
		if real == root {
			return real, nil
		}
		if strings.HasPrefix(real, root+string(filepath.Separator)) {
			return real, nil
		}
	}
	return "", ErrOutsideLibrary
}

// File is an opened media file ready to be served.
type File struct {
	*os.File
	Info os.FileInfo
}

// Open resolves and opens a media file.
func (g *Guard) Open(name string) (*File, error) {
	path, err := g.Resolve(name)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if info.IsDir() {
		f.Close()
		return nil, ErrOutsideLibrary
	}
	return &File{File: f, Info: info}, nil
}

// ServeFile writes f to the response with full Range support.
//
// http.ServeContent does the heavy lifting: it parses Range headers, answers
// with 206 and a Content-Range, handles If-Range and multipart ranges, and
// emits Accept-Ranges. That is what makes seeking in a 40-minute FLAC instant
// instead of a re-download.
func ServeFile(w http.ResponseWriter, r *http.Request, f *File, contentType string, immutable bool) {
	w.Header().Set("Content-Type", contentType)
	if immutable {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		// Originals never change, but their row might be re-pointed, so the
		// client revalidates instead of caching forever.
		w.Header().Set("Cache-Control", "private, max-age=60")
	}
	http.ServeContent(w, r, f.Info.Name(), modTime(f.Info), f.File)
}

func modTime(info os.FileInfo) time.Time {
	if info == nil {
		return time.Time{}
	}
	return info.ModTime()
}
