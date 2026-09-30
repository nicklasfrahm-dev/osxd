package web

import "testing"

func TestLink(t *testing.T) {
	for in, want := range map[string]string{
		"github.com":                 "https://github.com",
		"  go.dev/doc  ":             "https://go.dev/doc",
		"www.bbc.co.uk/news?x=1":     "https://www.bbc.co.uk/news?x=1",
		"https://example.com/a b":    "",
		"https://example.com/path":   "https://example.com/path",
		"http://localhost:3000":      "http://localhost:3000",
		"localhost:8080/api":         "http://localhost:8080/api",
		"192.168.1.1":                "http://192.168.1.1",
		"mailto:someone@example.com": "mailto:someone@example.com",
		"file:///etc/hosts":          "file:///etc/hosts",
		"hello world":                "",
		"firefox":                    "",
		"main.go":                    "",
		"notes.md":                   "",
		"1.5":                        "",
		"v1.2.3":                     "",
		"user@example.com":           "",
		"":                           "",
		"foo..com":                   "",
		"https://":                   "",
	} {
		got, ok := Link(in)
		if want == "" {
			if ok {
				t.Errorf("Link(%q) = %q, want no link", in, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("Link(%q) = %q, %v, want %q", in, got, ok, want)
		}
	}
}

func TestSearchURL(t *testing.T) {
	if got := SearchURL("", " go generics & tips "); got != "https://duckduckgo.com/?q=go+generics+%26+tips" {
		t.Errorf("default = %s", got)
	}
	if got := SearchURL("https://www.google.com/search?q=%s", "a b"); got != "https://www.google.com/search?q=a+b" {
		t.Errorf("google = %s", got)
	}
}
