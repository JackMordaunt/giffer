package main

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/OneOfOne/xxhash"
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
	img, err = g.make(url, start, end, fps, width, height, q)
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
	width, height int,
	q giffer.Quality,
) (*RenderedGif, error) {
	video, err := g.Download(url, start, end, q)
	if err != nil {
		return nil, errors.Wrap(err, "downloading")
	}
	gif, err := g.Convert(video, start, end, fps, width, height, "gif", "gif")
	img := &RenderedGif{
		Reader:   gif,
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
