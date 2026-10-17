package mcp

import (
	"ciview/internal/ci"
	"context"
	"errors"
	"fmt"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"strings"
	"testing"
	"time"
)

type reader struct {
	value ci.Overview
	err   error
}

func (r reader) Overview(ctx context.Context) (ci.Overview, error) {
	if ctx.Err() != nil {
		return ci.Overview{}, ctx.Err()
	}
	return r.value, r.err
}
func TestCachedTools(t *testing.T) {
	zero := int64(0)
	now := time.Now().UTC()
	runs := []ci.Run{}
	for i, state := range []string{"success", "failure", "cancelled", "skipped", "timed_out", "neutral", "action_required", "stale", "unknown"} {
		runs = append(runs, ci.Run{ID: fmt.Sprint(i), CreatedAt: now.Add(time.Duration(i) * time.Second), State: state, Terminal: true})
	}
	runs[0].DurationMS = &zero
	issues := make([]ci.Issue, 25)
	for i := range issues {
		issues[i] = ci.Issue{Kind: "unavailable", Message: "secret raw error"}
	}
	value := ci.Overview{GeneratedAt: now, PollIntervalSeconds: 30, Sources: []ci.SourceState{{Name: "git", State: "unavailable", Issues: issues}}, Workflows: []ci.Workflow{{Key: "b", Name: "Build", Repository: ci.Repository{Source: "git", Provider: "github", FullName: "owner/repo"}, LatestRun: &runs[0], Runs: runs, Coverage: ci.Coverage{Partial: true, FetchedRuns: 9}, Health: ci.Health{State: "stale", Message: "secret raw error"}}, {Key: "a", Name: "Other"}}}
	s := session(t, NewHandler(reader{value: value}, nil), 4)
	out := call(t, s, "list_workflows", map[string]any{"query": "BUILD", "provider": "github", "source": "git", "repository": "owner/repo", "status": "success", "limit": 1}, false)
	if out["total"] != float64(1) || out["generated_at"] == nil {
		t.Fatal(out)
	}
	w := out["workflows"].([]any)[0].(map[string]any)
	if _, ok := w["runs"]; ok {
		t.Fatal("unbounded runs")
	}
	one := call(t, s, "get_workflow", map[string]any{"workflow_key": "b"}, false)
	if strings.Contains(fmt.Sprint(one), "secret") {
		t.Fatal("raw message leaked")
	}
	page := call(t, s, "list_runs", map[string]any{"workflow_key": "b", "limit": 2}, false)
	items := page["runs"].([]any)
	if items[0].(map[string]any)["state"] != "unknown" || page["next_offset"] != float64(2) {
		t.Fatal(page)
	}
	all := call(t, s, "list_runs", map[string]any{"workflow_key": "b"}, false)["runs"].([]any)
	if all[8].(map[string]any)["duration_ms"] != float64(0) || all[0].(map[string]any)["duration_ms"] != nil {
		t.Fatal("null timing")
	}
	source := call(t, s, "get_source_status", map[string]any{}, false)
	item := source["sources"].([]any)[0].(map[string]any)
	if len(item["issues"].([]any)) != 20 || item["issues_truncated"] != true || strings.Contains(fmt.Sprint(source), "secret") {
		t.Fatal(source)
	}
	for _, tool := range []string{"get_workflow", "list_runs"} {
		call(t, s, tool, map[string]any{"workflow_key": "missing"}, true)
	}
	call(t, s, "get_source_status", map[string]any{"source": "missing"}, true)
	for _, limit := range []int{0, -1, 101} {
		call(t, s, "list_workflows", map[string]any{"limit": limit}, true)
	}
	call(t, s, "list_runs", map[string]any{"workflow_key": "b", "offset": -1}, true)
	if value.Workflows[0].Runs[0].ID != "0" || value.Workflows[0].Health.Message != "secret raw error" {
		t.Fatal("snapshot mutated")
	}
	bad := session(t, NewHandler(reader{err: errors.New("secret")}, nil), 4)
	call(t, bad, "get_source_status", map[string]any{}, true)
}

type blockingReader struct{ canceled chan struct{} }

func (r blockingReader) Overview(ctx context.Context) (ci.Overview, error) {
	<-ctx.Done()
	close(r.canceled)
	return ci.Overview{}, ctx.Err()
}
func TestRequestCancellation(t *testing.T) {
	canceled := make(chan struct{})
	s := session(t, NewHandler(blockingReader{canceled}, nil), 4)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := s.CallTool(ctx, &sdk.CallToolParams{Name: "get_source_status", Arguments: map[string]any{}})
	if err == nil {
		t.Fatal("expected canceled call")
	}
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("reader context did not cancel")
	}
}

func TestSearchMatchesUIWordsAndFields(t *testing.T) {
	value := ci.Overview{Workflows: []ci.Workflow{{Key: "build", Name: "Build", Repository: ci.Repository{Source: "work", Provider: "github", FullName: "owner/repo"}}}}
	s := session(t, NewHandler(reader{value: value}, nil), 4)
	for _, query := range []string{" github   Build ", "owner/repo Build", "work github", "Build missing"} {
		out := call(t, s, "list_workflows", map[string]any{"query": query}, false)
		want := float64(1)
		if strings.Contains(query, "missing") {
			want = 0
		}
		if out["total"] != want {
			t.Errorf("query %q total %v want %v", query, out["total"], want)
		}
	}
}
