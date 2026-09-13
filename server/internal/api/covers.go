package api

import (
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/perleybrook/attic/server/internal/media"
	"github.com/perleybrook/attic/server/internal/stream"
)

// Cover serves extracted album art by content hash.
//
// The bytes behind a hash can never change, so the response is immutable and
// cacheable for a year: an album grid scrolled twice hits the network once.
func (s *Server) Cover(w http.ResponseWriter, r *http.Request) {
	if s.covers == nil {
		WriteError(w, http.StatusNotFound, CodeNotFound, "No cover art available.")
		return
	}

	// CoverStore validates the hash before joining it onto a path, so a
	// "../../etc/passwd" hash never becomes a filesystem path.
	path, err := s.covers.Find(chi.URLParam(r, "hash"))
	if err != nil {
		WriteError(w, http.StatusNotFound, CodeNotFound, "No such cover.")
		return
	}

	file, err := openPlainFile(path)
	if err != nil {
		WriteError(w, http.StatusNotFound, CodeNotFound, "No such cover.")
		return
	}
	defer file.Close()

	stream.ServeFile(w, r, file, media.ImageMIMEForExtension(filepath.Ext(path)), true)
}

// openPlainFile opens a derived-cache file. Derived files are not subject to
// the library-root guard: they are ours, named by hash, and never user input.
func openPlainFile(path string) (*stream.File, error) {
	f, err := osOpen(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &stream.File{File: f, Info: info}, nil
}
