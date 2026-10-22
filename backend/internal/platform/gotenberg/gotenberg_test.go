package gotenberg

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTMLToPDF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/forms/chromium/convert/html" {
			http.Error(w, "route", http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f, h, err := r.FormFile("files")
		if err != nil || h.Filename != "index.html" || r.FormValue("preferCssPageSize") != "true" {
			http.Error(w, "form", http.StatusBadRequest)
			return
		}
		html, _ := io.ReadAll(f)
		_, _ = w.Write(append([]byte("%PDF-"), html...))
	}))
	defer srv.Close()

	c := New(srv.URL+"/", time.Second)
	pdf, err := c.HTMLToPDF(context.Background(), []byte("<p>Qadam</p>"), Page{PreferCSSPageSize: true})
	if err != nil || string(pdf) != "%PDF-<p>Qadam</p>" {
		t.Fatalf("%q %v", pdf, err)
	}
	if _, err := c.OfficeToPDF(context.Background(), "deck.pptx", []byte("x")); err == nil {
		t.Fatal("a failed conversion must be an error")
	}
	if _, err := New("", time.Second).HTMLToPDF(context.Background(), nil, Page{}); !errors.Is(err, ErrNotConfigured) {
		t.Fatal(err)
	}
}

func TestHTMLToPNG(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil || r.URL.Path != "/forms/chromium/screenshot/html" {
			http.Error(w, "route", http.StatusNotFound)
			return
		}
		if r.FormValue("width") != "1200" || r.FormValue("height") != "630" || r.FormValue("clip") != "true" || r.FormValue("format") != "png" {
			http.Error(w, "form", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("\x89PNG"))
	}))
	defer srv.Close()

	png, err := New(srv.URL, time.Second).HTMLToPNG(context.Background(), []byte("<p>Qadam</p>"), 1200, 630)
	if err != nil || string(png) != "\x89PNG" {
		t.Fatalf("%q %v", png, err)
	}
}
