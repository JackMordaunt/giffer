package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jackmordaunt/giffer"

	"github.com/GeertJohan/go.rice"

	"github.com/gorilla/mux"

	"github.com/zserge/webview"
)

var (
	host      string
	port      string
	devServer string
	verbose   bool
	headless  bool
	browser   bool
	static    http.Handler // responsible for serving UI files.
)

func init() {
	flag.StringVar(&host, "host", "localhost", "host address to serve on")
	flag.StringVar(&port, "p", "8080", "port to serve on")
	flag.StringVar(&devServer, "dev-proxy", "", "proxy to forward to (eg, yarn run serve)")
	flag.BoolVar(&verbose, "v", false, "verbose mode")
	flag.BoolVar(&headless, "headless", false, "headless mode; run only the server")
	flag.BoolVar(&browser, "browser", false, "open in default browser instead of webview; overriden by [headless]")
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
				Debug: verbose,
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
		Addr:         fmt.Sprintf("%s:%s", host, port),
		Handler:      ui,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	backend := sync.WaitGroup{}
	defer backend.Wait()
	backend.Add(1)
	go func() {
		defer backend.Done()
		if err := svr.ListenAndServe(); err != nil {
			log.Fatalf("ui server: %v", err)
		}
	}()
	if headless {
		return
	}
	url := fmt.Sprintf("http://%s:%s", host, port)
	if browser {
		b := &Browser{
			OnErr: func(err error) {
				log.Printf("browser: %v", err)
			},
			Loop: true,
			OnRestart: func() {
				log.Printf("restarting browser")
			},
		}
		if err := b.Open(url); err != nil {
			log.Fatalf("opening browser: %v", err)
		}
	} else {
		view := webview.New(webview.Settings{
			Title:     "Giffer",
			URL:       url,
			Width:     800,
			Height:    600,
			Resizable: true,
			Debug:     true,
		})
		view.Run()
	}
}
