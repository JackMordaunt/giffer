package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/OneOfOne/xxhash"
	"github.com/jackmordaunt/giffer"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"github.com/pkg/errors"
)

// UI serves the user interface over http.
type UI struct {
	App     *Giffer
	Router  *mux.Router
	Static  http.Handler
	Verbose bool
	Out io.Writer

	gifmap map[string]http.Handler
	init   sync.Once
}

func (ui *UI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ui.init.Do(func() {
		if ui.gifmap == nil {
			ui.gifmap = make(map[string]http.Handler)
		}
		ui.routes()
	})
	ui.Router.ServeHTTP(w, r)
}

func (ui *UI) routes() {
	if ui.Verbose {
		var out io.Writer
		if ui.Out != nil {
			out = ui.Out
		} else {
			out = os.Stdout
		}
		log := Log{
			Logger:   log.New(LogWriteHeaderErrors{Out: out}, "", 0),
			ShowBody: true,
		}
		ui.Router.Use(log.Middleware)
	}
	ui.Router.Handle("/gifify", ui.gifify())
	ui.Router.Handle("/gifs/{key}", ui.gifs())
	ui.Router.Handle("/gifs/{key}/info", ui.gifs())
	ui.Router.Handle("/{path:.*}", ui.Static)
}

func (ui *UI) gifify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		type request struct {
			URL     string  `json:"url,omitempty"`
			Start   float64 `json:"start,omitempty"`
			End     float64 `json:"end,omitempty"`
			FPS     float64 `json:"fps,omitempty"`
			Width   int     `json:"width,omitempty"`
			Height  int     `json:"height,omitempty"`
			Output  string  `json:"output,omitempty"`
			Quality int     `json:"quality,omitempty"`
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpError(w, errors.Wrap(err, "decoding json request"))
			return
		}
		by, err := json.Marshal(req)
		if err != nil {
			httpError(w, fmt.Errorf("marshalling json for hash"))
			return
		}
		h := xxhash.New64()
		if _, err := h.Write(by); err != nil {
			httpError(w, fmt.Errorf("writing to hash object"))
			return
		}
		g := &Gif{
			Upgrader: &websocket.Upgrader{
				ReadBufferSize:  1024,
				WriteBufferSize: 1024,
			},
		}
		g.Init()
		go g.Process(func() (*RenderedGif, error) {
			return ui.App.GififyURL(
				req.URL,
				req.Start,
				req.End,
				req.FPS,
				req.Width,
				req.Height,
				giffer.Quality(req.Quality))
		})
		key := fmt.Sprintf("%d", h.Sum64())
		ui.gifmap[key] = g
		type response struct {
			File string `json:"file"`
			Info string `json:"info"`
		}
		// FIXME(jfm): Should these endpoints be typed, instead of
		// 	relying on assumptions about the routing?
		writeJSON(w, response{
			File: fmt.Sprintf("/gifs/%s", key),
			Info: fmt.Sprintf("/gifs/%s/info", key),
		})
	}
}

func (ui *UI) gifs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := mux.Vars(r)["key"]
		h, ok := ui.gifmap[key]
		if !ok {
			writeJSON(w, fmt.Errorf("no gif exists for key %q", key))
			return
		}
		h.ServeHTTP(w, r)
	}
}

// Gif handles the serving of a gif file and a websocket endpoint to push out
// "gif is ready" messages.
//
// 3 parts:
// 1. A closure that produces the rendered gif and an error (to decouple how we
//	get a hold of the gif from the serving of it).
// 2. A websocket endpoint that pushes a single message to each connection when
// 	the gif is ready to download. If the gif is ready before the connection
//	comes in, the connection receives the message immediately. Otherwise
// 	the connection is stored in a map until the gif is ready.
// 3. A static file endpoint that serves the gif image as an attachment. If the
//	gif isn't ready to be downloaded, an appropriate message is returned.
type Gif struct {
	Upgrader *websocket.Upgrader
	Out io.Writer

	file        *RenderedGif
	subs        map[*websocket.Conn]struct{}
	done        chan *RenderedGif
	failed      chan error
	connections chan *websocket.Conn
}

// Init the Gif server.
func (g *Gif) Init() {
	g.subs = make(map[*websocket.Conn]struct{})
	g.done = make(chan *RenderedGif)
	g.failed = make(chan error)
	g.connections = make(chan *websocket.Conn)
	go g.run()
}

// Process runs the specified function and sends a websocket message when it
// completes.
func (g *Gif) Process(fn func() (*RenderedGif, error)) {
	img, err := fn()
	if err != nil {
		g.failed <- err
	} else {
		g.done <- img
	}
}

func (g *Gif) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "info") {
		g.subscribe(w, r)
	} else {
		g.serveFile(w, r)
	}
}

func (g *Gif) subscribe(w http.ResponseWriter, r *http.Request) {
	c, err := g.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.logf("upgrading websocket: %v", err)
		return
	}
	g.connections <- c
}

func (g *Gif) serveFile(w http.ResponseWriter, r *http.Request) {
	if g.file == nil {
		writeJSON(w, fmt.Errorf("gif not ready"))
		return
	}
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, g.file.FileName))
	if _, err := io.Copy(w, g.file); err != nil {
		httpError(w, errors.Wrap(err, "writing gif to response body"))
		return
	}
}

func (g *Gif) run() {
	type msg struct {
		Err string `json:"error,omitempty"`
	}
	for {
		select {
		case img := <-g.done:
			g.file = img
			for s := range g.subs {
				if err := s.WriteJSON(msg{}); err != nil {
					g.logf("writing json to websocket: %v", err)
				}
				if err := s.Close(); err != nil {
					g.logf("closing websocket: %v", err)
				}
				delete(g.subs, s)
			}
		case err := <-g.failed:
			for s := range g.subs {
				if err := s.WriteJSON(msg{Err: err.Error()}); err != nil {
					g.logf("writing json to websocket: %v", err)
				}
				if err := s.Close(); err != nil {
					g.logf("closing websocket: %v", err)
				}
				delete(g.subs, s)
			}
		case conn := <-g.connections:
			if g.file != nil {
				if err := conn.WriteJSON(msg{}); err != nil {
					g.logf("writing json to websocket: %v", err)
				}
				if err := conn.Close(); err != nil {
					g.logf("closing websocket: %v", err)
				}
			} else {
				g.subs[conn] = struct{}{}
			}
		}
	}
}

func (g *Gif) logf(f string, v ...interface{}) (int, error) {
	if g.Out == nil {
		return 0, nil
	}
	return fmt.Fprintf(g.Out, f, v...)
}