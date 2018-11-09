package main

import (
	"bytes"
	"flag"
	"image"
	"image/gif"
	"log"
	"os"
	"sync"

	"github.com/disintegration/imaging"

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
	flag.Parse()
	if url != "" {
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
		Dir: "./tmp/ffmpeg",
	}
	frames, err := ffmpeg.Extract(videofile, start, end, fps)
	if err != nil {
		log.Fatalf("extracting frames: %v", err)
	}
	type processed struct {
		Img   *image.Paletted
		Index int
	}
	images := make(chan processed)
	wg := &sync.WaitGroup{}
	wg.Add(len(frames))
	for ii, frame := range frames {
		ii := ii
		frame := frame
		go func() {
			defer wg.Done()
			if width != 0 || height != 0 {
				frame = imaging.Resize(frame, width, height, imaging.Box)
			}
			buf := bytes.Buffer{}
			if err := gif.Encode(&buf, frame, nil); err != nil {
				log.Printf("encoding gif: %v", err)
				return
			}
			tmpimg, err := gif.Decode(&buf)
			if err != nil {
				log.Printf("decoding gif: %v", err)
				return
			}
			images <- processed{
				Img:   tmpimg.(*image.Paletted),
				Index: ii,
			}
		}()
	}
	go func() {
		wg.Wait()
		close(images)
	}()
	paletted := make([]*image.Paletted, len(frames))
	for frame := range images {
		paletted[frame.Index] = frame.Img
	}
	delays := make([]int, len(frames))
	delay := int(100 / fps)
	for ii := range delays {
		delays[ii] = delay
	}
	opfile, err := os.Create(dest)
	if err != nil {
		log.Fatalf("creating output file %s: %v", dest, err)
	}
	defer opfile.Close()
	g := &gif.GIF{
		Image:     paletted,
		Delay:     delays,
		LoopCount: 0,
	}
	if err := gif.EncodeAll(opfile, g); err != nil {
		log.Printf("encoding animated gif: %v", err)
	}
}
