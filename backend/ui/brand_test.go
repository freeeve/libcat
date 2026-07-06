package ui

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func get(t *testing.T, h *httptest.Server, path string) (string, string) {
	t.Helper()
	res, err := h.Client().Get(h.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return string(body), res.Header.Get("Content-Type")
}

func TestHandlerLinksBrandCSS(t *testing.T) {
	css := ":root{--accent:#c42a8c;}html[data-theme=\"dark\"]{--accent:#df41a5;}"
	srv := httptest.NewServer(Handler([]byte(css)))
	defer srv.Close()

	for _, path := range []string{"/", "/index.html", "/some/spa/route"} {
		body, ct := get(t, srv, path)
		if !strings.Contains(body, `<link id="lcat-brand" rel="stylesheet" href="/brand.css"></head>`) {
			t.Errorf("GET %s: brand link not injected before </head>", path)
		}
		if !strings.HasPrefix(ct, "text/html") {
			t.Errorf("GET %s: Content-Type = %q", path, ct)
		}
	}

	body, ct := get(t, srv, "/brand.css")
	if body != css {
		t.Errorf("GET /brand.css = %q, want the configured CSS", body)
	}
	if !strings.HasPrefix(ct, "text/css") {
		t.Errorf("GET /brand.css: Content-Type = %q", ct)
	}
}

func TestHandlerWithoutBrandCSS(t *testing.T) {
	srv := httptest.NewServer(Handler(nil))
	defer srv.Close()

	body, _ := get(t, srv, "/")
	if strings.Contains(body, "lcat-brand") {
		t.Fatal("brand link injected with no brand CSS configured")
	}
	if !strings.Contains(body, "<html") {
		t.Fatal("index.html not served")
	}
	// /brand.css falls through to the SPA fallback rather than 404ing a
	// stylesheet that does not exist.
	if body, _ := get(t, srv, "/brand.css"); !strings.Contains(body, "<html") {
		t.Fatal("unconfigured /brand.css did not fall back to index")
	}
}
