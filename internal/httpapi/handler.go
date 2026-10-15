// Package httpapi exposes cached CI state through a typed Huma API.
package httpapi

import (
	"context"
	"io/fs"
	"net/http"
	"strings"

	"ciview/internal/ci"
	"ciview/internal/web"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

type overviewResponse struct{ Body ci.Overview }
type healthResponse struct {
	Body struct {
		Status string `json:"status"`
	}
}

func NewHandler(assets fs.FS, reader ci.OverviewReader) http.Handler {
	mux := http.NewServeMux()
	cfg := huma.DefaultConfig("CI View API", "0.1.0")
	cfg.OpenAPIPath = "/api/openapi"
	cfg.DocsPath = "/api/docs"
	cfg.SchemasPath = "/api/schemas"
	// Preserve the normalized model's exact JSON shape, including nullable metrics.
	cfg.CreateHooks = nil
	api := humago.New(mux, cfg)
	huma.Register(api, huma.Operation{OperationID: "get-overview", Method: http.MethodGet, Path: "/api/overview", Summary: "Read cached workflow overview", Errors: []int{http.StatusServiceUnavailable}}, func(ctx context.Context, _ *struct{}) (*overviewResponse, error) {
		value, err := reader.Overview(ctx)
		if err != nil {
			return nil, huma.Error503ServiceUnavailable("Cached overview unavailable")
		}
		return &overviewResponse{Body: value}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-health", Method: http.MethodGet, Path: "/healthz", Summary: "Check process health"}, func(context.Context, *struct{}) (*healthResponse, error) {
		response := &healthResponse{}
		response.Body.Status = "ok"
		return response, nil
	})
	mux.Handle("/", web.NewHandler(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			w.Header().Set("Cache-Control", "no-store")
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
	})
}
