package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/GeertJohan/go.rice"

	"github.com/gorilla/mux"

	"github.com/zserge/webview"
)

var (
	port string
)

func init() {
	flag.StringVar(&port, "p", "8080", "port to serve on")
	flag.Parse()
}

func main() {
	ui := &UI{
		Router: mux.NewRouter(),
		Static: http.FileServer(rice.MustFindBox("ui").HTTPBox()),
	}
	svr := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: ui,
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

// UI serves the user interface over http.
// This UI is extremely simple, consisting of exactly one page, as such we refer
// to that markup simply as "Template".
type UI struct {
	Router *mux.Router
	Static http.Handler
	init   sync.Once
}

func (ui *UI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ui.init.Do(func() {
		ui.routes()
	})
	ui.Router.ServeHTTP(w, r)
}

func (ui *UI) routes() {
	ui.Router.Handle("/", ui.serveHTML())
}

func (ui *UI) serveHTML() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ui.Static.ServeHTTP(w, r)
	}
}

func (ui *UI) error(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
