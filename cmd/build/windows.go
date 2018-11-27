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
	"time"

	"github.com/pkg/errors"
)

// Windows builds Giffer as a .exe for Windows.
type Windows struct {
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

// Build executes the build and writes output to dist.
func (exe *Windows) Build(dist string) error {
	tasks := FanOut([]Task{
		Task{
			Name: "compile ui",
			Op: func() error {
				ui, err := filepath.Abs(exe.UI)
				if err != nil {
					return errors.Wrap(err, "resolving absolute path")
				}
				exe.UI = filepath.Clean(ui)
				var (
					src          = filepath.Join(exe.UI, "src")
					checksumFile = filepath.Join(exe.UI, "checksum")
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
				cmd.Dir = exe.UI
				if out, err := cmd.CombinedOutput(); err != nil {
					return errors.Wrap(err, string(out))
				}
				buf := make([]byte, 64)
				binary.LittleEndian.PutUint64(buf, h)
				return ioutil.WriteFile(checksumFile, buf, 0644)
			},
		},
		Task{
			Name: "compile binary",
			Op: func() error {
				ldflags := []string{
					// The "X" follows the variable naming convention in webview_windows.go
					fmt.Sprintf("%s.%sX=%s", exe.Pkg, "ffmpeg", "ffmpeg.exe"),
					fmt.Sprintf("%s.%sX=%s", exe.Pkg, "logfile", "giffer.log"),
					fmt.Sprintf("%s.%Xs=%s", exe.Pkg, "tmp", "tmp"),
				}
				args := []string{
					"go",
					"build",
					"-ldflags",
					fmt.Sprintf("%q", strings.Join(ldflags, "-X ")),
				}
				if exe.BuildTags != "" {
					args = append(args, "-tags", exe.BuildTags)
				}
				args = append(args, []string{
					"-o",
					filepath.Join(dist, exe.Bin),
					exe.Pkg,
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
			Requires: []string{"compile binary", "compile ui"},
			Op: func() error {
				time.Sleep(time.Millisecond * 300)
				cmd := exec.Command(
					"rice",
					"append",
					"-i", exe.Pkg,
					"--exec", filepath.Join(dist, exe.Bin),
				)
				if out, err := cmd.CombinedOutput(); err != nil {
					return errors.Wrap(err, string(out))
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
				if exe.FFmpeg == "" {
					exe.FFmpeg = "https://ffmpeg.zeranoe.com/builds/win64/static/ffmpeg-20181126-90ac0e5-win64-static.zip"
				}
				if strings.Contains(exe.FFmpeg, "https://") {
					u, err := user.Current()
					if err != nil {
						return errors.Wrap(err, "determine current user")
					}
					downloads := filepath.Join(u.HomeDir, "Downloads")
					src, err = openZip(exe.FFmpeg, downloads, "bin/ffmpeg")
					if err != nil {
						return errors.Wrap(err, "downloading ffmpeg")
					}
				} else {
					srcf, err := os.Open(exe.FFmpeg)
					if err != nil {
						return errors.Wrap(err, "opening ffmpeg file")
					}
					defer srcf.Close()
					src = srcf
				}
				destf, err := os.OpenFile(
					filepath.Join(dist, "ffmpeg.exe"),
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
