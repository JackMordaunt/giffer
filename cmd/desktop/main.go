package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jackmordaunt/giffer"

	"github.com/GeertJohan/go.rice"

	"github.com/gorilla/mux"
)

var (
	host      string
	port      string
	devServer string
	verbose   bool
	headless  bool
	static    http.Handler // responsible for serving UI files.
)

func init() {
	flag.StringVar(&host, "host", "localhost", "host address to serve on")
	flag.StringVar(&port, "p", "8080", "port to serve on")
	flag.StringVar(&devServer, "dev-proxy", "", "proxy to forward to (eg, yarn run serve)")
	flag.BoolVar(&verbose, "v", false, "verbose mode")
	flag.BoolVar(&headless, "headless", false, "headless mode; run only the server")
	flag.Parse()
	if devServer != "" {
		original := devServer
		if devServer[0] == ':' {
			devServer = fmt.Sprintf("127.0.0.1:%s", devServer[1:])
		}
		if !strings.HasPrefix(devServer, "http://") {
			devServer = fmt.Sprintf("http://%s", devServer)
		}
		t, err := url.Parse(devServer)
		if err != nil {
			log.Fatalf("proxy: %s: not a valid URL", original)
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
	if err := Webview(url, "Giffer", 800, 600); err != nil {
		log.Printf("webview: %v", err)
	}
	if err := svr.Shutdown(context.Background()); err != nil {
		log.Printf("shutting down server: %v", err)
	}
}
