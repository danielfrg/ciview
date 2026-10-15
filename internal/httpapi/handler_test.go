package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"ciview/internal/ci"
)

type overviewReader struct {
	calls int
	value ci.Overview
	err   error
}

func (r *overviewReader) Overview(context.Context) (ci.Overview, error) {
	r.calls++
	return r.value, r.err
}
func TestCachedOverviewHealthAndNoStore(t *testing.T) {
	r := &overviewReader{value: ci.Overview{GeneratedAt: time.Now().UTC(), PollIntervalSeconds: 60, Sources: []ci.SourceState{{Name: "public", State: "partial", Issues: []ci.Issue{{Repository: "example/project", Kind: "permission", Message: "provider permission denied"}}}}}}
	h := NewHandler(fstest.MapFS{"index.html": {Data: []byte("app")}}, r)
	health := httptest.NewRecorder()
	h.ServeHTTP(health, httptest.NewRequest("GET", "/healthz", nil))
	if health.Code != 200 || r.calls != 0 {
		t.Fatalf("health calls provider: %d %d", health.Code, r.calls)
	}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/overview", nil))
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("overview: %d %v", w.Code, w.Header())
		}
		var v ci.Overview
		if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
			t.Fatal(e)
		}
		if v.Sources[0].State != "partial" || v.Workflows == nil {
			t.Fatalf("contract: %#v", v)
		}
	}
	if r.calls != 2 {
		t.Fatal("GET unexpectedly refreshes")
	}
}
func TestErrorsAreSanitized(t *testing.T) {
	r := &overviewReader{err: errors.New("Authorization: secret upstream body")}
	w := httptest.NewRecorder()
	NewHandler(fstest.MapFS{}, r).ServeHTTP(w, httptest.NewRequest("GET", "/api/overview", nil))
	if w.Code != 503 || strings.Contains(w.Body.String(), "secret") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unsafe error: %d %s", w.Code, w.Body.String())
	}
}
func TestOpenAPIAndStaticRouting(t *testing.T) {
	r := &overviewReader{}
	h := NewHandler(fstest.MapFS{"index.html": {Data: []byte("CI View app")}, "assets/app.js": {Data: []byte("export const app=true")}, ".env": {Data: []byte("secret")}}, r)
	for _, path := range []string{"/api/openapi.json", "/api/openapi.yaml", "/api/docs", "/", "/assets/app.js"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if path == "/api/openapi.json" {
			var v map[string]any
			if e := json.Unmarshal(w.Body.Bytes(), &v); e != nil {
				t.Fatal(e)
			}
			p := v["paths"].(map[string]any)
			if p["/api/overview"] == nil {
				t.Fatal("overview absent from OpenAPI")
			}
		}
	}
	for _, path := range []string{"/.env", "/assets/", "/unknown", "/api/no-such-endpoint", "/api/overview/", "/assets/.private.js"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Errorf("%s should be rejected: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", nil))
	if w.Code != 405 {
		t.Errorf("static POST: %d", w.Code)
	}
}
