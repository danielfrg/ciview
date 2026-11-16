package web

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestStaticRejectsDirectoriesDotfilesAndTraversal(t *testing.T) {
	h := NewHandler(fstest.MapFS{"index.html": {Data: []byte("app")}, "assets/app.js": {Data: []byte("js")}, ".env": {Data: []byte("secret")}})
	for _, path := range []string{"/assets/", "/.env", "/assets/../.env", "//index.html", "/api/unknown"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 || strings.Contains(w.Body.String(), "secret") {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
}
func TestMissingBuildIsUnavailable(t *testing.T) {
	w := httptest.NewRecorder()
	NewHandler(fstest.MapFS{}).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 503 {
		t.Fatalf("missing build: %d", w.Code)
	}
}

func TestOnlyNamedAppRoutesServeTheShell(t *testing.T) {
	h := NewHandler(fstest.MapFS{"index.html": {Data: []byte("app")}})
	for path, code := range map[string]int{"/": 200, "/workflows": 200, "/runners": 200, "/workflows/": 404, "/workflows/x": 404, "/other": 404, "/api/workflows": 404} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path+"?status=failure", nil))
		if w.Code != code || (code == 200 && w.Body.String() != "app") {
			t.Errorf("%s: %d %q", path, w.Code, w.Body.String())
		}
		if code == 200 && w.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("%s: shell must not be cached: %q", path, w.Header().Get("Cache-Control"))
		}
	}
}
