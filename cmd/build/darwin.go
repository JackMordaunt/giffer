package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const plist = `
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
		<string>giffer.sh</string>
	<key>CFBundleIconFile</key>
		<string>icon.icns</string>
	<key>CFBundleIdentifier</key>
		<string>com.jackmordaunt.giffer</string>
	<key>NSHighResolutionCapable</key>
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

// Darwin builds Giffer as a .app for MacOS.
type Darwin struct {
	// Bin is the name of the built binary.
	Bin string
	// Pkg is the go package import string ie,
	// "github.com/jackmordaunt/giffer/desktop".
	Pkg string
	// UI is the root path to the UI files.
	UI string
	// FFmpeg is the URL or filepath to a static FFmpeg binary.
	FFmpeg string
	// Icon is the path to the icon file (.icns or .png).
	Icon string
	// BuildTags to pass to go build.
	BuildTags string
}

// Build executes the build and writes the result to dist.
func (app *Darwin) Build(dist string) error {
	var (
		contents  = filepath.Join(dist, "Giffer.app", "Contents")
		macos     = filepath.Join(dist, "Giffer.app", "Contents", "MacOS")
		resources = filepath.Join(dist, "Giffer.app", "Contents", "Resources")
		info      = filepath.Join(dist, "Giffer.app", "Contents", "Info.plist")
		iconf     = filepath.Join(dist, "Giffer.app", "Contents", "Resources", "icon.icns")
	)
	for _, dir := range []string{contents, macos, resources} {
		if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
			return errors.Wrap(err, "preparing directory")
		}
	}
	tasks := FanOut([]Task{
		Task{
			Name: "compile ui",
			Op: func() error {
				ui, err := filepath.Abs(app.UI)
				if err != nil {
					return errors.Wrap(err, "resolving absolute path to UI files")
				}
				app.UI = ui
				var (
					src          = filepath.Join(app.UI, "src")
					checksumFile = filepath.Join(app.UI, "checksum")
				)
				h, err := Hash(src)
				if err != nil {
					return errors.Wrap(err, "hashing ui files")
				}
				checksum, err := ioutil.ReadFile(checksumFile)
				if err != nil && !os.IsNotExist(err) {
					return errors.Wrap(err, "reading checksum file")
				}
				if !os.IsNotExist(err) && h == binary.LittleEndian.Uint64(checksum) {
					return nil
				}
				cmd := exec.Command("yarn", "build")
				cmd.Dir = app.UI
				if out, err := cmd.CombinedOutput(); err != nil {
					return errors.Wrap(err, string(out))
				}
				buf := make([]byte, 64)
				binary.LittleEndian.PutUint64(buf, h)
				return ioutil.WriteFile(checksumFile, buf, 0644)
			},
		},
		Task{
			Name: "write wrapper script",
			Op: func() error {
				wrapper := strings.Join([]string{
					"#!/usr/bin/env bash\n",
					"DIR=$(cd \"$(dirname \"$0\")\"; pwd)\n",
					fmt.Sprintf("$DIR/%s ", app.Bin),
					"-v ",
					"-ffmpeg $DIR/../Resources/ffmpeg ",
					fmt.Sprintf("-log $DIR/../%s.log ", app.Bin),
					"-tmp $DIR/../Resources/tmp ",
				}, "")
				name := filepath.Join(macos, fmt.Sprintf("%s.sh", app.Bin))
				if err := ioutil.WriteFile(name, []byte(wrapper), 0777); err != nil {
					return errors.Wrap(err, "writing wrapper file")
				}
				return nil
			},
		},
		Task{
			Name:     "compile binary",
			Requires: []string{"compile ui"},
			Op: func() error {
				args := []string{"go", "build"}
				if app.BuildTags != "" {
					args = append(args, "-tags", app.BuildTags)
				}
				args = append(args, []string{
					"-o", filepath.Join(macos, app.Bin),
					app.Pkg,
				}...)
				cmd := exec.Command(args[0], args[1:]...)
				if out, err := cmd.CombinedOutput(); err != nil {
					return errors.Wrap(err, string(out))
				}
				return nil
			},
		},
		Task{
			Name:     "embed resources",
			Requires: []string{"compile binary"},
			Op: func() error {
				cmd := exec.Command(
					"rice",
					"append",
					"-i", app.Pkg,
					"--exec", filepath.Join(macos, app.Bin),
				)
				if out, err := cmd.CombinedOutput(); err != nil {
					return errors.Wrap(err, string(out))
				}
				return nil
			},
		},
		Task{
			Name: "create icon.icns",
			Op: func() error {
				if app.Icon != "" {
					if err := WriteIcon(app.Icon, iconf); err != nil {
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
				if app.FFmpeg == "" {
					app.FFmpeg = "https://ffmpeg.zeranoe.com/builds/macos64/static/ffmpeg-latest-macos64-static.zip"
				}
				if strings.Contains(app.FFmpeg, "https://") {
					u, err := user.Current()
					if err != nil {
						return errors.Wrap(err, "determine current user")
					}
					downloads := filepath.Join(u.HomeDir, "Downloads")
					src, err = openZip(app.FFmpeg, downloads, "bin/ffmpeg")
					if err != nil {
						return errors.Wrap(err, "downloading ffmpeg")
					}
				} else {
					srcf, err := os.Open(app.FFmpeg)
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
	return tasks.Run()
}
