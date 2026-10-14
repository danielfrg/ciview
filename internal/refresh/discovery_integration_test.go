package refresh

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"civiewer/internal/ci"
	"civiewer/internal/providers/github"
	"civiewer/internal/snapshots"
)

func TestGitHubDiscoveryRateLimitStopsRemainingSelectionsAndRunScans(t *testing.T) {
	var mu sync.Mutex
	paths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/orgs/example/repos":
			fmt.Fprint(w, `[]`)
		case "/repos/example/first":
			fmt.Fprint(w, `{"id":22,"owner":{"login":"example"},"name":"first","full_name":"example/first","default_branch":"main","html_url":"https://github.com/example/first"}`)
		case "/repos/example/limited":
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"message":"private upstream payload"}`)
		default:
			t.Errorf("request after discovery rate limit: %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	cfg := testConfig()
	cfg.Sources[0].Orgs = []string{"example"}
	cfg.Sources[0].Repos = []string{"example/first", "example/limited", "example/third"}
	// Establish a previously observed organization repository through the normal
	// coordinator path before the organization listing becomes incomplete.
	initial := provider()
	m := snapshots.NewMemory()
	c := New(cfg, map[string]ci.Provider{"public": initial}, m)
	ctx := context.Background()
	if err := c.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	before := inventory(t, c)
	lastDiscovery := *before.LastDiscoveryAt
	client, err := github.New("public", server.URL, "", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	c.sources[0].provider = client
	c.now = func() time.Time { return time.Now().UTC().Add(16 * time.Minute) }
	if err := c.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	state := inventory(t, c)
	if state.State != "partial" || len(state.Repositories) != 2 || state.LastDiscoveryAt == nil || !state.LastDiscoveryAt.Equal(lastDiscovery) {
		t.Fatalf("partial discovery discarded inventory or claimed success: %#v", state)
	}
	mu.Lock()
	observed := append([]string{}, paths...)
	mu.Unlock()
	expected := []string{"/orgs/example/repos", "/repos/example/first", "/repos/example/limited"}
	if fmt.Sprint(observed) != fmt.Sprint(expected) {
		t.Fatalf("discovery ignored source cooldown: paths=%v", observed)
	}
	firstLoading, limitedIssue, retryReason, thirdDeferred := false, false, false, false
	for _, issue := range state.Issues {
		if issue.Repository == "example/first" && issue.Kind == "loading" {
			firstLoading = true
		}
		if issue.Repository == "example/third" && issue.Kind == "loading" {
			thirdDeferred = true
		}
		if issue.Repository == "example/limited" && issue.Kind == "rate_limit" {
			limitedIssue = true
		}
		if issue.Kind == "rate_limit" && strings.Contains(issue.Message, "retry after") {
			retryReason = true
		}
		if strings.Contains(issue.Message, "private") {
			t.Fatal("upstream response leaked into source notice")
		}
	}
	if !firstLoading || !limitedIssue || !retryReason || !thirdDeferred {
		t.Fatalf("discovery notices missing: %#v", state.Issues)
	}
	for _, key := range state.Repositories {
		snap, found, err := m.Read(ctx, key)
		if err != nil || !found {
			t.Fatalf("missing verified metadata: %v", err)
		}
		if snap.Repository.FullName == "example/first" && (snap.Health.State != "loading" || snap.Health.RetryAt == nil || len(snap.Runs) != 0) {
			t.Fatalf("new repository was scanned during source cooldown: %#v", snap)
		}
	}
	if err := c.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != len(expected) {
		t.Fatalf("second refresh hammered deferred source: %v", paths)
	}
}
