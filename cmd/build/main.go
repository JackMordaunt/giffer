// build functions as a build script, building and bundling the package.
package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jackmordaunt/icns"
	"github.com/pkg/errors"
)

const plist = `
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>giffer</string>
	<key>CFBundleIconFile</key>
	<string>icon.icns</string>
	<key>CFBundleIdentifier</key>
	<string>com.jackmordaunt.giffer</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>NSSupportsAutomaticGraphicsSwitching</key>
	<true/>
	<key>NSAppTransportSecurity</key>
	<dict>
		<key>NSExceptionDomains</key>
		<dict>
			<key>localhost</key>
			<dict>
			<key>NSExceptionAllowsInsecureHTTPLoads</key>
			<true/>
			<key>NSIncludesSubdomains</key>
			<true/>
			</dict>
		</dict>
	</dict>	
</dict>
</plist>
`

// <key>ProgramArguments</key>
// <array>
// 	<string>giffer</string>
// 	<string>-v</string>
// 	<string>-ffmpeg</string>
// 	<string>Resources/ffmpeg</string>
// </array>

var (
	dist   string
	tags   string
	icon   string
	ffmpeg string
)

func init() {
	flag.StringVar(&dist, "dist", "dist", "folder to put artifact")
	flag.StringVar(&icon, "icon", "", "path to icon file (.png or .icns)")
	flag.StringVar(&tags, "tags", "", "build tags to pass to go build")
	flag.StringVar(&ffmpeg, "ffmpeg", "", "path to ffmpeg binary")
	flag.Parse()
}

func main() {
	if runtime.GOOS == "darwin" {
		var (
			contents   = filepath.Join(dist, "Giffer.app", "Contents")
			macos      = filepath.Join(dist, "Giffer.app", "Contents", "MacOS")
			resources  = filepath.Join(dist, "Giffer.app", "Contents", "Resources")
			info       = filepath.Join(dist, "Giffer.app", "Contents", "Info.plist")
			iconf      = filepath.Join(dist, "Giffer.app", "Contents", "Resources", "icon.icns")
			frameworks = filepath.Join(dist, "Giffer.app", "Contents", "Frameworks")
		)
		for _, dir := range []string{contents, macos, resources, frameworks} {
			if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
				log.Fatalf("preparing directory: %v", err)
			}
		}
		tasks := FanOut([]Task{
			Task{
				Name: "compile ui",
				Op: func() error {
					cmd := exec.Command("yarn", "build")
					cmd.Dir = "/Users/jack/dev/personal/giffer/cmd/desktop/ui"
					if out, err := cmd.CombinedOutput(); err != nil {
						return errors.Wrap(err, string(out))
					}
					return nil
				},
			},
			Task{
				Requires: []string{"compile ui"},
				Name:     "compile binary",
				Op: func() error {
					args := []string{"go", "build"}
					if tags != "" {
						args = append(args, "-tags", tags)
					}
					args = append(args, []string{
						"-o", filepath.Join(macos, "giffer"),
						"github.com/jackmordaunt/giffer/cmd/desktop",
					}...)
					if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
						return fmt.Errorf("%s: %v", string(out), err)
					}
					return nil
				},
			},
			Task{
				Requires: []string{"compile binary"},
				Name:     "embed resources",
				Op: func() error {
					cmd := exec.Command(
						"rice",
						"append",
						"--exec", filepath.Join(macos, "giffer"),
					)
					cmd.Dir = "/Users/jack/dev/personal/giffer/cmd/build"
					if out, err := cmd.CombinedOutput(); err != nil {
						return errors.Wrap(err, string(out))
					}
					return nil
				},
			},
			Task{
				Name: "create icon.icns",
				Op: func() error {
					if icon != "" {
						if err := WriteIcon(icon, iconf); err != nil {
							return errors.Wrap(err, "writing icon")
						}
					}
					return nil
				},
			},
			Task{
				Name: "create Info.plist",
				Op: func() error {
					if err := ioutil.WriteFile(info, []byte(plist), 0644); err != nil {
						return errors.Wrap(err, "writing plist")
					}
					return nil
				},
			},
			Task{
				Name: "bundle ffmpeg",
				Op: func() error {
					var (
						src io.Reader
						err error
					)
					if ffmpeg == "" {
						ffmpeg = "https://ffmpeg.zeranoe.com/builds/macos64/static/ffmpeg-latest-macos64-static.zip"
					}
					if strings.Contains(ffmpeg, "https://") {
						u, err := user.Current()
						if err != nil {
							return errors.Wrap(err, "determine current user")
						}
						downloads := filepath.Join(u.HomeDir, "Downloads")
						src, err = openZip(ffmpeg, downloads, "bin/ffmpeg")
						if err != nil {
							return errors.Wrap(err, "downloading ffmpeg")
						}
					} else {
						srcf, err := os.Open(ffmpeg)
						if err != nil {
							return errors.Wrap(err, "opening ffmpeg file")
						}
						defer srcf.Close()
						src = srcf
					}
					destf, err := os.OpenFile(
						filepath.Join(resources, "ffmpeg"),
						os.O_CREATE|os.O_RDWR,
						0777)
					if err != nil {
						return errors.Wrap(err, "opening desintaiton file")
					}
					defer destf.Close()
					if _, err := io.Copy(destf, src); err != nil {
						return errors.Wrap(err, "copying")
					}
					return nil
				},
			},
		})
		if err := tasks.Run(); err != nil {
			log.Fatalf("packaging giffer: %v", err)
		}
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
