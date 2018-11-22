package giffer

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// Engine implements video and image manipulation.
type Engine struct {
	FFmpeg  string    // Path to FFmpeg binary.
	Convert string    // Path to imagemagick Convert binary.
	Debug   bool      // Print commands used.
	Out     io.Writer // Writer to use if debug is true.
	Junk    []string  // Temporary files to cleanup.
}

// Cut and merge the target file into the specified time slices.
// Cuts is a slice of int pairs which are start and end times (in seconds)
// respectively.
// Returns a filepath to the merged file.
func (eng *Engine) Cut(video string, cuts ...[2]int) (string, error) {
	var (
		cutfiles []string
		entries  []string
		filelist = "tmp_file_list.txt"
		merged   = fmt.Sprintf("merged%s", filepath.Ext(video))
	)
	defer func() {
		eng.Junk = append(eng.Junk, append(cutfiles, filelist, merged)...)
	}()
	for ii, c := range cuts {
		start, end := c[0], c[1]
		if start > end {
			return "", fmt.Errorf("start > end: %d > %d", start, end)
		}
		output := fmt.Sprintf("tmp_%d%s", ii, filepath.Ext(video))
		cutSlice := eng.command(
			eng.FFmpeg,
			"-ss", fmt.Sprintf("%d", start),
			"-t", fmt.Sprintf("%d", end-start),
			"-i", video,
			output,
		)
		if out, err := cutSlice.CombinedOutput(); err != nil {
			return "", errors.Wrapf(err, "cutting video: %s", string(out))
		}
		cutfiles = append(cutfiles, output)
	}
	for _, f := range cutfiles {
		entries = append(entries, fmt.Sprintf("file '%s'", f))
	}
	if err := ioutil.WriteFile(
		filelist,
		[]byte(strings.Join(entries, "\n")),
		0644,
	); err != nil {
		return "", errors.Wrap(err, "creating file list for concatentation")
	}
	merge := eng.command(
		eng.FFmpeg,
		"-f", "concat",
		"-i", filelist,
		"-c", "copy",
		merged,
	)
	if out, err := merge.CombinedOutput(); err != nil {
		return "", errors.Wrapf(err, "merging cut files: %s", string(out))
	}
	return merged, nil
}

// Transcode the target video file into a gif.
// Returns a filepath to the gif image.
func (eng *Engine) Transcode(
	video string,
	start, end float64,
	width, height int,
	fps float64,
) (string, error) {
	var (
		duration   = end - start
		filters    string
		palettegen string
		output     = fmt.Sprintf("%s.gif", strings.Split(video, ".")[0])
	)
	if height < -2 {
		height = -2
	}
	if width < -2 {
		width = -2
	}
	if fps > 0.0 {
		filters += fmt.Sprintf("fps=%2f", fps)
	}
	if width > 0.0 || height > 0.0 {
		if filters != "" {
			filters += ","
		}
		filters += fmt.Sprintf("scale=%d:%d:flags=lanczos", width, height)
	}
	if len(filters) > 0 {
		palettegen = fmt.Sprintf("%s,palettegen", filters)
	} else {
		palettegen = "palettegen"
	}
	defer func() {
		eng.Junk = append(eng.Junk, "palette.png", output)
	}()
	genPalette := eng.command(
		eng.FFmpeg,
		"-ss", fmt.Sprintf("%2f;omitempty", start),
		"-t", fmt.Sprintf("%2f;omitempty", duration),
		"-i", video,
		"-vf", palettegen,
		"-y", "palette.png",
	)
	if out, err := genPalette.CombinedOutput(); err != nil {
		return "", errors.Wrapf(err, "generating palette: %s", string(out))
	}
	makeGif := eng.command(
		eng.FFmpeg,
		"-ss", fmt.Sprintf("%2f;omitempty", start),
		"-t", fmt.Sprintf("%2f;omitempty", duration),
		"-i", video, "-i", "palette.png",
		"-lavfi", fmt.Sprintf("%s [x]; [x][1:v] paletteuse", filters),
		"-y", output,
	)
	if out, err := makeGif.CombinedOutput(); err != nil {
		return "", errors.Wrapf(err, "making gif: %s", string(out))
	}
	return output, nil
}

// Crush reduces the file size of a gif image.
// Accepts a filepath to the gif image and replaces it with the crushed gif.
// Fuzz is a percentage value between 0 and 100, where 0 is best quality, 100 is
// smallest file size. Optimal is typically 2-5.
func (eng *Engine) Crush(gif string, fuzz int) error {
	if eng.Convert == "" {
		return nil
	}
	args := []string{gif}
	if fuzz > 0 {
		args = append(args, "-fuzz", fmt.Sprintf("%d%%", fuzz))
	}
	args = append(args, "-layers", "Optimize", gif)
	crushGif := eng.command(eng.Convert, args...)
	if out, err := crushGif.CombinedOutput(); err != nil {
		return errors.Wrap(err, string(out))
	}
	return nil
}

// Clean the temporary files.
func (eng *Engine) Clean() {
	for _, f := range eng.Junk {
		if err := os.Remove(f); err != nil {
			eng.logf("clean: %v\n", err)
		}
	}
}

// command creates a new exec.Cmd after removing empty arguments.
// If an argument value contains "<value>;omitempty" and <value> is a zero
// value, the argument value and it's corresponding argument specifier are
// considered "empty" and omitted.
func (eng *Engine) command(cmd string, args ...string) *exec.Cmd {
	var a []string
	for ii := 0; ii < len(args); ii++ {
		arg := args[ii]
		if arg[0] == '-' { // This is an argument specifier eg "-h".
			if v := args[ii+1]; v == "" {
				ii++
				continue
			} else if strings.Contains(v, ";omitempty") {
				// Handle special ;omitempty directive.
				v = strings.Split(v, ";omitempty")[0]
				if v == "" {
					ii++
					continue
				}
				x, _ := strconv.Atoi(v)
				if x == 0 {
					ii++
					continue
				}
				fl, _ := strconv.ParseFloat(v, 64)
				if fl <= 0.0 {
					ii++
					continue
				}
				args[ii+1] = v
			}
		}
		a = append(a, arg)
	}
	if eng.Debug {
		eng.logf("%s %s\n", cmd, strings.Join(a, " "))
	}
	return exec.Command(cmd, a...)
}

func (eng *Engine) logf(f string, v ...interface{}) (int, error) {
	if eng.Debug && eng.Out != nil {
		return fmt.Fprintf(eng.Out, f, v...)
	}
	return 0, nil
}
