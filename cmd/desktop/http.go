package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime/debug"
	"strings"

	"github.com/pkg/errors"
)

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
	Logger   *log.Logger
	ShowBody bool
}

// Middleware the handler with logging middleware.
func (l Log) Middleware(next http.Handler) http.Handler {
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
			by, _ := readUntil(r.Body, 1000*32) // 32KB
			if len(by) > 0 {
				fmt.Fprintf(msg, " Body:       %s\n", string(by))
			}
			r.Body = ProxyCloser{
				Reader: io.MultiReader(bytes.NewBuffer(by), r.Body),
				Closer: r.Body,
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

func sanitiseFilepath(p string) string {
	r := strings.NewReplacer(
		"(", "",
		")", "",
		"!", "",
		"+", "",
		"?", "",
		"*", "",
		"&", "",
		"^", "",
		"=", "",
		" ", "",
	)
	return r.Replace(p)
}

// ProxyCloser decouples the reader from the closer.
// This enables the use of io.MultiReader while closing the underlying source.
type ProxyCloser struct {
	io.Reader
	io.Closer
}

// LogWriteHeaderErrors helps when debugging http "multiple response.WriteHeader"
// calls.
type LogWriteHeaderErrors struct {
	Out io.Writer
}

func (d LogWriteHeaderErrors) Write(p []byte) (n int, err error) {
	s := string(p)
	if strings.Contains(s, "multiple response.WriteHeader") {
		n, err = d.Out.Write(debug.Stack())
		if err != nil {
			return n, err
		}
	}
	return d.Out.Write(p)
}

// FIXME(jfm): The error should be logged instead of returned to client, such
// 	that it makes it's way to the developers.
// 	You don't want clients knowing your failure modes, since this can lead
// 	to security exploits.
func httpError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	type appErr struct {
		Error error `json:"error,omitempty"`
	}
	// Wrap an error to ensure the returned object has an "error" property.
	if err, ok := v.(error); ok {
		v = appErr{
			Error: err,
		}
	}
	by, err := json.Marshal(v)
	if err != nil {
		httpError(w, errors.Wrap(err, "creating json response"))
		return
	}
	if _, err := w.Write(by); err != nil {
		// log? panic?
		panic(errors.Wrap(err, "atttempting to write to http.ResponseWriter"))
	}
}
