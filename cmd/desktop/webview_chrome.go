// +build chrome

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
	"github.com/zserge/lorca"
)

// Webview (+chrome) renders the UI via a chrome window.
// Requires Chrome to be installed.
func Webview(url, title string, w, h int) error {
	b, err := lorca.New(url, "", 800, 600)
	if err != nil {
		return errors.Wrap(err, "opening chrome")
	}
	defer b.Close()
	sigc := make(chan os.Signal)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigc:
	case <-b.Done():
	}
	return nil
}
