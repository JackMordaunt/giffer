package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/OneOfOne/xxhash"
	"github.com/jackmordaunt/giffer"
	"github.com/pkg/errors"
)

// Giffer wraps the giffer business logic.
type Giffer struct {
	*giffer.Downloader
	*giffer.Engine
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
	width, height, fuzz int,
	q giffer.Quality,
) (*RenderedGif, error) {
	if g.Store == nil {
		return g.make(url, start, end, fps, width, height, fuzz, q)
	}
	key, err := hash(fmt.Sprintf("%s_%f_%f_%f_%d_%d_%d", url, start, end, fps, width, height, q))
	if err != nil {
		return nil, err
	}
	img, ok, err := g.Store.Lookup(key)
	if err != nil {
		return nil, errors.Wrap(err, "store lookup")
	}
	if ok && img != nil {
		return img, nil
	}
	img, err = g.make(url, start, end, fps, width, height, fuzz, q)
	if err != nil {
		return nil, err
	}
	dup := &RenderedGif{
		Reader:   bytes.NewBuffer([]byte(img.Reader.(*bytes.Buffer).String())),
		FileName: img.FileName,
	}
	if err := g.Store.Insert(key, dup); err != nil {
		return nil, errors.Wrap(err, "inserting gif into store")
	}
	return img, nil
}

func (g Giffer) make(
	url string,
	start, end, fps float64,
	width, height, fuzz int,
	q giffer.Quality,
) (*RenderedGif, error) {
	video, err := g.Download(url, start, end, q)
	if err != nil {
		return nil, errors.Wrap(err, "downloading")
	}
	gif, err := g.Transcode(video, start, end, width, height, fps)
	if err != nil {
		return nil, errors.Wrap(err, "transcoding video to gif")
	}
	if err := g.Crush(gif, fuzz); err != nil {
		return nil, errors.Wrap(err, "optimising gif image")
	}
	defer g.Clean()
	gifdata, err := ioutil.ReadFile(gif)
	if err != nil {
		return nil, errors.Wrap(err, "buffering gif")
	}
	img := &RenderedGif{
		Reader:   bytes.NewBuffer(gifdata),
		FileName: sanitiseFilepath(strings.Split(filepath.Base(video), ".")[0] + ".gif"),
	}
	return img, nil
}

// RenderedGif wraps the gif data with some metadata.
type RenderedGif struct {
	io.Reader
	// FileName is <title>.<ext>
	FileName string
}

func hash(input string) (string, error) {
	hasher := xxhash.New64()
	_, err := hasher.WriteString(input)
	if err != nil {
		return "", errors.Wrap(err, "hashing input")
	}
	h := fmt.Sprintf("%d", hasher.Sum64())
	return h, nil
}
