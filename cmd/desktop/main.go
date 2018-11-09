package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/jackmordaunt/giffer"

	"github.com/GeertJohan/go.rice"

	"github.com/gorilla/mux"

	"github.com/zserge/webview"
)

var (
	port      string
	devServer string
	verbose   bool
	static    http.Handler // responsible for serving UI files.
)

func init() {
	flag.StringVar(&port, "p", "8080", "port to serve on")
	flag.StringVar(&devServer, "dev-proxy", "", "proxy to forward to (eg, yarn run serve)")
	flag.BoolVar(&verbose, "v", false, "verbose mode")
	flag.Parse()
	if devServer != "" {
		t, err := url.Parse(devServer)
		if err != nil {
			log.Fatalf("proxy: %s: not a valid URL", devServer)
		}
		static = &Proxy{Target: t}
	} else {
		static = http.FileServer(rice.MustFindBox("ui/dist").HTTPBox())
	}
}

func main() {
	ui := &UI{
		App: &Giffer{
			Downloader: &giffer.Downloader{
				Dir: "tmp/download",
			},
			FFMpeg: &giffer.FFMpeg{
				Dir: "tmp/ffmpeg",
			},
			Store: &gifdb{
				Dir: "tmp/gifs",
			},
		},
		Router:  mux.NewRouter(),
		Static:  static,
		Verbose: verbose,
	}
	svr := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      ui,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	go func() {
		if err := svr.ListenAndServe(); err != nil {
			log.Fatalf("ui server: %v", err)
		}
	}()
	view := webview.New(webview.Settings{
		Title:     "WebGen",
		URL:       fmt.Sprintf("http://localhost:%s", port),
		Width:     800,
		Height:    600,
		Resizable: true,
		Debug:     true,
	})
	view.Run()
}
