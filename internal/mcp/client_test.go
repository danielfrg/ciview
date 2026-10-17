package mcp

import (
	"context"
	"encoding/json"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func session(t *testing.T, h http.Handler, n int) *sdk.ClientSession {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	s, e := sdk.NewClient(&sdk.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, &sdk.StreamableClientTransport{Endpoint: server.URL, DisableStandaloneSSE: true}, nil)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	v, e := s.ListTools(ctx, nil)
	if e != nil || len(v.Tools) != n {
		t.Fatalf("tools %v %v", v, e)
	}
	for _, tool := range v.Tools {
		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Fatal("not read-only")
		}
	}
	return s
}
func call(t *testing.T, s *sdk.ClientSession, name string, args any, wantError bool) map[string]any {
	t.Helper()
	v, e := s.CallTool(context.Background(), &sdk.CallToolParams{Name: name, Arguments: args})
	if e != nil {
		t.Fatal(e)
	}
	if v.IsError != wantError {
		t.Fatalf("%s: %v", name, v)
	}
	if wantError {
		return nil
	}
	b, e := json.Marshal(v.StructuredContent)
	if e != nil {
		t.Fatal(e)
	}
	var out map[string]any
	if e = json.Unmarshal(b, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
