package giffer

import (
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-multierror"

	"github.com/OneOfOne/xxhash"
	"github.com/disintegration/imaging"

	"github.com/pkg/errors"
)

// FFMpeg wraps the ffmpeg binary.
type FFMpeg struct {
	Dir string
}

// Extract the frames between start and end from the video file.
func (f FFMpeg) Extract(video string, start, end, fps float64) ([]image.Image, error) {
	cut, err := f.Cut(video, start, end)
	if err != nil {
		return nil, errors.Wrap(err, "cutting video file")
	}
	if fps == 0 {
		fps = 24.4
	}
	hasher := xxhash.New64()
	input := fmt.Sprintf("%s_%f_%f_%f", video, start, end, fps)
	if _, err := hasher.WriteString(input); err != nil {
		return nil, errors.Wrap(err, "hashing input")
	}
	dir := filepath.Join(f.Dir, fmt.Sprintf("%d", hasher.Sum64()))
	info, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "inspecting output directory")
	}
	if os.IsNotExist(err) {
		// Wrap work in a closure so we can scope err and defer a cleanup
		// function.
		// The cleanup is necessary because we only check for existence
		// of files, not validity.
		err := func() (err error) {
			log.Printf("making frames")
			defer func() {
				if err != nil {
					if cleanup := os.RemoveAll(dir); cleanup != nil {
						err = multierror.Append(err, cleanup)
					}
				}
			}()
			if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
				return errors.Wrap(err, "preparing directory")
			}
			if err := f.run(
				"-i", cut,
				"-vf", fmt.Sprintf("fps=%2f", fps),
				filepath.Join(dir, "$frame%03d.jpg"),
			); err != nil {
				return errors.Wrap(err, "extracting frames")
			}
			// Since ffmpeg doesn't always return an error we need to manually check
			// for the expected output. This is naive, simply checking that
			// the directory isn't empty.
			entries, err := ioutil.ReadDir(dir)
			if err != nil {
				return errors.Wrap(err, "reading output directory")
			}
			if len(entries) == 0 {
				return fmt.Errorf("no frames found (inspect ffmpeg output)")
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}
	if info != nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("inspecting output directory: got a file, not a directory")
		}
	}
	var frames []image.Image
	walk := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		frame, err := imaging.Open(path)
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
	if err := os.MkdirAll(f.Dir, 0755); err != nil && !os.IsExist(err) {
		return "", errors.Wrap(err, "preparing directory")
	}
	hasher := xxhash.New64()
	input := fmt.Sprintf("%s_%f_%f", video, start, end)
	if _, err := hasher.WriteString(input); err != nil {
		return "", errors.Wrap(err, "hashing input")
	}
	cut := filepath.Join(f.Dir, fmt.Sprintf("%d.mp4", hasher.Sum64()))
	info, err := os.Stat(cut)
	if err != nil && !os.IsNotExist(err) {
		return "", errors.Wrap(err, "inspecting cut file")
	}
	if info != nil {
		if info.IsDir() {
			return "", errors.Wrap(err, "expected file, got directory")
		}
		return cut, nil
	}
	log.Printf("cutting file.")
	if err := f.run(
		"-ss", fmt.Sprintf("%4f", start),
		"-t", fmt.Sprintf("%4f", end-start),
		"-i", video,
		"-c", "copy", cut,
	); err != nil {
		return "", errors.Wrap(err, "ffmpeg")
	}
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
