package preview

import (
	"io"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// Parse reads the metadata in an HTML page's head. OpenGraph tags win over
// Twitter cards, which win over the plain <title> and description. Relative
// image and icon URLs are resolved against base.
func Parse(base *url.URL, r io.Reader) Preview {
	var (
		p               Preview
		title           strings.Builder
		inTitle         bool
		meta            = map[string]string{}
		icon, touchIcon string
	)
	z := html.NewTokenizer(r)
loop:
	for {
		switch z.Next() {
		case html.ErrorToken:
			break loop
		case html.TextToken:
			if inTitle {
				title.Write(z.Text())
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "title":
				inTitle = false
			case "head":
				break loop
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			attrs := map[string]string{}
			for hasAttr {
				var k, v []byte
				k, v, hasAttr = z.TagAttr()
				attrs[string(k)] = string(v)
			}
			switch string(name) {
			case "body":
				break loop
			case "title":
				inTitle = true
			case "meta":
				key := strings.ToLower(attrs["property"])
				if key == "" {
					key = strings.ToLower(attrs["name"])
				}
				if _, seen := meta[key]; !seen && key != "" && attrs["content"] != "" {
					meta[key] = attrs["content"]
				}
			case "link":
				for _, rel := range strings.Fields(strings.ToLower(attrs["rel"])) {
					switch {
					case rel == "apple-touch-icon" && touchIcon == "":
						touchIcon = attrs["href"]
					case rel == "icon" && icon == "":
						icon = attrs["href"]
					}
				}
			}
		}
	}

	first := func(keys ...string) string {
		for _, k := range keys {
			if v := clean(meta[k]); v != "" {
				return v
			}
		}
		return ""
	}
	p.Title = first("og:title", "twitter:title")
	if p.Title == "" {
		p.Title = clean(title.String())
	}
	p.Description = first("og:description", "twitter:description", "description")
	p.SiteName = first("og:site_name", "application-name")
	p.ImageURL = resolve(base, first("og:image:secure_url", "og:image:url", "og:image", "twitter:image", "twitter:image:src"))
	// Touch icons are larger than favicons and look better in the card.
	if touchIcon != "" {
		icon = touchIcon
	}
	p.iconURL = resolve(base, icon)
	return p
}

// clean collapses whitespace and drops invalid UTF-8.
func clean(s string) string {
	return strings.Join(strings.Fields(strings.ToValidUTF8(s, "")), " ")
}

// resolve makes ref absolute; anything other than http(s) is dropped.
func resolve(base *url.URL, ref string) string {
	if ref == "" {
		return ""
	}
	u, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return ""
	}
	u = base.ResolveReference(u)
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.String()
}
