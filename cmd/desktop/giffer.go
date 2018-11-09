package main

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"io"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/OneOfOne/xxhash"
	"github.com/disintegration/imaging"
	"github.com/jackmordaunt/giffer"
	"github.com/pkg/errors"
)

// Giffer wraps the giffer business logic.
type Giffer struct {
	*giffer.Downloader
	*giffer.FFMpeg
	Store GifStore
}

// GifStore contains Gif files.
type GifStore interface {
	Lookup(key string) (*RenderedGif, bool, error)
	Insert(key string, img *RenderedGif) error
}

// GififyURL downloads the video at url and creates a .gif based on the spcified
// parameters.
func (g Giffer) GififyURL(
	url string,
	start, end, fps float64,
	width, height int,
	q giffer.Quality,
) (*RenderedGif, error) {
	if g.Store == nil {
		return g.make(url, start, end, fps, width, height, q)
	}
	hasher := xxhash.New64()
	input := fmt.Sprintf("%s_%f_%f_%f_%d_%d_%d", url, start, end, fps, width, height, q)
	_, err := hasher.WriteString(input)
	if err != nil {
		return nil, errors.Wrap(err, "hashing input")
	}
	hash := fmt.Sprintf("%d", hasher.Sum64())
	img, ok, err := g.Store.Lookup(hash)
	if err != nil {
		return nil, errors.Wrap(err, "store lookup")
	}
	if ok && img != nil {
		return img, nil
	}
	img, err = g.make(url, start, end, fps, width, height, q)
	if err != nil {
		return nil, err
	}
	dup := &RenderedGif{
		Reader:   bytes.NewBuffer([]byte(img.Reader.(*bytes.Buffer).String())),
		FileName: img.FileName,
	}
	if err := g.Store.Insert(hash, dup); err != nil {
		return nil, errors.Wrap(err, "inserting gif into store")
	}
	return img, nil
}

func (g Giffer) make(
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
	img := &RenderedGif{
		Reader: buf,
		// Keep the title but replace the .mp4 extension with .gif
		FileName: sanitiseFilepath(strings.Split(filepath.Base(videofile), ".")[0] + ".gif"),
	}
	return img, nil
}

// RenderedGif wraps the gif data with some metadata.
type RenderedGif struct {
	io.Reader
	// FileName is <title>.<ext>
	FileName string
}
