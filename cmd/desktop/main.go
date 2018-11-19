package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackmordaunt/giffer"
	"github.com/pkg/errors"

	"github.com/GeertJohan/go.rice"

	"github.com/gorilla/mux"
)

var (
	host      string
	port      string
	devServer string
	verbose   bool
	headless  bool
	ffmpeg    string
	logfile   string
	logf      *os.File
	static    http.Handler // responsible for serving UI files.
)

func init() {
	flag.StringVar(&host, "host", "localhost", "host address to serve on")
	flag.StringVar(&port, "p", "8080", "port to serve on")
	flag.StringVar(&devServer, "dev-proxy", "", "proxy to forward to (eg, yarn run serve)")
	flag.BoolVar(&verbose, "v", false, "verbose mode")
	flag.BoolVar(&headless, "headless", false, "headless mode; run only the server")
	flag.StringVar(&ffmpeg, "ffmpeg", "", "custom path to ffmpeg binary")
	flag.StringVar(&logfile, "log", "", "path to log file to capture stdout")
	flag.Parse()
	if logfile != "" {
		var err error
		logf, err = os.OpenFile(logfile, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			panic(errors.Wrap(err, "opening log file"))
		}
	} else {
		logf = os.Stdout
	}
	log.SetOutput(logf)
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
				Dir:    filepath.Join(filepath.Dir(ffmpeg), "tmp/downloads"),
				FFmpeg: ffmpeg,
				Out:    logf,
			},
			FFMpeg: &giffer.FFMpeg{
				Debug: verbose,
				Use:   ffmpeg,
			},
			Store: &gifdb{
				Dir: filepath.Join(filepath.Dir(ffmpeg), "tmp/gifs"),
			},
		},
		Router:  mux.NewRouter(),
		Static:  static,
		Verbose: verbose,
		Out:     logf,
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
