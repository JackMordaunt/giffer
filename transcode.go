package giffer

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"os/exec"
)

// Transcoder converts video files to Gif images by wrappping FFmpeg.
type Transcoder struct {
	// FFmpeg is a path to an FFmpeg binary.
	// If empty, system path is used.
	FFmpeg string
	// Debug logs the FFmpeg command.
	Debug bool
	Out   io.Writer
}

// Convert a video into that of the specified encoding and format between start
// and end.
// If end is zero we convert from start until the end of the video.
func (t Transcoder) Convert(
	video string,
	fps float64,
	width, height int,
	encoding, format string,
) (*bytes.Buffer, error) {
	var (
		out  bytes.Buffer
		args []string
		bin  = "ffmpeg"
	)
	if t.FFmpeg != "" {
		bin = t.FFmpeg
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
	if t.Debug {
		t.logf("%s %s", bin, strings.Join(args, " "))
	}
	cmd := CmdPipe{
		Out:   &out,
		Debug: t.Debug,
		Stack: []*exec.Cmd{
			exec.Command(bin, args...),
		},
	}
	return &out, cmd.Run()
}

func (t Transcoder) logf(f string, v ...interface{}) {
	if !t.Debug || t.Out == nil {
		return
	}
	fmt.Fprintf(t.Out, f, v...)
}
