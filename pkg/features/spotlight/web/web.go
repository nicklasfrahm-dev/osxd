// Package web turns launcher queries into links and web searches.
package web

import (
	"net"
	"net/url"
	"strings"
)

// DefaultSearch is the search engine used when none is configured.
// %s is replaced with the escaped query.
const DefaultSearch = "https://duckduckgo.com/?q=%s"

// SearchURL returns the address that searches the web for query using
// template, in which %s stands for the escaped query.
func SearchURL(template, query string) string {
	if template == "" || !strings.Contains(template, "%s") {
		template = DefaultSearch
	}
	return strings.Replace(template, "%s", url.QueryEscape(strings.TrimSpace(query)), 1)
}

// Link reports whether query looks like a website or link and, if so, returns
// the URL to open. Inputs without a scheme get https://, except localhost and
// IP addresses, which get http://.
//
// "github.com", "https://go.dev/doc", "localhost:8080" and "mailto:a@b.co" are
// links; "hello world", "main.go" and "1.5" are not.
func Link(query string) (string, bool) {
	q := strings.TrimSpace(query)
	if q == "" || strings.ContainsAny(q, " \t\n") {
		return "", false
	}
	lower := strings.ToLower(q)
	for _, s := range []string{"mailto:", "tel:"} {
		if strings.HasPrefix(lower, s) && len(q) > len(s) {
			return q, true
		}
	}
	if u, err := url.Parse(q); err == nil && strings.Contains(q, "://") {
		if u.Host == "" && u.Scheme != "file" {
			return "", false
		}
		return q, true
	}

	u, err := url.Parse("//" + q)
	if err != nil || u.Host == "" || u.User != nil {
		return "", false
	}
	host := u.Hostname()
	switch {
	case host == "localhost" || net.ParseIP(host) != nil && strings.Count(host, ".") == 3:
		return "http://" + q, true
	case isDomain(host):
		return "https://" + q, true
	}
	return "", false
}

// isDomain accepts names like example.com or www.bbc.co.uk whose last label is
// a plausible top-level domain. File names such as main.go are rejected by
// requiring a known TLD for two-letter endings that clash with extensions.
func isDomain(host string) bool {
	labels := strings.Split(strings.ToLower(host), ".")
	if len(labels) < 2 {
		return false
	}
	for _, l := range labels {
		if l == "" || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return false
		}
		for _, r := range l {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	tld := labels[len(labels)-1]
	if len(tld) < 2 {
		return false
	}
	for _, r := range tld {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return !fileExt[tld]
}

// fileExt lists file extensions that are more likely meant as a file name
// than as a top-level domain, so "notes.md" is not opened as a website. Type
// the scheme, e.g. https://example.sh, to open such a domain anyway.
var fileExt = map[string]bool{
	"c": true, "cc": true, "cpp": true, "cs": true, "css": true, "csv": true,
	"doc": true, "docx": true, "exe": true, "gif": true, "go": true, "gz": true,
	"h": true, "htm": true, "html": true, "ini": true, "iso": true, "java": true,
	"jpeg": true, "jpg": true, "js": true, "json": true, "log": true, "lua": true,
	"md": true, "mkv": true, "mov": true, "mp3": true, "mp4": true, "odt": true,
	"pdf": true, "php": true, "pl": true, "png": true, "ppt": true, "pptx": true,
	"py": true, "rb": true, "rs": true, "sh": true, "sql": true, "svg": true,
	"tar": true, "toml": true, "ts": true, "txt": true, "wav": true, "xls": true,
	"xlsx": true, "xml": true, "yaml": true, "yml": true, "zip": true,
}
