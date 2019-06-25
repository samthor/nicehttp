package nicehttp

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
)

var (
	// ErrInvalidStatusCode is generated when a Handler returns an out-of-range status code.
	ErrInvalidStatusCode = errors.New("status code <100 or >=600 specified")
)

// Template runs a *template.Template.
type Template struct {
	Template *template.Template
	Data     interface{}
}

// Redirect is a http.StatusSeeOther.
type Redirect string

// Status wraps a Response with a specific status code.
type Status struct {
	Status   int
	Response interface{}
}

// ContentType wraps a Response a with specific Content-Type.
type ContentType struct {
	ContentType string
	Response    interface{}
}

// Handler is the nicehttp helper type, allowing arbitrary return types from a HTTP handler. This supports:
//  - error to generate a StatusInternalServerError
//  - int for status code
//  - string to render directly
//  - the Template type to execute a template
//  - the Redirect type to cause a StatusSeeOther
//  - io.Reader to be piped to the output
//  - ... all other responses to be written as JSON
// Additionally, wrapper types are supported:
//  - the ContentType type to wrap another repsonse with a given Content-Type header
//  - the Status type to wrap another response with a HTTP status code
type Handler func(ctx context.Context, r *http.Request) interface{}

// Handle is a convenience method around the Handler type.
func Handle(pattern string, handler func(ctx context.Context, r *http.Request) interface{}) {
	http.Handle(pattern, Handler(handler))
}

// ServeHTTP implements http.Handler.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := h(ctx, r)

retry:
	var err error

	switch v := out.(type) {
	case ContentType:
		w.Header().Set("Content-Type", v.ContentType)
		out = v.Response
		goto retry

	case Status:
		w.WriteHeader(v.Status)
		out = v.Response
		goto retry

	case int:
		if v < 100 || v >= 600 {
			err = ErrInvalidStatusCode
			break
		}
		w.WriteHeader(v)

	case string:
		fmt.Fprint(w, v)

	case Template:
		err = v.Template.Execute(w, v.Data)

	case Redirect:
		target := string(v)
		if target == "" {
			target = r.URL.Path
		}
		http.Redirect(w, r, target, http.StatusSeeOther)

	case io.Reader:
		_, err = io.Copy(w, v)

	case []byte:
		w.Header().Set("Content-Length", strconv.Itoa(len(v)))
		_, err = w.Write(v)

	case error:
		err = v

	default:
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "GET" {
			// probably not idempotent, don't generate/check ETag
			err = json.NewEncoder(w).Encode(v)
			break
		}

		var b []byte
		b, err = json.Marshal(v)
		if err != nil {
			break
		}
		hash := md5.Sum(b)

		etag := fmt.Sprintf("\"%x\"", hash)
		if r.Header.Get("If-None-Match") == etag {
			http.Error(w, "", http.StatusNotModified)
			break
		}

		w.Header().Set("ETag", etag)
		_, err = w.Write(b)
	}

	if err != nil {
		log.Printf("got err serving: %v", err)
		code := http.StatusInternalServerError
		http.Error(w, http.StatusText(code), code)
	}
}
