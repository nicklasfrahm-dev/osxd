// Package preview fetches the title, description and image of a web page,
// the way chat apps show a card for a pasted link.
package preview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html/charset"
)

const (
	maxPage  = 1 << 20 // metadata lives in <head>; stop reading long pages early
	maxImage = 5 << 20
	maxCache = 64
)

// Preview describes a web page.
type Preview struct {
	URL         string // the page, after redirects
	Title       string
	Description string
	SiteName    string
	ImageURL    string
	Image       []byte // encoded image, if one could be fetched
	// IsIcon is true when Image is only the site's icon, because the page
	// names no preview image.
	IsIcon bool

	iconURL string
}

// Fetcher fetches previews and remembers recent ones.
type Fetcher struct {
	Client *http.Client

	mu    sync.Mutex
	cache map[string]Preview
	order []string
}

// NewFetcher returns a Fetcher with a short timeout, so a slow site never
// keeps a preview waiting for long.
func NewFetcher() *Fetcher {
	return &Fetcher{Client: &http.Client{Timeout: 8 * time.Second}}
}

// Cached returns the preview of rawURL if it was fetched recently.
func (f *Fetcher) Cached(rawURL string) (Preview, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.cache[rawURL]
	return p, ok
}

// Get fetches the preview of an http or https URL.
func (f *Fetcher) Get(ctx context.Context, rawURL string) (Preview, error) {
	if p, ok := f.Cached(rawURL); ok {
		return p, nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return Preview{}, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return Preview{}, fmt.Errorf("cannot preview %s links", u.Scheme)
	}

	resp, err := f.get(ctx, rawURL, "text/html,application/xhtml+xml,image/*;q=0.8,*/*;q=0.5")
	if err != nil {
		return Preview{}, err
	}
	defer resp.Body.Close()
	final := resp.Request.URL
	ctype, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))

	var p Preview
	switch {
	case strings.HasPrefix(ctype, "image/"):
		// A direct link to an image previews as the image itself.
		if img, err := readImage(resp.Body); err == nil {
			p.Image = img
		}
		p.Title = path.Base(final.Path)
	case ctype == "text/html" || ctype == "application/xhtml+xml" || ctype == "":
		body, err := charset.NewReader(io.LimitReader(resp.Body, maxPage), resp.Header.Get("Content-Type"))
		if err != nil {
			return Preview{}, err
		}
		p = Parse(final, body)
		f.fetchImage(ctx, final, &p)
	default:
		p.Title = path.Base(final.Path)
	}
	p.URL = final.String()
	if p.SiteName == "" {
		p.SiteName = strings.TrimPrefix(final.Hostname(), "www.")
	}
	if p.Title == "" || p.Title == "/" || p.Title == "." {
		p.Title = p.SiteName
	}
	f.store(rawURL, p)
	return p, nil
}

func (f *Fetcher) get(ctx context.Context, rawURL, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// Some sites only serve OpenGraph tags to clients that look like a
	// browser or a link-preview bot.
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) osxd link preview")
	req.Header.Set("Accept", accept)
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", rawURL, resp.Status)
	}
	return resp, nil
}

// fetchImage downloads the page's preview image, or its icon if it has none.
// A missing image is not an error; the preview just has no picture.
func (f *Fetcher) fetchImage(ctx context.Context, page *url.URL, p *Preview) {
	candidates := []string{p.ImageURL}
	if p.ImageURL == "" {
		p.IsIcon = true
		candidates = []string{p.iconURL, page.ResolveReference(&url.URL{Path: "/favicon.ico"}).String()}
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if img, err := f.image(ctx, c); err == nil {
			p.Image = img
			return
		}
	}
}

func (f *Fetcher) image(ctx context.Context, rawURL string) ([]byte, error) {
	resp, err := f.get(ctx, rawURL, "image/*")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.HasPrefix(ct, "image/") {
		return nil, fmt.Errorf("%s is %s, not an image", rawURL, ct)
	}
	return readImage(resp.Body)
}

func readImage(r io.Reader) ([]byte, error) {
	img, err := io.ReadAll(io.LimitReader(r, maxImage+1))
	if err != nil {
		return nil, err
	}
	if len(img) > maxImage {
		return nil, errors.New("image too large")
	}
	return img, nil
}

func (f *Fetcher) store(key string, p Preview) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cache == nil {
		f.cache = map[string]Preview{}
	}
	if _, ok := f.cache[key]; !ok {
		f.order = append(f.order, key)
	}
	f.cache[key] = p
	for len(f.order) > maxCache {
		delete(f.cache, f.order[0])
		f.order = f.order[1:]
	}
}
