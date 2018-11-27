// build functions as a build script, building and bundling the package.
package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jackmordaunt/icns"
	"github.com/pkg/errors"
)

var (
	dist   string
	tags   string
	icon   string
	ui     string
	ffmpeg string
)

func init() {
	flag.StringVar(&dist, "dist", "dist", "folder to put artifact")
	flag.StringVar(&icon, "icon", icon, "path to icon file (.png or .icns)")
	flag.StringVar(&tags, "tags", tags, "build tags to pass to go build")
	flag.StringVar(&ffmpeg, "ffmpeg", ffmpeg, "path to ffmpeg binary")
	flag.StringVar(&ui, "ui", ui, "path to UI files")
	flag.Parse()
}

func main() {
	switch runtime.GOOS {
	case "darwin":
		builder := &Darwin{
			Bin:       "giffer",
			Pkg:       "github.com/jackmordaunt/giffer/cmd/desktop",
			UI:        ui,
			Icon:      icon,
			FFmpeg:    ffmpeg,
			BuildTags: tags,
		}
		if err := builder.Build(dist); err != nil {
			log.Fatalf("building Giffer for MacOS: %v", err)
		}
	case "windows":
		builder := &Windows{
			Bin:       "giffer.exe",
			Pkg:       "github.com/jackmordaunt/giffer/cmd/desktop",
			UI:        ui,
			Icon:      icon,
			FFmpeg:    ffmpeg,
			BuildTags: tags,
		}
		if err := builder.Build(dist); err != nil {
			log.Fatalf("building Giffer for Windows: %v", err)
		}
	default:
		log.Fatalf("OS %s is not supported", runtime.GOOS)
	}

}

// WriteIcon writes out the icon.icns using specified source icon.
// If the source icon is a .png then it is converted to .icns.
func WriteIcon(in, out string) error {
	var (
		src io.ReadWriter
		ext = filepath.Ext(in)
	)
	if ext != ".png" && ext != ".icns" {
		return fmt.Errorf("icon must be .icns or .png: %s", icon)
	}
	inf, err := os.Open(in)
	if err != nil {
		return errors.Wrap(err, "opening input file")
	}
	defer inf.Close()
	src = inf
	if ext == ".png" {
		img, err := png.Decode(inf)
		if err != nil {
			return errors.Wrap(err, "decoding png")
		}
		buf := bytes.NewBuffer(nil)
		if err := icns.Encode(buf, img); err != nil {
			return errors.Wrap(err, "encoding icns")
		}
		src = buf
	}
	outf, err := os.Create(out)
	if err != nil {
		return errors.Wrap(err, "creating output file")
	}
	defer outf.Close()
	if _, err := io.Copy(outf, src); err != nil {
		return err
	}
	return nil
}

func omitEmpty(flag, value string) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf("%s=%s", flag, value)
}

// openZip returns the first file in the zip that contains the pattern.
func openZip(u, dir, pattern string) (*bytes.Buffer, error) {
	file, err := download(u, filepath.Join(dir, "ffmpeg.zip"))
	if err != nil {
		return nil, errors.Wrap(err, "downloading")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Wrap(err, "getting file info")
	}
	zr, err := zip.NewReader(file, info.Size())
	if err != nil {
		return nil, errors.Wrap(err, "creating zip reader")
	}
	for _, zf := range zr.File {
		if strings.Contains(zf.Name, pattern) {
			var buf = bytes.NewBuffer(nil)
			zfr, err := zf.Open()
			if err != nil {
				return nil, errors.Wrap(err, "reading zipped file")
			}
			defer zfr.Close()
			if _, err := io.Copy(buf, zfr); err != nil {
				return nil, errors.Wrap(err, "buffering zipped file")
			}
			return buf, nil
		}
	}
	return nil, fmt.Errorf("no file found in zip for given pattern %q", pattern)
}

// download the file at the specified url.
// If a file exists at dst, that file is returned.
func download(u, dst string) (*os.File, error) {
	dstf, err := os.Open(dst)
	if err != nil && !os.IsNotExist(err) {
		return nil, errors.Wrap(err, "opening downloaded file")
	}
	if err == nil {
		return dstf, nil
	}
	resp, err := http.Get(u)
	if err != nil {
		return nil, errors.Wrap(err, "GET")
	}
	defer resp.Body.Close()
	if !(resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices) {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	zipf, err := os.Create(dst)
	if err != nil {
		return nil, errors.Wrap(err, "creating destination file")
	}
	if _, err := io.Copy(zipf, resp.Body); err != nil {
		return nil, errors.Wrap(err, "copying body")
	}
	return zipf, nil
}
