// +build !chrome,!native

package main

import "log"

// Webview renders the UI through the system's default browser.
func Webview(url, title string, w, h int) error {
	b := &Browser{
		OnErr: func(err error) {
			log.Printf("browser: %v", err)
		},
		Loop: true,
		OnRestart: func() {
			log.Printf("restarting browser")
		},
	}
	return b.Run(url)
}
