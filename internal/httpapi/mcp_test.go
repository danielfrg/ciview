package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestMCPEndpoint(t *testing.T) {
	h := NewHandler(nil, nil)
	for _, method := range []string{"GET", "HEAD", "PUT"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "http://localhost/mcp", nil))
		if w.Code != 405 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("MCP routing %s: %d", method, w.Code)
		}
	}
}
