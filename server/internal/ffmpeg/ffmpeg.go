// Package ffmpeg wraps the FFmpeg and ffprobe binaries. They are always
// external processes: Attic never links libav.
package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"time"
)

// Tools locates the external binaries.
type Tools struct {
	FFmpegPath  string
	FFprobePath string
}

// New builds a Tools from configured paths.
func New(ffmpegPath, ffprobePath string) *Tools {
	return &Tools{FFmpegPath: ffmpegPath, FFprobePath: ffprobePath}
}

// Available reports whether both binaries can be found on PATH.
func (t *Tools) Available() error {
	if _, err := exec.LookPath(t.FFmpegPath); err != nil {
		return fmt.Errorf("ffmpeg: %w", err)
	}
	if _, err := exec.LookPath(t.FFprobePath); err != nil {
		return fmt.Errorf("ffprobe: %w", err)
	}
	return nil
}

// Probe is the subset of ffprobe's output Attic reads. The full JSON is kept
// verbatim in media_files.probe, so later phases can mine it without rescanning.
type Probe struct {
	Raw       []byte
	DurationS float64
	Bitrate   int
	Codec     string
	Channels  int
	SampleHz  int
}

type probeJSON struct {
	Format struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		CodecType  string `json:"codec_type"`
		CodecName  string `json:"codec_name"`
		Channels   int    `json:"channels"`
		SampleRate string `json:"sample_rate"`
		Duration   string `json:"duration"`
	} `json:"streams"`
}

// ProbeFile runs ffprobe and returns both the parsed highlights and the raw
// JSON document.
func (t *Tools) ProbeFile(ctx context.Context, path string) (*Probe, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.FFprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe %s: %w: %s", path, err, stderr.String())
	}

	raw := stdout.Bytes()
	var parsed probeJSON
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("ffprobe %s: parse output: %w", path, err)
	}

	p := &Probe{Raw: append([]byte(nil), raw...)}
	p.DurationS, _ = strconv.ParseFloat(parsed.Format.Duration, 64)
	p.Bitrate, _ = strconv.Atoi(parsed.Format.BitRate)

	for _, stream := range parsed.Streams {
		if stream.CodecType != "audio" {
			continue
		}
		p.Codec = stream.CodecName
		p.Channels = stream.Channels
		p.SampleHz, _ = strconv.Atoi(stream.SampleRate)
		// Some containers only carry duration per stream.
		if p.DurationS == 0 {
			p.DurationS, _ = strconv.ParseFloat(stream.Duration, 64)
		}
		break
	}

	return p, nil
}

// TranscodeAudio decodes path and re-encodes it to Opus in a WebM container,
// writing to w.
//
// One process, one response, no session machinery: the client asks for a
// transcoded stream and gets a stream. That costs seekability (the output is
// not seekable, so the app only asks for this on a slow link), and it means a
// cancelled request kills the encoder, which is exactly what should happen.
func (t *Tools) TranscodeAudio(ctx context.Context, path string, bitrateKbps int, w io.Writer) error {
	cmd := exec.CommandContext(ctx, t.FFmpegPath,
		"-hide_banner",
		"-loglevel", "error",
		"-i", path,
		"-vn", // drop embedded cover art
		"-map", "0:a:0",
		"-c:a", "libopus",
		"-b:a", strconv.Itoa(bitrateKbps)+"k",
		"-vbr", "on",
		"-f", "webm",
		"pipe:1",
	)

	var stderr bytes.Buffer
	cmd.Stdout = w
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// A client that hangs up mid-track cancels the context and kills the
		// process; that is normal, not a failure worth surfacing.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg transcode %s: %w: %s", path, err, stderr.String())
	}
	return nil
}
