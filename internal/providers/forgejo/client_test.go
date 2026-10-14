package forgejo

import (
	"civiewer/internal/ci"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPrefixedPrivateDiscoveryAndRunNanoseconds(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token forge-secret" {
			t.Error("missing private token")
		}
		if !strings.HasPrefix(r.URL.Path, "/forge/api/v1/") {
			t.Errorf("prefix lost %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/forge/api/v1/repos/acme/project":
			fmt.Fprint(w, `{"id":42,"name":"project","owner":{"login":"acme"},"full_name":"acme/project","default_branch":"trunk","html_url":"https://forge.example/forge/acme/project"}`)
		case "/forge/api/v1/repos/acme/project/actions/runs":
			fmt.Fprint(w, `{"total_count":2,"workflow_runs":[{"id":17,"workflow_id":"build.yml","title":"title changes","prettyref":"trunk","event":"push","html_url":"https://forge.example/forge/acme/project/actions/runs/17","status":"success","duration":2500000000,"created":"2026-01-01T00:00:00Z","started":"2026-01-01T00:00:01Z","stopped":"2026-01-01T00:00:03.5Z"},{"id":18,"workflow_id":"build.yml","status":"blocked","duration":5000000000,"created":"0001-01-01T00:00:00Z"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer s.Close()
	c, e := New("forge", s.URL+"/forge/", "forge-secret", s.Client())
	if e != nil {
		t.Fatal(e)
	}
	d, e := c.Discover(context.Background(), ci.Selection{Repos: []string{"acme/project"}})
	if e != nil || !d.Complete || len(d.Repositories) != 1 {
		t.Fatalf("discovery=%+v err=%v", d, e)
	}
	p, e := c.ListRuns(context.Background(), d.Repositories[0], 1)
	if e != nil || len(p.Runs) != 2 {
		t.Fatalf("runs=%+v err=%v", p, e)
	}
	r := p.Runs[0]
	if r.ID != "17" || r.WorkflowID != "build.yml" || r.WorkflowName != "build.yml" || r.State != "success" || !r.Terminal || r.DurationMS == nil || *r.DurationMS != 2500 || r.DurationKind != "execution" {
		t.Fatalf("run=%+v", r)
	}
	r = p.Runs[1]
	if r.Terminal || r.State != "waiting" || r.DurationMS != nil || !r.CreatedAt.IsZero() {
		t.Fatalf("blocked=%+v", r)
	}
}
func TestCombinedStatusesAndMissingDuration(t *testing.T) {
	for _, x := range []struct {
		raw, state string
		terminal   bool
	}{{"waiting", "queued", false}, {"running", "running", false}, {"blocked", "waiting", false}, {"failure", "failure", true}, {"cancelled", "canceled", true}, {"skipped", "skipped", true}, {"unknown", "unknown", false}, {"new_state", "unknown", false}} {
		t.Run(x.raw, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"id":1,"workflow_id":"test.yaml","status":%q,"duration":0}`, x.raw)
			}))
			defer s.Close()
			c, e := New("forge", s.URL, "", s.Client())
			if e != nil {
				t.Fatal(e)
			}
			r, e := c.GetRun(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, "1")
			if e != nil || r.State != x.state || r.Terminal != x.terminal || r.DurationMS != nil {
				t.Fatalf("run=%+v err=%v", r, e)
			}
		})
	}
}
func TestPrefixEscapeRejected(t *testing.T) {
	var escaped bool
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/outside" {
			escaped = true
		}
		http.Redirect(w, r, "/outside", 302)
	}))
	defer s.Close()
	c, e := New("forge", s.URL+"/forge", "secret", s.Client())
	if e != nil {
		t.Fatal(e)
	}
	_, e = c.ListRuns(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, 1)
	if e == nil || escaped {
		t.Fatalf("escaped=%v err=%v", escaped, e)
	}
}
func TestBranchRefNormalizationPreservesPullRequestIdentity(t *testing.T) {
	for _, x := range []struct{ raw, want string }{{"refs/heads/main", "main"}, {"main", "main"}, {"#42", "#42"}, {"refs/pull/42/head", "refs/pull/42/head"}} {
		t.Run(x.raw, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"id":1,"workflow_id":"test.yaml","status":"success","prettyref":%q}`, x.raw)
			}))
			defer s.Close()
			c, e := New("forge", s.URL, "", s.Client())
			if e != nil {
				t.Fatal(e)
			}
			r, e := c.GetRun(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, "1")
			if e != nil || r.Branch != x.want {
				t.Fatalf("branch=%q want=%q err=%v", r.Branch, x.want, e)
			}
		})
	}
}
func TestMalformedRunEnvelopeAndOrgInventoryRemainIncomplete(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/orgs/") {
			fmt.Fprint(w, `null`)
		} else {
			fmt.Fprint(w, `{}`)
		}
	}))
	defer s.Close()
	c, e := New("forge", s.URL, "", s.Client())
	if e != nil {
		t.Fatal(e)
	}
	_, e = c.ListRuns(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, 1)
	if e == nil {
		t.Fatal("missing run page accepted")
	}
	d, e := c.Discover(context.Background(), ci.Selection{Orgs: []string{"acme"}})
	if e != nil || d.Complete || len(d.Issues) != 1 {
		t.Fatalf("malformed inventory accepted: %+v err=%v", d, e)
	}
}
func TestForgejoDiscoveryPagesAndUnionDeduplicate(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token secret" {
			t.Error("missing auth")
		}
		repo := `{"id":42,"name":"project","full_name":"acme/project","owner":{"login":"acme"},"default_branch":"main"}`
		if strings.Contains(r.URL.Path, "/orgs/") {
			if r.URL.Query().Get("limit") != "50" {
				t.Error("unsupported page size")
			}
			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("Link", `<?page=2&limit=50>; rel="next"`)
				fmt.Fprintf(w, "[%s]", repo)
			} else {
				fmt.Fprint(w, `[{"id":43,"name":"second","owner":{"login":"acme"},"default_branch":"main"}]`)
			}
		} else {
			fmt.Fprint(w, repo)
		}
	}))
	defer s.Close()
	c, e := New("forge", s.URL+"/prefix", "secret", s.Client())
	if e != nil {
		t.Fatal(e)
	}
	d, e := c.Discover(context.Background(), ci.Selection{Orgs: []string{"acme"}, Repos: []string{"acme/project"}})
	if e != nil || !d.Complete || len(d.Repositories) != 2 || len(d.Issues) != 0 {
		t.Fatalf("discovery=%+v err=%v", d, e)
	}
}

func TestInstanceCappedRunPagesPreserveAllRuns(t *testing.T) {
	for _, total := range []int{20, 25, 34} {
		t.Run(fmt.Sprint(total), func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var page int
				fmt.Sscan(r.URL.Query().Get("page"), &page)
				if r.URL.Query().Get("limit") != "50" {
					t.Error("wrong requested limit")
				}
				fmt.Fprintf(w, `{"total_count":%d,"workflow_runs":[`, total)
				start := (page - 1) * 10
				end := min(start+10, total)
				for i := start; i < end; i++ {
					if i > start {
						fmt.Fprint(w, ",")
					}
					fmt.Fprintf(w, `{"id":%d,"workflow_id":"build.yaml","status":"success"}`, i+1)
				}
				fmt.Fprint(w, "]}")
			}))
			defer s.Close()
			c, e := New("forge", s.URL, "", s.Client())
			if e != nil {
				t.Fatal(e)
			}
			fetched := 0
			for page := 1; page <= 5; page++ {
				p, e := c.ListRuns(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, page)
				if e != nil {
					t.Fatal(e)
				}
				fetched += len(p.Runs)
				if !p.Next {
					break
				}
			}
			if fetched != total || calls != (total+9)/10 {
				t.Fatalf("instance cap dropped runs or overpolled: fetched=%d want=%d requests=%d", fetched, total, calls)
			}
		})
	}
}

func TestInstanceCappedOrgPagesPreserveCompleteDiscovery(t *testing.T) {
	for _, total := range []int{20, 25, 34} {
		t.Run(fmt.Sprint(total), func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var page int
				fmt.Sscan(r.URL.Query().Get("page"), &page)
				w.Header().Set("X-Total-Count", fmt.Sprint(total))
				fmt.Fprint(w, "[")
				start := (page - 1) * 10
				end := min(start+10, total)
				for i := start; i < end; i++ {
					if i > start {
						fmt.Fprint(w, ",")
					}
					fmt.Fprintf(w, `{"id":%d,"name":"repo-%d","owner":{"login":"acme"},"default_branch":"main"}`, i+1, i+1)
				}
				fmt.Fprint(w, "]")
			}))
			defer s.Close()
			c, e := New("forge", s.URL, "", s.Client())
			if e != nil {
				t.Fatal(e)
			}
			d, e := c.Discover(context.Background(), ci.Selection{Orgs: []string{"acme"}})
			if e != nil || !d.Complete || len(d.Repositories) != total || calls != (total+9)/10 {
				t.Fatalf("capped discovery=%+v requests=%d err=%v", d, calls, e)
			}
		})
	}
}

func TestWorkflowHistoryURLUsesFilenameInListAndGet(t *testing.T) {
	for _, tc := range []struct{ name, repositoryURL, filename, want string }{
		{"file", "https://forge.example/acme/project", "check.yml", "https://forge.example/acme/project/actions?workflow=check.yml"},
		{"instance_prefix", "https://forge.example/forge/acme/project/", "check.yaml", "https://forge.example/forge/acme/project/actions?workflow=check.yaml"},
		{"encoded_filename", "https://forge.example/forge/acme/project?tab=readme#intro", "check #?%+.yml", "https://forge.example/forge/acme/project/actions?workflow=check+%23%3F%25%2B.yml"},
		{"display_name", "https://forge.example/acme/project", "Checks", ""},
		{"numeric_identity", "https://forge.example/acme/project", "77", ""},
		{"traversal", "https://forge.example/acme/project", "../check.yml", ""},
		{"subdirectory", "https://forge.example/acme/project", "nested/check.yml", ""},
		{"backslash", "https://forge.example/acme/project", "check\\name.yml", ""},
		{"absolute_url", "https://forge.example/acme/project", "https://other.example/check.yml", ""},
		{"unsafe_repository", "javascript:alert(1)", "check.yml", ""},
		{"repository_auth", "https://user:secret@forge.example/acme/project", "check.yml", ""},
		{"repository_traversal", "https://forge.example/forge/../acme/project", "check.yml", ""},
		{"encoded_repository_traversal", "https://forge.example/forge/%2e%2e/acme/project", "check.yml", ""},
		{"missing_repository", "", "check.yml", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := fmt.Sprintf(`{"id":9,"workflow_id":%q,"title":"Friendly display name","html_url":"https://forge.example/acme/project/actions/runs/9","status":"success"}`, tc.filename)
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch r.URL.Path {
				case "/api/v1/repos/acme/project/actions/runs":
					fmt.Fprintf(w, `{"total_count":1,"workflow_runs":[%s]}`, fixture)
				case "/api/v1/repos/acme/project/actions/runs/9":
					fmt.Fprint(w, fixture)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer s.Close()
			c, err := New("forge", s.URL, "", s.Client())
			if err != nil {
				t.Fatal(err)
			}
			repo := ci.Repository{Owner: "acme", Name: "project", URL: tc.repositoryURL}
			page, err := c.ListRuns(context.Background(), repo, 1)
			if err != nil || len(page.Runs) != 1 {
				t.Fatalf("list=%#v err=%v", page, err)
			}
			detail, err := c.GetRun(context.Background(), repo, "9")
			if err != nil {
				t.Fatal(err)
			}
			for _, run := range []ci.Run{page.Runs[0], detail} {
				body, err := json.Marshal(run)
				if err != nil {
					t.Fatal(err)
				}
				var wire map[string]any
				if err := json.Unmarshal(body, &wire); err != nil {
					t.Fatal(err)
				}
				got, _ := wire["workflow_url"].(string)
				if got != tc.want || run.URL != "https://forge.example/acme/project/actions/runs/9" || run.WorkflowID != tc.filename || repo.URL != tc.repositoryURL {
					t.Errorf("workflow URL=%q want=%q run=%#v", got, tc.want, run)
				}
			}
			if calls.Load() != 2 {
				t.Errorf("workflow links fetched additional metadata: requests=%d", calls.Load())
			}
		})
	}
}
