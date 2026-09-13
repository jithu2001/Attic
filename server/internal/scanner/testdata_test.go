package scanner

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// Attic's test fixtures are hand-built ID3v2.4 files rather than real
// recordings: dhowden/tag identifies MP3s by their ID3 header and reads the
// frames, so a tag followed by a few bytes of filler exercises exactly the
// code path a real file does, without checking a megabyte of audio into git.

type id3Frame struct {
	id      string
	payload []byte
}

// textFrame builds a UTF-8 text frame (encoding byte 0x03).
func textFrame(id, value string) id3Frame {
	return id3Frame{id: id, payload: append([]byte{0x03}, []byte(value)...)}
}

// pictureFrame builds an APIC frame holding embedded cover art.
func pictureFrame(mimeType string, image []byte) id3Frame {
	var payload bytes.Buffer
	payload.WriteByte(0x03) // UTF-8
	payload.WriteString(mimeType)
	payload.WriteByte(0x00)
	payload.WriteByte(0x03) // picture type: front cover
	payload.WriteString("") // description
	payload.WriteByte(0x00)
	payload.Write(image)
	return id3Frame{id: "APIC", payload: payload.Bytes()}
}

// syncsafe encodes a length the way ID3v2 sizes are encoded: seven bits per
// byte, so a size can never look like an MPEG sync word.
func syncsafe(n int) []byte {
	return []byte{
		byte((n >> 21) & 0x7f),
		byte((n >> 14) & 0x7f),
		byte((n >> 7) & 0x7f),
		byte(n & 0x7f),
	}
}

// writeMP3 writes a tagged file and returns its path.
func writeMP3(t *testing.T, path string, frames ...id3Frame) string {
	t.Helper()

	var body bytes.Buffer
	for _, frame := range frames {
		body.WriteString(frame.id)
		body.Write(syncsafe(len(frame.payload)))
		binary.Write(&body, binary.BigEndian, uint16(0)) // flags
		body.Write(frame.payload)
	}

	var file bytes.Buffer
	file.WriteString("ID3")
	file.Write([]byte{0x04, 0x00}) // version 2.4.0
	file.WriteByte(0x00)           // flags
	file.Write(syncsafe(body.Len()))
	file.Write(body.Bytes())
	// Filler standing in for audio frames. Nothing decodes it in these tests;
	// it exists so the files have distinct sizes and contents.
	file.Write(bytes.Repeat([]byte{0xff, 0xfb, 0x90, 0x00}, 64))

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, file.Bytes(), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
