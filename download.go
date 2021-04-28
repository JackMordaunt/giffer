package giffer

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/OneOfOne/xxhash"
	"github.com/kkdai/youtube/v2"
	"github.com/kkdai/youtube/v2/downloader"
	"github.com/pkg/errors"
)

// Downloader is responsible for downloading videos.
type Downloader struct {
	Dir    string
	FFmpeg string
	Debug  bool
	Out    io.Writer
}

// Download the video from URL into Dir and return the full path to the
// downloaded file.
func (dl Downloader) Download(
	URL string,
	start, end float64,
	q Quality,
) (string, error) {
	dl.logf("ffmpeg: %q\n", dl.FFmpeg)
	input := fmt.Sprintf("%s_%f_%f_%s", URL, start, end, q)
	h, err := hash(input)
	if err != nil {
		return "", errors.Wrap(err, "creating hash")
	}
	dir := filepath.Join(dl.Dir, h)
	// Return cached file if it exists.
	entries, _ := ioutil.ReadDir(dir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), fmt.Sprintf("%s_", q.String())) {
			return filepath.Join(dir, entry.Name()), nil
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
		return "", errors.Wrap(err, "preparing directories")
	}

	tmp, err := dl.download(URL, start, end, q)
	if err != nil {
		return "", err
	}
	real := filepath.Join(dir, fmt.Sprintf("%s_%s", q, h+filepath.Ext(tmp)))
	if err := os.Rename(tmp, real); err != nil {
		return "", errors.Wrap(err, "renaming temporary file")
	}
	return real, nil
}

func (dl Downloader) logf(f string, v ...interface{}) {
	if !dl.Debug || dl.Out == nil {
		return
	}
	fmt.Fprintf(dl.Out, f, v...)
}

// Quality is an enum representing the various video qualities.
type Quality int

// Matches returns true if the input represents the quality as a string.
// This is primarily an adaptor so we don't have to change the video-downloader
// package.
func (q Quality) Matches(str string) bool {
	var patterns []string
	switch q {
	case Best:
		patterns = append(patterns, "1080p")
	case High:
		patterns = append(patterns, "720p", "480p")
	case Medium:
		patterns = append(patterns, "360p")
	case Low:
		patterns = append(patterns, "240p", "144p")
	}
	for _, p := range patterns {
		if strings.Contains(str, p) {
			return true
		}
	}
	return false
}

const (
	// Low 144p
	Low Quality = iota
	// Medium 360p
	Medium
	// High 720p
	High
	// Best 1080p@60 > 720p@60 > 1080p > 720p
	Best
)

func (q Quality) String() string {
	switch q {
	case Low:
		return "low"
	case Medium:
		return "medium"
	case High:
		return "high"
	case Best:
		return "best"
	default:
		return "unknown"
	}
}

// download a video from the specified url.
func (dl Downloader) download(
	videoURL string,
	start, end float64,
	quality Quality,
) (string, error) {
	d := downloader.Downloader{
		Client: youtube.Client{},
	}
	v, err := d.GetVideo(videoURL)
	if err != nil {
		return "", fmt.Errorf("getting video: %w", err)
	}
	outf := filepath.Join(os.TempDir(), v.Title)
	return outf, d.Download(context.TODO(), v, &v.Formats[0], outf)
}

func hash(input string) (string, error) {
	h := xxhash.New64()
	if _, err := h.WriteString(input); err != nil {
		return "", errors.Wrap(err, "writing to hash object")
	}
	return fmt.Sprintf("%d", h.Sum64()), nil
}
