// +build native

package main

import "github.com/zserge/webview"

// Webview (+native) renders the UI via a native webview window.
// Requires cgo as a compile time dependency.
func Webview(url, title string, w, h int) error {
	view := webview.New(webview.Settings{
		Title:     title,
		URL:       url,
		Width:     w,
		Height:    h,
		Resizable: true,
		Debug:     true,
	})
	view.Run()
	return nil
}
