package preview

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	base, _ := url.Parse("https://example.com/blog/post")
	page := `<!doctype html><html><head>
		<title>  Plain
		title </title>
		<meta name="description" content="plain description">
		<meta property="og:title" content="OG title">
		<meta property="og:title" content="second OG title">
		<meta name="twitter:description" content="twitter description">
		<meta property="og:site_name" content="Example Blog">
		<meta property="og:image" content="/img/card.png">
		<link rel="shortcut icon" href="/favicon.png">
		</head><body><meta property="og:description" content="in body"></body></html>`
	p := Parse(base, strings.NewReader(page))
	want := Preview{
		Title:       "OG title",
		Description: "twitter description",
		SiteName:    "Example Blog",
		ImageURL:    "https://example.com/img/card.png",
		iconURL:     "https://example.com/favicon.png",
	}
	if !reflect.DeepEqual(p, want) {
		t.Errorf("Parse =\n%+v\nwant\n%+v", p, want)
	}

	p = Parse(base, strings.NewReader(`<title>Only &amp; title</title><body>`))
	if p.Title != "Only & title" || p.ImageURL != "" {
		t.Errorf("fallback Parse = %+v", p)
	}
}

func TestGet(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\nfake")
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><head><title>Home</title><meta name="description" content="Hello"></head></html>`)
	})
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(png)
	})
	mux.HandleFunc("/old", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusFound)
	})
	mux.HandleFunc("/missing", http.NotFound)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f := NewFetcher()
	p, err := f.Get(context.Background(), srv.URL+"/old")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Home" || p.Description != "Hello" || p.URL != srv.URL+"/" {
		t.Errorf("Get = %+v", p)
	}
	if !p.IsIcon || string(p.Image) != string(png) {
		t.Errorf("expected the favicon as image, got %q (icon %v)", p.Image, p.IsIcon)
	}
	if _, ok := f.Cached(srv.URL + "/old"); !ok {
		t.Error("preview was not cached")
	}

	if _, err := f.Get(context.Background(), srv.URL+"/missing"); err == nil {
		t.Error("404 should fail")
	}
	if _, err := f.Get(context.Background(), "mailto:a@example.com"); err == nil {
		t.Error("mailto should not be fetched")
	}
}
