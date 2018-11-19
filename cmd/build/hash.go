package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"unicode"

	"github.com/OneOfOne/xxhash"
	"github.com/pkg/errors"
)

// Hash recusively and concurrently hashes a fileEntry or directory.
func Hash(path string) (uint64, error) {
	var (
		found = make(chan fileEntry)
		wg    = &sync.WaitGroup{}
	)
	walk := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			by, err := ioutil.ReadFile(path)
			found <- fileEntry{
				Path:    path,
				Content: by,
				Err:     err,
			}
		}()
		return nil
	}
	if err := filepath.Walk(path, walk); err != nil {
		return 0, err
	}
	go func() {
		wg.Wait()
		close(found)
	}()
	var files []fileEntry
	for f := range found {
		if f.Err != nil {
			return 0, f.Err
		}
		files = append(files, f)
	}
	sort.Sort(alphabetical(files))
	hash := xxhash.New64()
	for _, f := range files {
		if _, err := hash.Write(f.Content); err != nil {
			return 0, errors.Wrap(err, "hashing")
		}
	}
	return hash.Sum64(), nil
}

type fileEntry struct {
	Path    string
	Content []byte
	Err     error
}

type alphabetical []fileEntry

func (a alphabetical) Len() int {
	return len(a)
}

func (a alphabetical) Swap(ii, jj int) {
	a[ii], a[jj] = a[jj], a[ii]
}

func (a alphabetical) Less(ii, jj int) bool {
	iRunes := []rune(a[ii].Path)
	jRunes := []rune(a[jj].Path)
	max := len(iRunes)
	if max > len(jRunes) {
		max = len(jRunes)
	}
	for idx := 0; idx < max; idx++ {
		ir := iRunes[idx]
		jr := jRunes[idx]
		lir := unicode.ToLower(ir)
		ljr := unicode.ToLower(jr)
		if lir != ljr {
			return lir < ljr
		}
		// the lowercase runes are the same, so compare the original
		if ir != jr {
			return ir < jr
		}
	}
	return false
}
