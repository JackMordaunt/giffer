package main

import (
	"bytes"
	"image"
	"image/gif"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/jackmordaunt/giffer"
	"github.com/pkg/errors"
)

// Giffer wraps the giffer business logic.
type Giffer struct {
	*giffer.Downloader
	*giffer.FFMpeg
}

// GififyURL downloads the video at url and creates a .gif based on the spcified
// parameters.
func (g Giffer) GififyURL(
	url string,
	start, end, fps float64,
	width, height int,
	q giffer.Quality,
) (*RenderedGif, error) {
	videofile, err := g.Download(url, q)
	if err != nil {
		return nil, errors.Wrap(err, "downloading")
	}
	frames, err := g.Extract(videofile, start, end, fps)
	if err != nil {
		return nil, errors.Wrap(err, "extracting frames")
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
	buf := bytes.NewBuffer(nil)
	cfg := &gif.GIF{
		Image:     paletted,
		Delay:     delays,
		LoopCount: 0,
	}
	if err := gif.EncodeAll(buf, cfg); err != nil {
		return nil, errors.Wrap(err, "encoding animated gif")
	}
	r := &RenderedGif{
		Buffer: buf,
		// Keep the title but replace the .mp4 extension with .gif
		FileName: sanitiseFilepath(strings.Split(filepath.Base(videofile), ".")[0] + ".gif"),
	}
	return r, nil
}

// RenderedGif wraps the gif data with some metadata.
type RenderedGif struct {
	*bytes.Buffer
	// FileName is <title>.<ext>
	FileName string
}
