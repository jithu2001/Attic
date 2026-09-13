// Package media knows which files Attic treats as media and what MIME types
// they are served with. It is deliberately tiny and dependency-free so both
// the scanner and the streaming handlers can share it.
package media

import (
	"path/filepath"
	"strings"
)

// audioTypes maps a lowercase extension to the MIME type Attic serves it as.
//
// The list is deliberately conservative: every entry is something both
// dhowden/tag can read tags from and just_audio can play natively, so a file
// that scans is a file that plays.
var audioTypes = map[string]string{
	".mp3":  "audio/mpeg",
	".flac": "audio/flac",
	".m4a":  "audio/mp4",
	".m4b":  "audio/mp4",
	".aac":  "audio/aac",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".opus": "audio/opus",
	".wav":  "audio/wav",
	".wma":  "audio/x-ms-wma",
	".aiff": "audio/aiff",
	".aif":  "audio/aiff",
}

// IsAudio reports whether path looks like a music file Attic can ingest.
func IsAudio(path string) bool {
	_, ok := audioTypes[strings.ToLower(filepath.Ext(path))]
	return ok
}

// AudioMIME returns the MIME type for path, or application/octet-stream if the
// extension is unknown.
func AudioMIME(path string) string {
	if mime, ok := audioTypes[strings.ToLower(filepath.Ext(path))]; ok {
		return mime
	}
	return "application/octet-stream"
}

// imageExtensions are the cover-art formats that may be written to the derived
// covers directory, in the order the cover handler probes for them.
var imageExtensions = []string{".jpg", ".png", ".webp", ".gif"}

// ImageExtensions lists the extensions a cover file may carry on disk.
func ImageExtensions() []string { return imageExtensions }

// ImageExtensionForMIME picks the on-disk extension for an embedded picture.
// Unknown types are stored as .jpg, which is what all but a handful of taggers
// actually embed.
func ImageExtensionForMIME(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

// ImageMIMEForExtension is the inverse, used when serving a cover back.
func ImageMIMEForExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}
