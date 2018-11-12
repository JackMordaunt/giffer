package giffer

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"os/exec"
)

// FFMpeg wraps the ffmpeg binary.
type FFMpeg struct {
	Debug bool
}

// Convert a video into that of the specified encoding and format between start
// and end.
// If end is zero we convert from start until the end of the video.
func (f FFMpeg) Convert(
	video string,
	start, end float64,
	fps float64,
	width, height int,
	encoding, format string,
) (*bytes.Buffer, error) {
	var out bytes.Buffer
	args := []string{
		"-ss", fmt.Sprintf("%4f", start),
	}
	if end > 0 && end-start > 0 {
		args = append(args, "-t", fmt.Sprintf("%2f", end-start))
	}
	args = append(args, "-i", video)
	if width > 0 || height > 0 || fps > 0 {
		var vfargs []string
		if fps > 0 {
			vfargs = append(vfargs, fmt.Sprintf("fps=%2f", fps))
		}
		if width > 0 || height > 0 {
			if width <= 0 {
				width = -1
			}
			if height <= 0 {
				height = -1
			}
			vfargs = append(vfargs, fmt.Sprintf("scale=%d:%d", width, height))
		}
		args = append(args, "-vf", strings.Join(vfargs, ","))
	}
	args = append(args,
		"-c", "copy",
		"-c:v", encoding,
		"-f", format, "-",
	)
	if f.Debug {
		log.Printf("ffmpeg %s", strings.Join(args, " "))
	}
	cmd := CmdPipe{
		Out:   &out,
		Debug: f.Debug,
		Stack: []*exec.Cmd{
			exec.Command("ffmpeg", args...),
		},
	}
	return &out, cmd.Run()
}
