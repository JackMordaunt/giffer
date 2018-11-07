package main

import (
	"os"
	"net/http/httputil"
	"net/url"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/gif"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/disintegration/imaging"
	"github.com/jackmordaunt/giffer"

	"github.com/GeertJohan/go.rice"
	"github.com/pkg/errors"

	"github.com/gorilla/mux"

	"github.com/zserge/webview"
)

var (
	port string
	devServer string
	static http.Handler // responsible for serving UI files. 
)

func init() {
	flag.StringVar(&port, "p", "8080", "port to serve on")
	flag.StringVar(&devServer, "dev-proxy", "", "proxy to forward to (eg, yarn run serve)")
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
				LeaveMess: true,
			},
		},
		Router: mux.NewRouter(),
		Static: static,
	}
	svr := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      ui,
		// Can't use these time-outs yet since the gifify route doesn't
		// return immediately (downloads 400MB, etc...).
		// WriteTimeout: 15 * time.Second,
		// ReadTimeout:  15 * time.Second,
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
	App    *Giffer
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
	log := Log{
		Logger: log.New(os.Stdout, "", 0),
		ShowBody: true,
	}
	ui.Router.Handle("/gifify", Wrap(ui.gifify(), log))
	// The router typically treats "/" as a unique router We have to tell
	// it otherwise in order to handle file paths correctly.
	ui.Router.Handle("/{path:.*}", Wrap(ui.Static, log))
}

func (ui *UI) gifify() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		type request struct {
			URL    string  `json:"url,omitempty"`
			Start  float64 `json:"start,omitempty"`
			End    float64 `json:"end,omitempty"`
			FPS    float64 `json:"fps,omitempty"`
			Width  int     `json:"width,omitempty"`
			Height int     `json:"height,omitempty"`
			Output string  `json:"output,omitempty"`
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			ui.error(w, errors.Wrap(err, "decoding json request"))
			return
		}
		img, err := ui.App.GififyURL(
			req.URL,
			req.Start,
			req.End,
			req.FPS,
			req.Width,
			req.Height)
		if err != nil {
			ui.error(w, err)
			return
		}
		w.Header().Set("Content-Type", http.DetectContentType(img.Bytes()))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, img.FileName))
		if _, err := io.Copy(w, img); err != nil {
			ui.error(w, errors.Wrap(err, "writing response"))
			return
		}
	}
}

func (ui *UI) error(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// Giffer wraps the giffer business logic.
type Giffer struct {
	*giffer.Downloader
	*giffer.FFMpeg
}

// GififyURL downloads the video at url and creates a .gif based on the spcified
// parameters.
func (g Giffer) GififyURL(
	url string,
	start, end, fps float64,
	width, height int,
) (*RenderedGif, error) {
	videofile, err := g.Download(url)
	if err != nil {
		return nil, errors.Wrap(err, "downloading")
	}
	frames, err := g.Extract(videofile, start, end, fps)
	if err != nil {
		return nil, errors.Wrap(err, "extracting frames")
	}
	type processed struct {
		Img   *image.Paletted
		Index int
	}
	images := make(chan processed)
	wg := &sync.WaitGroup{}
	wg.Add(len(frames))
	for ii, frame := range frames {
		ii := ii
		frame := frame
		go func() {
			defer wg.Done()
			if width != 0 || height != 0 {
				frame = imaging.Resize(frame, width, height, imaging.Box)
			}
			buf := bytes.Buffer{}
			if err := gif.Encode(&buf, frame, nil); err != nil {
				// errors.Wrap(err, "encoding gif")
				return
			}
			tmpimg, err := gif.Decode(&buf)
			if err != nil {
				// errors.Wrap(err, "decoding gif")
				return
			}
			images <- processed{
				Img:   tmpimg.(*image.Paletted),
				Index: ii,
			}
		}()
	}
	go func() {
		wg.Wait()
		close(images)
	}()
	paletted := make([]*image.Paletted, len(frames))
	for frame := range images {
		paletted[frame.Index] = frame.Img
	}
	delays := make([]int, len(frames))
	delay := int(100 / fps)
	for ii := range delays {
		delays[ii] = delay
	}
	buf := bytes.NewBuffer(nil)
	cfg := &gif.GIF{
		Image:     paletted,
		Delay:     delays,
		LoopCount: 0,
	}
	if err := gif.EncodeAll(buf, cfg); err != nil {
		return nil, errors.Wrap(err, "encoding animated gif")
	}
	r := &RenderedGif{
		Buffer: buf,
		// Keep the title but replace the .mp4 extension with .gif
		FileName: strings.Split(filepath.Base(videofile), ".")[0] + ".gif",
	}
	return r, nil
}

// RenderedGif wraps the gif data with some metadata.
type RenderedGif struct {
	*bytes.Buffer
	// FileName is <title>.<ext>
	FileName string
}

// Proxy incoming requests to target.
type Proxy struct {
	Target *url.URL
}

func (p Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	proxy := httputil.NewSingleHostReverseProxy(p.Target)
	r.URL.Host = p.Target.Host
	r.URL.Scheme = p.Target.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Host = p.Target.Host
	proxy.ServeHTTP(w, r)
}

// Log creates middleware that logs requests.
// If out is nil it is assumed that no logs are desired.
type Log struct {
	Logger *log.Logger
	ShowBody bool
}

// Wrap the handler with logging middleware.
func (l Log) Wrap(next http.Handler) http.Handler {
	if l.Logger == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg = &strings.Builder{}
		fmt.Fprintf(msg, "URL:         %s\n", r.URL)
		fmt.Fprintf(msg, "Method:      %s\n", r.Method)
		fmt.Fprintf(msg, "Headers:     http.Header{\n")
		for h, v := range r.Header {
			fmt.Fprintf(msg, "\t%q: {", h)
			for ii := range v {
				fmt.Fprintf(msg, "%s", v[ii])
				if ii != len(v)-1 {
					fmt.Fprint(msg, ",")
				}
			}
			fmt.Fprintf(msg, "},\n")
		}
		fmt.Fprintf(msg, "}\n")
		if r.Response != nil {
			fmt.Fprintf(msg, "Status Code: %d\n", r.Response.StatusCode)
		}
		if l.ShowBody {
			by, _ := readUntil(r.Body, 1000*64) // 64Kb
			if len(by) > 0 {
				fmt.Fprintf(msg, " Body:       %s\n", string(by))
			}
		}
		fmt.Fprintf(msg, "\n")
		l.Logger.Printf(msg.String())
		next.ServeHTTP(w, r)
	})
}

func readUntil(r io.Reader, until int64) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	if _, err := io.CopyN(buf, r, until); err != nil && err != io.EOF {
		return buf.Bytes(), err
	}
	return buf.Bytes(), nil
}

// Wrapper wraps an http.Handler to provide things like middleware.
type Wrapper interface {
	Wrap(http.Handler) http.Handler
}

// Wrap the handler with the provided wrappers.
func Wrap(h http.Handler, wrappers ...Wrapper) http.Handler {
	for _, w := range wrappers {
		h = w.Wrap(h)
	}
	return h
}