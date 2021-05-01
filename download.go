package giffer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
) (string, error) {
	dl.logf("ffmpeg: %q\n", dl.FFmpeg)
	h, err := hash(fmt.Sprintf("%s_%f_%f", URL, start, end))
	if err != nil {
		return "", errors.Wrap(err, "creating hash")
	}
	dir := filepath.Join(dl.Dir, h)
	if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
		return "", errors.Wrap(err, "preparing directories")
	}
	tmp, err := dl.download(URL, start, end)
	if err != nil {
		return "", err
	}
	real := filepath.Join(dir, h+filepath.Ext(tmp))
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

// download a video from the specified url.
func (dl Downloader) download(
	videoURL string,
	start, end float64,
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
