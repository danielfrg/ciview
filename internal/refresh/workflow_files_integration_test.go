package refresh

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"ciview/internal/ci"
	"ciview/internal/providers/forgejo"
	"ciview/internal/providers/github"
	"ciview/internal/snapshots"
)

func TestProvidersRemoveHistoricalWorkflowRowsWhenBranchFileIsDeleted(t *testing.T) {
	for _, tc := range []struct {
		kind         string
		existingOnly bool
	}{{"github", true}, {"github", false}, {"forgejo", true}, {"forgejo", false}} {
		kind := tc.kind
		t.Run(fmt.Sprintf("%s/filter=%v", kind, tc.existingOnly), func(t *testing.T) {
			var deleted atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/api/v1")
				switch path {
				case "/repos/example/project":
					fmt.Fprint(w, `{"id":42,"name":"project","owner":{"login":"example"},"full_name":"example/project","default_branch":"main","html_url":"https://example.net/example/project"}`)
				case "/repos/example/project/contents/.github/workflows", "/repos/example/project/contents/.forgejo/workflows":
					if r.URL.Query().Get("ref") != "main" {
						t.Errorf("wrong branch: %s", r.URL)
					}
					dir := ".github/workflows"
					if kind == "forgejo" {
						dir = ".forgejo/workflows"
					}
					fmt.Fprintf(w, `[{"name":"ci.yml","path":%q,"type":"file"}`, dir+"/ci.yml")
					if !deleted.Load() {
						fmt.Fprintf(w, `,{"name":"removed.yml","path":%q,"type":"file"}`, dir+"/removed.yml")
					}
					fmt.Fprint(w, `]`)
				case "/repos/example/project/actions/runs":
					if kind == "github" {
						fmt.Fprint(w, `{"total_count":2,"workflow_runs":[{"id":1,"workflow_id":11,"path":".github/workflows/ci.yml","name":"CI","head_branch":"main","event":"push","status":"in_progress","created_at":"2026-10-14T12:00:00Z"},{"id":2,"workflow_id":12,"path":".github/workflows/removed.yml","name":"Removed","head_branch":"main","event":"push","status":"in_progress","created_at":"2026-10-14T12:01:00Z"}]}`)
					} else {
						fmt.Fprint(w, `{"total_count":2,"workflow_runs":[{"id":1,"workflow_id":"ci.yml","prettyref":"refs/heads/main","event":"push","status":"running","created":"2026-10-14T12:00:00Z"},{"id":2,"workflow_id":"removed.yml","prettyref":"refs/heads/main","event":"push","status":"running","created":"2026-10-14T12:01:00Z"}]}`)
					}
				default:
					t.Errorf("unexpected provider request: %s", r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			var client ci.Provider
			var err error
			if kind == "github" {
				client, err = github.New("public", server.URL, "", server.Client())
			} else {
				client, err = forgejo.New("public", server.URL, "", server.Client())
			}
			if err != nil {
				t.Fatal(err)
			}
			cfg := testConfig()
			cfg.Sources[0].Provider = kind
			cfg.Sources[0].ExistingWorkflowsOnly = tc.existingOnly
			store := snapshots.NewMemory()
			c := New(cfg, map[string]ci.Provider{"public": client}, store)
			overview := ci.NewOverview(store, c, 30, "default", cfg.Viewer.RefreshDuration())
			for _, remove := range []bool{false, true} {
				deleted.Store(remove)
				if err := c.Refresh(context.Background()); err != nil {
					t.Fatal(err)
				}
				result, err := overview.Overview(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				want := 2
				if remove && tc.existingOnly {
					want = 1
				}
				if len(result.Workflows) != want {
					t.Fatalf("deleted=%v rows=%+v", remove, result.Workflows)
				}
				for _, row := range result.Workflows {
					exists := !remove || row.LatestRun.ID == "1"
					if row.FileExists == nil || *row.FileExists != exists {
						t.Fatalf("wrong workflow marker: %+v", row)
					}
				}
				if remove && tc.existingOnly && result.Workflows[0].LatestRun.ID != "1" {
					t.Fatal("current branch workflow disappeared")
				}
			}
		})
	}
}

func TestAllBranchScopeKeepsHistoricalWorkflows(t *testing.T) {
	p := provider()
	p.workflowFiles = []string{}
	p.workflowErr = &ci.ProviderError{Kind: "permission"}
	cfg := testConfig()
	cfg.Viewer.Branch = "all"
	cfg.Sources[0].ExistingWorkflowsOnly = true
	store := snapshots.NewMemory()
	c := New(cfg, map[string]ci.Provider{"public": p}, store)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(read(t, store).Runs) != 1 || len(p.workflowBranches) != 0 {
		t.Fatal("all-branch history was filtered by a single branch's files")
	}
}
