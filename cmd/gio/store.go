package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/pkg/errors"
)

// Gif images are stored as-is on disk with a corresponding json file that
// contains the metadata. Why did I use goroutines to parellelise writing just
// two files? Don't ask me that, man.
type gifdb struct {
	Dir  string
	init sync.Once
}

// Lookup loads the rendered gif from disk.
func (db *gifdb) Lookup(key string) (*RenderedGif, bool, error) {
	var (
		info os.FileInfo
		err  error
	)
	db.init.Do(func() {
		err = os.MkdirAll(db.Dir, 0755)
	})
	if err != nil && !os.IsExist(err) {
		return nil, false, errors.Wrap(err, "initialising")
	}
	meta := filepath.Join(db.Dir, key+".json")
	info, err = os.Stat(meta)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.IsDir() {
		return nil, false, fmt.Errorf("key leads to a directory, not a json file")
	}
	img := filepath.Join(db.Dir, key+".gif")
	info, err = os.Stat(img)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if info.IsDir() {
		return nil, false, fmt.Errorf("key leads to a directory, not a gif file")
	}
	var (
		failed = make(chan error)
		done   = make(chan interface{})
		wg     = sync.WaitGroup{}
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		if name, err := func() (string, error) {
			metaf, err := os.Open(meta)
			if err != nil {
				return "", errors.Wrap(err, "opening metadata file")
			}
			defer metaf.Close()
			type metadata struct {
				FileName string `json:"filename"`
			}
			var md metadata
			if err := json.NewDecoder(metaf).Decode(&md); err != nil {
				return "", errors.Wrap(err, "decoding metadata")
			}
			return md.FileName, nil
		}(); err != nil {
			failed <- err
		} else {
			done <- name
		}
	}()
	go func() {
		defer wg.Done()
		if buf, err := func() (*bytes.Buffer, error) {
			buf := bytes.NewBuffer(nil)
			file, err := os.Open(img)
			if err != nil {
				return nil, errors.Wrap(err, "opening gif file")
			}
			defer file.Close()
			if _, err := io.Copy(buf, file); err != nil {
				return nil, errors.Wrap(err, "reading gif file")
			}
			return buf, nil
		}(); err != nil {
			failed <- err
		} else {
			done <- buf
		}
	}()
	go func() {
		wg.Wait()
		close(failed)
	}()
	var failure error
	go func() {
		for err := range failed {
			failure = multierror.Append(failure, err)
		}
		close(done)
	}()
	r := &RenderedGif{}
	for v := range done {
		switch v := v.(type) {
		case string:
			r.FileName = v
		case *bytes.Buffer:
			r.Reader = v
		}
	}
	if failure != nil {
		return nil, false, failure
	}
	return r, true, nil
}

// Insert stores the rendered gif on disk.
func (db *gifdb) Insert(key string, img *RenderedGif) (err error) {
	db.init.Do(func() {
		err = os.MkdirAll(db.Dir, 0755)
	})
	if err != nil && !os.IsExist(err) {
		return errors.Wrap(err, "initialising")
	}
	var (
		failed = make(chan error)
		wg     = sync.WaitGroup{}
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := func() error {
			imgpath := filepath.Join(db.Dir, key+".gif")
			imgf, err := os.Create(imgpath)
			if err != nil {
				return errors.Wrap(err, "creating gif file")
			}
			defer imgf.Close()
			if _, err := io.Copy(imgf, img); err != nil {
				return errors.Wrap(err, "persisting gif to disk")
			}
			return nil
		}(); err != nil {
			failed <- err
		}
	}()
	go func() {
		defer wg.Done()
		if err := func() error {
			meta := filepath.Join(db.Dir, key+".json")
			metaf, err := os.Create(meta)
			if err != nil {
				return errors.Wrap(err, "creating metadata file")
			}
			defer metaf.Close()
			type metadata struct {
				FileName string `json:"filename"`
			}
			if err := json.NewEncoder(metaf).Encode(metadata{
				FileName: img.FileName,
			}); err != nil {
				return errors.Wrap(err, "writing to metadata file")
			}
			return nil
		}(); err != nil {
			failed <- err
		}
	}()
	go func() {
		wg.Wait()
		close(failed)
	}()
	for failure := range failed {
		err = multierror.Append(err, failure)
	}
	return err
}
