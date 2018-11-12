package main

import (
	"flag"
	"io"
	"log"
	"os"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/jackmordaunt/giffer"
)

var (
	videofile string
	start     float64
	end       float64
	dest      string
	fps       float64
	width     int
	height    int
	url       string
	debug     bool
)

func main() {
	flag.StringVar(&videofile, "v", "", "path to video file to gifify")
	flag.StringVar(&url, "url", "", "url to video file to gifenate")
	flag.Float64Var(&start, "s", 0.0, "time in seconds to start the gif")
	flag.Float64Var(&end, "e", 0.0, "time in seconds to end the gif")
	flag.StringVar(&dest, "dest", "movie.gif", "a destination filename for the animated gif")
	flag.IntVar(&width, "width", 0, "width in pixels of the output frames")
	flag.IntVar(&height, "height", 0, "height in pixels of the output frames")
	flag.Float64Var(&fps, "fps", 24, "frames per second")
	flag.BoolVar(&debug, "debug", false, "debug mode")
	flag.Parse()
	if terminal.IsTerminal(int(os.Stdin.Fd())) {
		tmp, err := os.Create("tmp")
		if err != nil {
			log.Fatalf("creating temporary file: %v", err)
		}
		defer tmp.Close()
		defer os.Remove("tmp")
		if _, err := io.Copy(tmp, os.Stdin); err != nil {
			log.Fatalf("writing temporary file: %v", err)
		}
		tmp.Close()
		videofile = "tmp"
	} else if url != "" {
		dl := giffer.Downloader{
			Dir: "./tmp/dl",
		}
		downloaded, err := dl.Download(url, giffer.Medium)
		if err != nil {
			log.Fatalf("downloading: %v", err)
		}
		videofile = downloaded
	}
	ffmpeg := giffer.FFMpeg{
		Debug: debug,
	}
	gif, err := ffmpeg.Convert(videofile, start, end, fps, width, height, "gif", "gif")
	if err != nil {
		log.Fatalf("converting to gif: %v", err)
	}
	var out io.WriteCloser
	if terminal.IsTerminal(int(os.Stdout.Fd())) {
		out = os.Stdout
	} else {
		out, err = os.Open(dest)
		if err != nil {
			log.Fatalf("opening destination file: %v", err)
		}
	}
	defer out.Close()
	if _, err := io.Copy(out, gif); err != nil {
		log.Fatalf("writing gif to file: %v", err)
	}
}
