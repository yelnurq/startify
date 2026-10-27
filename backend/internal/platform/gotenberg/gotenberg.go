// Package gotenberg converts documents with a Gotenberg server: HTML pages
// into PDF or images (Chromium) and office files into PDF (LibreOffice).
package gotenberg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ErrNotConfigured means no Gotenberg URL is set.
var ErrNotConfigured = errors.New("gotenberg: not configured")

// maxOutput bounds the size of a converted document.
const maxOutput = 50 << 20

// Client calls one Gotenberg server.
type Client struct {
	url  string
	http *http.Client
}

// New returns a client; an empty url gives a client that always fails with
// ErrNotConfigured.
func New(url string, timeout time.Duration) *Client {
	return &Client{url: strings.TrimSuffix(url, "/"), http: &http.Client{Timeout: timeout}}
}

// Page sets the paper of an HTML conversion; zero values keep the CSS @page size.
type Page struct {
	// Landscape slides and portrait sheets set their size with CSS @page.
	PreferCSSPageSize bool
}

// HTMLToPDF renders a self-contained HTML page (inline styles and images).
func (c *Client) HTMLToPDF(ctx context.Context, html []byte, page Page) ([]byte, error) {
	fields := map[string]string{"printBackground": "true", "marginTop": "0", "marginBottom": "0", "marginLeft": "0", "marginRight": "0"}
	if page.PreferCSSPageSize {
		fields["preferCssPageSize"] = "true"
	}
	return c.convert(ctx, "/forms/chromium/convert/html", "index.html", html, fields)
}

// HTMLToPNG takes a screenshot of a self-contained HTML page of the given
// size in CSS pixels.
func (c *Client) HTMLToPNG(ctx context.Context, html []byte, width, height int) ([]byte, error) {
	fields := map[string]string{
		"width": strconv.Itoa(width), "height": strconv.Itoa(height), "clip": "true", "format": "png", "optimizeForSpeed": "true",
	}
	return c.convert(ctx, "/forms/chromium/screenshot/html", "index.html", html, fields)
}

// OfficeToPDF converts a document such as PPTX or DOCX.
func (c *Client) OfficeToPDF(ctx context.Context, name string, data []byte) ([]byte, error) {
	return c.convert(ctx, "/forms/libreoffice/convert", name, data, nil)
}

func (c *Client) convert(ctx context.Context, route, name string, data []byte, fields map[string]string) ([]byte, error) {
	if c.url == "" {
		return nil, ErrNotConfigured
	}
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("files", name)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: form: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("gotenberg: form: %w", err)
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("gotenberg: form: %w", err)
		}
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("gotenberg: form: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+route, &body)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gotenberg: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	out, err := io.ReadAll(io.LimitReader(res.Body, maxOutput+1))
	if err != nil {
		return nil, fmt.Errorf("gotenberg: read: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gotenberg: %s: %s", res.Status, strings.TrimSpace(string(out[:min(len(out), 300)])))
	}
	if len(out) > maxOutput {
		return nil, errors.New("gotenberg: the document is too large")
	}
	return out, nil
}
