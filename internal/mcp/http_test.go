package mcp

import (
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPPolicy(t *testing.T) {
	for _, value := range []string{"https://example.net", "*.example.net", "bad:8080", "a,,b"} {
		if _, err := ParseAllowedHosts(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	if _, err := ParseAllowedHosts("example.net, [::1]"); err != nil {
		t.Fatal(err)
	}
	h := protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), []string{"example.net"})
	for _, tc := range []struct {
		host, origin string
		status       int
	}{{"localhost:8080", "", 204}, {"[::1]:8080", "", 204}, {"example.net", "", 204}, {"evil.net", "", 403}, {"example.net", "https://evil.net", 403}, {"example.net", "null", 403}, {"example.net", "https://example.net/", 403}, {"example.net", "https://example.net", 204}, {"example.net", "https://user@example.net", 403}} {
		r := httptest.NewRequest("POST", "http://"+tc.host+"/mcp", strings.NewReader("{}"))
		r.Host = tc.host
		if tc.origin != "" {
			r.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Errorf("%+v got %d", tc, w.Code)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cache")
		}
	}

}

func TestPolicyMethodsAndBodyLimit(t *testing.T) {
	h := transport(sdk.NewServer(&sdk.Implementation{Name: "test", Version: "1"}, nil), nil)
	for _, method := range []string{"GET", "DELETE", "PUT"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "http://localhost/mcp", nil))
		if w.Code != 405 || w.Header().Get("Allow") != "POST" {
			t.Fatalf("method %s: %d", method, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader(strings.Repeat(" ", (1<<20)+1)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.ContentLength = -1
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 413 {
		t.Fatalf("oversize %d: %s", w.Code, w.Body.String())
	}
}

func TestMalformedAuthoritiesAndOrigins(t *testing.T) {
	h := protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), []string{"example.net"})
	for _, tc := range []struct{ host, origin string }{
		{"example.net:", ""}, {"example.net:99999", ""}, {"example.net.evil", ""}, {"user@example.net", ""}, {"example.net", "https://example.net?"}, {"example.net", "https://example.net#"}, {"example.net:8080", "https://example.net"}, {"example.net", ""},
	} {
		r := httptest.NewRequest("POST", "http://localhost/mcp", strings.NewReader("{}"))
		r.Host = tc.host
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("accepted %+v: %d", tc, w.Code)
		}
	}
	r := httptest.NewRequest("POST", "http://localhost/mcp", nil)
	r.Header.Add("Origin", "http://localhost")
	r.Header.Add("Origin", "http://evil.net")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("multiple origins accepted")
	}
}
