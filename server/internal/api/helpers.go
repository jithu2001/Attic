package api

import (
	"net/http"
	"os"
	"sort"

	"github.com/perleybrook/attic/server/internal/stream"
)

var osOpen = os.Open

func sortStrings(s []string) { sort.Strings(s) }

// serveOriginal is a thin alias so handlers read consistently; the caching
// policy for originals lives in the stream package.
func serveOriginal(w http.ResponseWriter, r *http.Request, f *stream.File, contentType string) {
	stream.ServeFile(w, r, f, contentType, false)
}

// flushWriter pushes each chunk of a transcode straight to the client instead
// of waiting for a buffer to fill, so playback starts in about a second rather
// than whenever 32 KB has accumulated.
type flushWriter struct{ w http.ResponseWriter }

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if flusher, ok := fw.w.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}
