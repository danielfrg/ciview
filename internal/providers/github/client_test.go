package github

import (
	"civiewer/internal/ci"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func repoFixture() string {
	return `{"id":42,"name":"project","full_name":"acme/project","owner":{"login":"acme"},"default_branch":"main","html_url":"https://github.com/acme/project"}`
}
func testClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	c, e := New("private", s.URL, "private-secret", s.Client())
	if e != nil {
		t.Fatal(e)
	}
	return c, s
}
func TestDiscoveryUnionPaginationAndPartialFailure(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-secret" || r.Header.Get("X-GitHub-Api-Version") == "" {
			t.Errorf("missing private auth/API version")
		}
		switch r.URL.Path {
		case "/orgs/acme/repos":
			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("Link", `</orgs/acme/repos?page=2&per_page=100>; rel="next"`)
				fmt.Fprintf(w, "[%s]", repoFixture())
			} else {
				http.Error(w, "private-secret internal details", 500)
			}
		case "/repos/acme/project":
			fmt.Fprint(w, repoFixture())
		case "/repos/acme/missing":
			http.Error(w, "private-secret", 404)
		default:
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
		}
	})
	d, e := c.Discover(context.Background(), ci.Selection{Orgs: []string{"acme"}, Repos: []string{"acme/project", "acme/missing"}})
	if e != nil || d.Complete || len(d.Repositories) != 1 || len(d.Issues) != 2 {
		t.Fatalf("discovery=%+v err=%v", d, e)
	}
	p := d.Repositories[0]
	if p.ID != "42" || p.DefaultBranch != "main" || p.Source != "private" || p.Key == "" {
		t.Fatalf("metadata=%+v", p)
	}
	for _, issue := range d.Issues {
		if strings.Contains(issue.Message, "private-secret") {
			t.Fatal("secret disclosed")
		}
	}
}
func TestRunsNormalizeRerunsAndStatuses(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-secret" {
			t.Error("missing auth")
		}
		fmt.Fprint(w, `{"total_count":102,"workflow_runs":[{"id":9,"workflow_id":77,"name":"","path":".github/workflows/ci.yml","display_title":"build","head_branch":"main","event":"push","html_url":"https://github.com/acme/project/actions/runs/9","run_attempt":2,"status":"completed","conclusion":"action_required","created_at":"2026-01-02T12:00:00+02:00","run_started_at":"1970-01-01T00:00:00Z","updated_at":"2026-01-02T12:30:00Z"}]}`)
	})
	p, e := c.ListRuns(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, 1)
	if e != nil || len(p.Runs) != 1 || !p.Next {
		t.Fatalf("page=%+v err=%v", p, e)
	}
	r := p.Runs[0]
	if r.ID != "9" || r.WorkflowID != "77" || r.WorkflowName != ".github/workflows/ci.yml" || r.State != "attention" || !r.Terminal || r.Attempt != 2 || r.StartedAt != nil || r.FinishedAt != nil || r.DurationMS != nil || r.CreatedAt.Format(time.RFC3339) != "2026-01-02T10:00:00Z" {
		t.Fatalf("run=%+v", r)
	}
}
func TestStatePreservesUnknownAndTerminality(t *testing.T) {
	cases := []struct {
		status, conclusion, state string
		terminal                  bool
	}{{"completed", "success", "success", true}, {"completed", "timed_out", "failure", true}, {"completed", "cancelled", "canceled", true}, {"completed", "skipped", "skipped", true}, {"completed", "neutral", "neutral", true}, {"completed", "stale", "attention", true}, {"completed", "alien", "unknown", true}, {"completed", "", "unknown", true}, {"in_progress", "", "running", false}, {"waiting", "", "waiting", false}, {"pending", "", "queued", false}, {"alien", "success", "unknown", false}}
	for _, x := range cases {
		t.Run(x.status+"/"+x.conclusion, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"id":8,"workflow_id":1,"status":%q,"conclusion":%q}`, x.status, x.conclusion)
			})
			r, e := c.GetRun(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, "8")
			if e != nil || r.State != x.state || r.Terminal != x.terminal || r.RawStatus != x.status || r.RawConclusion != x.conclusion {
				t.Fatalf("run=%+v err=%v", r, e)
			}
		})
	}
}
func TestDurationAttemptSpecificPaginationAndCache(t *testing.T) {
	var calls atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if !strings.Contains(r.URL.Path, "/attempts/2/jobs") && !strings.Contains(r.URL.Path, "/attempts/3/jobs") {
			t.Errorf("wrong attempt path %s", r.URL.Path)
		}
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", `<?page=2&per_page=100>; rel="next"`)
			fmt.Fprint(w, `{"total_count":3,"jobs":[{"id":1,"status":"completed","conclusion":"success","started_at":"2026-01-01T00:00:01Z","completed_at":"2026-01-01T00:00:03Z"},{"id":2,"status":"completed","conclusion":"skipped","started_at":null,"completed_at":null}]}`)
		} else {
			fmt.Fprint(w, `{"total_count":3,"jobs":[{"id":3,"status":"completed","conclusion":"cancelled","started_at":"2026-01-01T00:00:02Z","completed_at":"2026-01-01T00:00:05Z"}]}`)
		}
	})
	repo := ci.Repository{Key: "repo", Owner: "acme", Name: "project"}
	r := ci.Run{ID: "9", Attempt: 2, Terminal: true, State: "success"}
	got, e := c.EnrichDuration(context.Background(), repo, r)
	if e != nil || got.DurationMS == nil || *got.DurationMS != 4000 || got.DurationKind != "job_span" || got.FinishedAt == nil {
		t.Fatalf("run=%+v err=%v", got, e)
	}
	got.DurationMS = new(int64)
	*got.DurationMS = 99
	again, e := c.EnrichDuration(context.Background(), repo, r)
	if e != nil || again.DurationMS == nil || *again.DurationMS != 4000 || calls.Load() != 2 {
		t.Fatalf("cache leaked or missing: %+v calls=%d err=%v", again, calls.Load(), e)
	}
	r.Attempt = 3
	r.DurationMS = again.DurationMS
	r.FinishedAt = again.FinishedAt
	_, e = c.EnrichDuration(context.Background(), repo, r)
	if e != nil || calls.Load() != 4 {
		t.Fatalf("rerun reused attempt: calls=%d err=%v", calls.Load(), e)
	}
}
func TestIncompleteTimingRemainsUnavailable(t *testing.T) {
	cases := []struct {
		name, body string
		next       bool
	}{{"empty", `{"jobs":[]}`, false}, {"missing", `{"jobs":[{"id":1,"status":"completed","conclusion":"success","started_at":"2026-01-01T00:00:01Z","completed_at":null}]}`, false}, {"skipped", `{"jobs":[{"id":1,"status":"completed","conclusion":"skipped"}]}`, false}, {"canceled_without_start", `{"jobs":[{"id":1,"status":"completed","conclusion":"cancelled"}]}`, false}, {"truncated", `{"total_count":501,"jobs":[{"id":1,"status":"completed","conclusion":"success","started_at":"2026-01-01T00:00:01Z","completed_at":"2026-01-01T00:00:03Z"}]}`, true}, {"short_missing_page", `{"total_count":2,"jobs":[{"id":1,"status":"completed","conclusion":"success","started_at":"2026-01-01T00:00:01Z","completed_at":"2026-01-01T00:00:03Z"}]}`, false}}
	for _, x := range cases {
		t.Run(x.name, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if x.next {
					w.Header().Set("Link", `<?page=2>; rel="next"`)
				}
				fmt.Fprint(w, x.body)
			})
			old := int64(88)
			r, e := c.EnrichDuration(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, ci.Run{ID: "9", Attempt: 2, Terminal: true, DurationMS: &old})
			if e != nil || r.DurationMS != nil {
				t.Fatalf("run=%+v err=%v", r, e)
			}
		})
	}
}
func TestSafeErrorsRetriesAndRedirects(t *testing.T) {
	for _, status := range []int{401, 403, 404, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "120")
				w.WriteHeader(status)
				fmt.Fprint(w, `{"message":"private-secret private payload"}`)
			})
			_, e := c.ListRuns(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, 1)
			var pe *ci.ProviderError
			if !errors.As(e, &pe) || strings.Contains(e.Error(), "private-secret") {
				t.Fatalf("unsafe err=%v", e)
			}
			if (status == 429 || status == 403) && pe.RetryAt == nil {
				t.Fatalf("missing retry %v", e)
			}
		})
	}
	var leaked atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Add(1)
		json.NewEncoder(w).Encode(map[string]any{"workflow_runs": []any{}})
	}))
	defer other.Close()
	for _, kind := range []string{"redirect", "pagination"} {
		t.Run(kind, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if kind == "redirect" {
					http.Redirect(w, r, other.URL+"/secret", 302)
				} else {
					w.Header().Set("Link", "<"+other.URL+"/secret>; rel=\"next\"")
					fmt.Fprint(w, `{"workflow_runs":[]}`)
				}
			})
			_, e := c.ListRuns(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, 1)
			if e == nil || leaked.Load() != 0 {
				t.Fatalf("unsafe request err=%v requests=%d", e, leaked.Load())
			}
		})
	}
}
func TestInvalidCanceledJobTimestampsCannotHideAnExecutedJob(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"total_count":2,"jobs":[{"id":1,"status":"completed","conclusion":"success","started_at":"2026-01-01T00:00:01Z","completed_at":"2026-01-01T00:00:03Z"},{"id":2,"status":"completed","conclusion":"cancelled","started_at":"bad-timestamp","completed_at":null}]}`)
	})
	r, e := c.EnrichDuration(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, ci.Run{ID: "9", Attempt: 2, Terminal: true})
	if e != nil || r.DurationMS != nil {
		t.Fatalf("invalid canceled timing accepted: %+v err=%v", r, e)
	}
}
func TestEnrichmentFailurePreservesInputRun(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "secret", 503) })
	duration := int64(99)
	input := ci.Run{ID: "9", Attempt: 2, Terminal: true, State: "success", DurationMS: &duration}
	r, e := c.EnrichDuration(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, input)
	if e == nil || r.ID != "9" || r.State != "success" || r.DurationMS != input.DurationMS || *r.DurationMS != 99 {
		t.Fatalf("changed history on error: %+v err=%v", r, e)
	}
}
func TestMalformedRunEnvelopeCannotEraseHistory(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"workflow_runs":null}`, `{"message":"not a run page"}`} {
		t.Run(body, func(t *testing.T) {
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
			_, e := c.ListRuns(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, 1)
			if e == nil {
				t.Fatalf("malformed run envelope accepted: %s", body)
			}
		})
	}
}
func TestConcurrentTimingCacheValuesStayIndependent(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"total_count":1,"jobs":[{"id":1,"status":"completed","conclusion":"success","started_at":"2026-01-01T00:00:01Z","completed_at":"2026-01-01T00:00:03Z"}]}`)
	})
	done := make(chan bool, 20)
	for n := 0; n < 20; n++ {
		go func() {
			r, e := c.EnrichDuration(context.Background(), ci.Repository{Key: "repo", Owner: "acme", Name: "project"}, ci.Run{ID: "9", Attempt: 2, Terminal: true})
			if e != nil || r.DurationMS == nil || *r.DurationMS != 2000 {
				t.Errorf("timing=%+v err=%v", r, e)
			}
			if r.DurationMS != nil {
				*r.DurationMS = 1
			}
			done <- true
		}()
	}
	for n := 0; n < 20; n++ {
		<-done
	}
}
func TestFiveCompleteJobPagesProduceTimingAndSixthIsNeverRequested(t *testing.T) {
	var calls atomic.Int32
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		page := r.URL.Query().Get("page")
		var n int
		fmt.Sscan(page, &n)
		if n < 1 || n > 5 {
			t.Fatalf("unbounded job pagination %d", n)
		}
		fmt.Fprint(w, `{"total_count":500,"jobs":[`)
		for i := 0; i < 100; i++ {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, `{"id":%d,"status":"completed","conclusion":"success","started_at":"2026-01-01T00:00:01Z","completed_at":"2026-01-01T00:00:05Z"}`, ((n-1)*100)+i+1)
		}
		fmt.Fprint(w, "]}")
	})
	r, e := c.EnrichDuration(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, ci.Run{ID: "9", Attempt: 2, Terminal: true})
	if e != nil || r.DurationMS == nil || *r.DurationMS != 4000 || calls.Load() != 5 {
		t.Fatalf("complete job pagination unavailable: %+v calls=%d err=%v", r, calls.Load(), e)
	}
}
func TestNumericIdentitiesPreserveLargeIntegers(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":9007199254740993,"workflow_id":9007199254740995,"status":"completed","conclusion":"success"}`)
	})
	r, e := c.GetRun(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, "9007199254740993")
	if e != nil || r.ID != "9007199254740993" || r.WorkflowID != "9007199254740995" {
		t.Fatalf("numeric identity rounded: %+v err=%v", r, e)
	}
}

func TestMalformedJobCompletenessNeverCachesInferredTiming(t *testing.T) {
	const validJobs = `"jobs":[{"id":1,"status":"completed","conclusion":"success","started_at":"2026-01-01T00:00:01Z","completed_at":"2026-01-01T00:00:05Z"}]`
	for _, body := range []string{"{" + validJobs + "}", `{"total_count":null,` + validJobs + "}", `{"total_count":-1,` + validJobs + "}", `{"total_count":1}`, `{"total_count":1,"jobs":null}`} {
		t.Run(body, func(t *testing.T) {
			calls := 0
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 {
					fmt.Fprint(w, body)
				} else {
					fmt.Fprint(w, `{"total_count":1,`+validJobs+"}")
				}
			})
			repo := ci.Repository{Key: "repo", Owner: "acme", Name: "project"}
			input := ci.Run{ID: "9", Attempt: 2, Terminal: true, State: "success"}
			run, e := c.EnrichDuration(context.Background(), repo, input)
			if e != nil || run.DurationMS != nil || run.ID != "9" || run.State != "success" {
				t.Fatalf("malformed completeness accepted: %+v err=%v", run, e)
			}
			run, e = c.EnrichDuration(context.Background(), repo, input)
			if e != nil || run.DurationMS == nil || *run.DurationMS != 4000 || calls != 2 {
				t.Fatalf("malformed timing cached: %+v requests=%d err=%v", run, calls, e)
			}
		})
	}
}

func TestEveryJobPageRequiresCompletenessMetadata(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", `<?page=2&per_page=100>; rel="next"`)
			fmt.Fprint(w, `{"total_count":2,"jobs":[{"id":1,"status":"completed","conclusion":"success","started_at":"2026-01-01T00:00:01Z","completed_at":"2026-01-01T00:00:03Z"}]}`)
		} else {
			fmt.Fprint(w, `{"jobs":[{"id":2,"status":"completed","conclusion":"success","started_at":"2026-01-01T00:00:02Z","completed_at":"2026-01-01T00:00:05Z"}]}`)
		}
	})
	run, e := c.EnrichDuration(context.Background(), ci.Repository{Owner: "acme", Name: "project"}, ci.Run{ID: "9", Attempt: 2, Terminal: true, State: "success"})
	if e != nil || run.DurationMS != nil || run.State != "success" {
		t.Fatalf("second page missing count accepted: %+v err=%v", run, e)
	}
}

func TestConditionalRunRefreshPreservesHistoryOn304(t *testing.T) {
	calls := 0
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("ETag", `"run-page"`)
			fmt.Fprint(w, `{"total_count":1,"workflow_runs":[{"id":9,"workflow_id":77,"name":"CI","status":"completed","conclusion":"success","created_at":"2026-01-01T00:00:00Z"}]}`)
		} else {
			if r.Header.Get("If-None-Match") != `"run-page"` {
				t.Error("run refresh omitted conditional tag")
			}
			w.WriteHeader(http.StatusNotModified)
		}
	})
	repo := ci.Repository{Owner: "acme", Name: "project"}
	for n := 0; n < 2; n++ {
		page, e := c.ListRuns(context.Background(), repo, 1)
		if e != nil || len(page.Runs) != 1 || page.Runs[0].ID != "9" || page.Runs[0].State != "success" || page.Next {
			t.Fatalf("conditional refresh lost run history: %+v err=%v", page, e)
		}
	}
}

func TestListAndGetRunPreserveHeadRepositoryIdentity(t *testing.T) {
	for _, tc := range []struct{ name, head, want string }{
		{"local", `{"id":42}`, "42"},
		{"foreign", `{"id":99}`, "99"},
		{"unknown", `null`, ""},
		{"zero", `{"id":0}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := fmt.Sprintf(`{"id":9,"workflow_id":77,"head_branch":"main","head_repository":%s,"repository":{"id":42},"event":"pull_request","html_url":"https://github.com/acme/project/actions/runs/9","status":"completed","conclusion":"action_required"}`, tc.head)
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/repos/acme/project/actions/runs":
					fmt.Fprintf(w, `{"total_count":1,"workflow_runs":[%s]}`, fixture)
				case "/repos/acme/project/actions/runs/9":
					fmt.Fprint(w, fixture)
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			repo := ci.Repository{ID: "42", Owner: "acme", Name: "project"}
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
				got, _ := wire["head_repository_id"].(string)
				if got != tc.want || run.Branch != "main" || run.URL != "https://github.com/acme/project/actions/runs/9" || run.Event != "pull_request" || run.State != "attention" || !run.Terminal {
					t.Fatalf("provenance or raw PR identity lost: head=%q want=%q run=%#v", got, tc.want, run)
				}
			}
		})
	}
}

func TestWorkflowHistoryURLUsesPathInListAndGet(t *testing.T) {
	for _, tc := range []struct{ name, repositoryURL, workflowPath, want string }{
		{"file", "https://github.com/acme/project", ".github/workflows/check.yml", "https://github.com/acme/project/actions/workflows/check.yml"},
		{"instance_prefix", "https://github.example/git/acme/project/", ".github/workflows/check.yaml", "https://github.example/git/acme/project/actions/workflows/check.yaml"},
		{"encoded_filename", "https://github.example/git/acme/project?tab=readme#intro", ".github/workflows/check #?%+.yml", "https://github.example/git/acme/project/actions/workflows/check%20%23%3F%25+.yml"},
		{"missing_path", "https://github.com/acme/project", "", ""},
		{"display_name", "https://github.com/acme/project", "Checks", ""},
		{"generated", "https://github.com/acme/project", "dynamic/pages/pages-build-deployment", ""},
		{"numeric_path", "https://github.com/acme/project", "77", ""},
		{"different_directory", "https://github.com/acme/project", ".gitlab/workflows/check.yml", ""},
		{"traversal", "https://github.com/acme/project", ".github/workflows/../check.yml", ""},
		{"subdirectory", "https://github.com/acme/project", ".github/workflows/nested/check.yml", ""},
		{"backslash", "https://github.com/acme/project", ".github/workflows/check\\name.yml", ""},
		{"absolute_url", "https://github.com/acme/project", "https://other.example/check.yml", ""},
		{"unsafe_repository", "javascript:alert(1)", ".github/workflows/check.yml", ""},
		{"repository_auth", "https://user:secret@github.example/acme/project", ".github/workflows/check.yml", ""},
		{"repository_traversal", "https://github.example/git/../acme/project", ".github/workflows/check.yml", ""},
		{"encoded_repository_traversal", "https://github.example/git/%2e%2e/acme/project", ".github/workflows/check.yml", ""},
		{"missing_repository", "", ".github/workflows/check.yml", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := fmt.Sprintf(`{"id":9,"workflow_id":77,"name":"Friendly display name","path":%q,"html_url":"https://github.com/acme/project/actions/runs/9","status":"completed","conclusion":"success"}`, tc.workflowPath)
			var calls atomic.Int32
			c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch r.URL.Path {
				case "/repos/acme/project/actions/runs":
					fmt.Fprintf(w, `{"total_count":1,"workflow_runs":[%s]}`, fixture)
				case "/repos/acme/project/actions/runs/9":
					fmt.Fprint(w, fixture)
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					http.NotFound(w, r)
				}
			})
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
				if got != tc.want || run.URL != "https://github.com/acme/project/actions/runs/9" || run.WorkflowID != "77" || repo.URL != tc.repositoryURL {
					t.Errorf("workflow URL=%q want=%q run=%#v", got, tc.want, run)
				}
			}
			if calls.Load() != 2 {
				t.Errorf("workflow links fetched additional metadata: requests=%d", calls.Load())
			}
		})
	}
}
