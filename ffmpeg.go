package giffer

import (
	"bytes"
	"fmt"
	"image"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/disintegration/imaging"

	"github.com/pkg/errors"
)

// FFMpeg wraps the ffmpeg binary.
type FFMpeg struct {
	Dir       string
	LeaveMess bool
}

// Extract the frames between start and end from the video file.
func (f FFMpeg) Extract(video string, start, end, fps float64) ([]image.Image, error) {
	os.RemoveAll(f.Dir)
	if !f.LeaveMess {
		defer os.RemoveAll(f.Dir)
	}
	err := os.MkdirAll(filepath.Join(f.Dir, "frames"), 0755)
	if err != nil && err != os.ErrExist {
		return nil, errors.Wrap(err, "preparing directories")
	}
	cut, err := f.Cut(video, start, end)
	if err != nil {
		return nil, errors.Wrap(err, "cutting video file")
	}
	if fps == 0 {
		fps = 24.4
	}
	if err := f.run(
		"-i", cut,
		"-vf", fmt.Sprintf("fps=%2f", fps),
		filepath.Join(f.Dir, "frames", "$frame%03d.jpg"),
	); err != nil {
		return nil, errors.Wrap(err, "extracting frames")
	}
	dir := filepath.Join(f.Dir, "frames")
	// Since ffmpeg doesn't always return an error we need to manually check
	// for the expected output.
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("no frames found (inspect ffmpeg output)")
	} else if err != nil {
		return nil, errors.Wrap(err, "checking output")
	}
	var frames []image.Image
	walk := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// Note: Unsure if imaging buffers the file, so we buffer it
		// 	here. This avoids invalidating the images when LeaveMess
		// 	is false and the files are removed.
		img, err := ioutil.ReadFile(path)
		if err != nil {
			return errors.Wrap(err, "opening frame")
		}
		frame, err := imaging.Decode(bytes.NewBuffer(img))
		if err != nil {
			return errors.Wrap(err, "decoding frame")
		}
		frames = append(frames, frame)
		return nil
	}
	if err := filepath.Walk(dir, walk); err != nil {
		return nil, errors.Wrap(err, "walking")
	}
	return frames, nil
}

// Cut the video file from start to end (in seconds).
// The returned string is the path to the resulting file.
func (f FFMpeg) Cut(video string, start, end float64) (string, error) {
	if start > end {
		return "", fmt.Errorf("start > end: %f > %f", start, end)
	}
	if start < 0 {
		return "", fmt.Errorf("start < 0: %f < 0", start)
	}
	if err := f.run(
		"-ss", fmt.Sprintf("%4f", start),
		"-t", fmt.Sprintf("%4f", end-start),
		"-i", video,
		"-c", "copy", filepath.Join(f.Dir, "cut.mp4"),
	); err != nil {
		return "", errors.Wrap(err, "ffmpeg")
	}
	cut := filepath.Join(f.Dir, "cut.mp4")
	if _, err := os.Stat(cut); os.IsNotExist(err) {
		return "", fmt.Errorf("cut failed: no output file detected (inspect ffmpeg output)")
	} else if err != nil {
		return "", errors.Wrap(err, "checking output")
	}
	return cut, nil
}

// IsInstalled checks whether FFMpeg is available on the system PATH.
func (f FFMpeg) IsInstalled() bool {
	return f.run() == nil
}

func (f FFMpeg) run(args ...string) error {
	out, err := exec.Command("ffmpeg", args...).CombinedOutput()
	if err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}
