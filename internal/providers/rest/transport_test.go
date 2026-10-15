package rest

import (
	"ciview/internal/ci"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryHeadersUseDatesAndRateReset(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		headers http.Header
		want    string
	}{{"delta", http.Header{"Retry-After": {"120"}}, "2026-01-01T00:02:00Z"}, {"date", http.Header{"Retry-After": {"Thu, 01 Jan 2026 00:03:00 GMT"}}, "2026-01-01T00:03:00Z"}, {"reset", http.Header{"X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"1767225840"}}, "2026-01-01T00:04:00Z"}, {"combined", http.Header{"Retry-After": {"120"}, "X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {"1767225840"}}, "2026-01-01T00:04:00Z"}}
	for _, x := range cases {
		t.Run(x.name, func(t *testing.T) {
			got := RetryAt(x.headers, now)
			if got == nil || got.Format(time.RFC3339) != x.want {
				t.Fatalf("retry=%v want %s", got, x.want)
			}
		})
	}
}
func TestMalformedURLsAndTimestampsCannotBecomeUsableValues(t *testing.T) {
	for _, base := range []string{"", "file:///tmp/data", "https://secret@example.net", "https://example.net/prefix?token=secret", "https://example.net/prefix#secret", "https://example.net/a/../b", "https://example.net/a%2Fb"} {
		if _, e := New("name", "forgejo", base, "token", "token", nil); e == nil {
			t.Errorf("accepted base %q", base)
		}
	}
	for _, raw := range []string{"", "bad", "0001-01-01T00:00:00Z", "1970-01-01T00:00:00Z", "1969-12-31T23:59:59Z"} {
		if Timestamp(raw) != nil {
			t.Errorf("accepted timestamp %q", raw)
		}
	}
	for _, raw := range []string{"javascript:alert(1)", "//evil.example.net", "https://secret@example.net/path", "file:///etc/passwd"} {
		if SafeURL(raw) != "" {
			t.Errorf("unsafe link %q", raw)
		}
	}
}
func TestCredentialBoundaryRejectsEncodedTraversalAndPrefixSibling(t *testing.T) {
	var calls int
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, `{}`) }))
	defer s.Close()
	tr, e := New("source", "forgejo", s.URL+"/forge/api/v1", "secret", "token", s.Client())
	if e != nil {
		t.Fatal(e)
	}
	for _, target := range []string{s.URL + "/forge/api/v10/repos", s.URL + "/forge/api/v1/%2e%2e/%2e%2e/outside", s.URL + "/forge/api/v1/../outside", s.URL + "/forge/api/v1/a%2f..%2f..%2foutside", s.URL + "/forge/api/v1/x%5cy"} {
		var out map[string]any
		if _, e := tr.Get(context.Background(), target, &out); e == nil {
			t.Errorf("unsafe target accepted %q", target)
		}
	}
	if calls != 0 {
		t.Fatalf("unsafe requests sent: %d", calls)
	}
}
func TestMalformedResponseAndNetworkErrorsStaySanitized(t *testing.T) {
	for _, body := range []string{`{"token":"private-secret",`, "private-secret"} {
		t.Run(body, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer s.Close()
			tr, _ := New("source", "github", s.URL, "private-secret", "Bearer", s.Client())
			target, _ := tr.Endpoint("/repos/acme/project", nil)
			var out map[string]any
			_, e := tr.Get(context.Background(), target, &out)
			if e == nil || strings.Contains(e.Error(), "private-secret") {
				t.Fatalf("error=%v", e)
			}
		})
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	tr, _ := New("source", "github", s.URL, "secret", "Bearer", s.Client())
	s.Close()
	target, _ := tr.Endpoint("/repos/acme/project", nil)
	var out map[string]any
	_, e := tr.Get(context.Background(), target, &out)
	var pe *ci.ProviderError
	if !errors.As(e, &pe) || strings.Contains(pe.Message, s.URL) {
		t.Fatalf("network error exposed: %v", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = tr.Get(ctx, target, &out)
	if !errors.Is(e, context.Canceled) {
		t.Fatalf("cancellation lost: %v", e)
	}
}
func TestStableIdentitySeparatesSourcesAndInstances(t *testing.T) {
	key := StableKey("source", "github", "https://example.net", "42")
	for _, other := range []ci.RepositoryKey{StableKey("other", "github", "https://example.net", "42"), StableKey("source", "forgejo", "https://example.net", "42"), StableKey("source", "github", "https://other.net", "42"), StableKey("source", "github", "https://example.net/prefix", "42"), StableKey("sour", "cegithub", "https://example.net", "42")} {
		if key == other {
			t.Fatal("identity collision")
		}
	}
	if key != StableKey("source", "github", "https://example.net", "42") {
		t.Fatal("unstable identity")
	}
}
func TestSamePrefixRedirectPreservesPrivateHeader(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/start" {
			http.Redirect(w, r, "/api/v1/done", 302)
			return
		}
		if r.Header.Get("Authorization") != "token secret" {
			t.Error("private header missing after safe redirect")
		}
		fmt.Fprint(w, `{"value":"ok"}`)
	}))
	defer s.Close()
	tr, _ := New("source", "forgejo", s.URL+"/api/v1", "secret", "token", s.Client())
	target, _ := tr.Endpoint("/start", url.Values{})
	var out map[string]string
	_, e := tr.Get(context.Background(), target, &out)
	if e != nil || out["value"] != "ok" {
		t.Fatalf("safe redirect result=%v err=%v", out, e)
	}
}

func TestEffectivePageSizeIsScopedAndUpdatesOnNewFirstPage(t *testing.T) {
	tr, e := New("source", "forgejo", "https://forge.example/api/v1", "", "token", nil)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := url.Parse("https://forge.example/api/v1/repos/acme/a/actions/runs?limit=50&page=1")
	b, _ := url.Parse("https://forge.example/api/v1/repos/acme/b/actions/runs?limit=50&page=1")
	for _, x := range []struct {
		u                  *url.URL
		page, count, total int
		next               bool
	}{{a, 1, 10, 25, true}, {b, 1, 5, 12, true}, {a, 2, 10, 25, true}, {b, 2, 5, 12, true}, {a, 3, 5, 25, false}, {b, 3, 2, 12, false}, {a, 1, 4, 10, true}, {a, 2, 4, 10, true}, {a, 3, 2, 10, false}} {
		next, e := tr.HasNext(Page{URL: x.u, Header: http.Header{}}, x.page, 50, x.count, x.total)
		if e != nil || next != x.next {
			t.Fatalf("endpoint=%s page=%d count=%d next=%v want=%v err=%v", x.u.Path, x.page, x.count, next, x.next, e)
		}
	}
}

func TestEmptyPageBeforeReportedTotalIsIncomplete(t *testing.T) {
	tr, _ := New("source", "forgejo", "https://forge.example/api/v1", "", "token", nil)
	u, _ := url.Parse("https://forge.example/api/v1/repos/acme/a/actions/runs?limit=50&page=1")
	if _, e := tr.HasNext(Page{URL: u, Header: http.Header{}}, 1, 50, 0, 20); e == nil {
		t.Fatal("empty first page incorrectly claimed complete")
	}
}

func TestConditionalGETReusesBodyAndPageMetadataThenReplacesOn200(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			if r.Header.Get("If-None-Match") != "" {
				t.Error("unexpected initial condition")
			}
			w.Header().Set("ETag", `"old"`)
			w.Header().Set("Link", `<?page=2>; rel="next"`)
			fmt.Fprint(w, `{"runs":["old"]}`)
		case 2:
			if r.Header.Get("If-None-Match") != `"old"` {
				t.Error("missing old conditional tag")
			}
			w.WriteHeader(http.StatusNotModified)
		case 3:
			if r.Header.Get("If-None-Match") != `"old"` {
				t.Error("missing prior tag before replacement")
			}
			w.Header().Set("ETag", `"new"`)
			w.Header().Set("Link", `<?page=3>; rel="next"`)
			fmt.Fprint(w, `{"runs":["new"]}`)
		case 4:
			if r.Header.Get("If-None-Match") != `"new"` {
				t.Error("replacement tag not applied")
			}
			w.WriteHeader(http.StatusNotModified)
		default:
			t.Errorf("unexpected request %d", calls)
		}
	}))
	defer s.Close()
	tr, _ := New("source", "github", s.URL, "secret", "Bearer", s.Client())
	target, _ := tr.Endpoint("/runs", url.Values{"page": {"1"}})
	for _, want := range []struct{ run, page string }{{"old", "2"}, {"old", "2"}, {"new", "3"}, {"new", "3"}} {
		var body struct {
			Runs []string `json:"runs"`
		}
		meta, e := tr.Get(context.Background(), target, &body)
		if e != nil || len(body.Runs) != 1 || body.Runs[0] != want.run {
			t.Fatalf("conditional body=%+v want=%s err=%v", body, want.run, e)
		}
		next, e := tr.NextLink(meta)
		u, _ := url.Parse(next)
		if e != nil || u.Query().Get("page") != want.page {
			t.Fatalf("cached page metadata lost: next=%s err=%v", next, e)
		}
		meta.Header.Set("Link", `<https://evil.example>; rel="next"`)
	}
}

func TestConditionalGETNeverCachesErrorAndSourcesStayIsolated(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			w.Header().Set("ETag", `"error"`)
			http.Error(w, "private-secret", 503)
		case 2:
			if r.Header.Get("If-None-Match") != "" {
				t.Error("error response was cached")
			}
			w.Header().Set("ETag", `"success"`)
			fmt.Fprint(w, `{"value":"ok"}`)
		case 3:
			if r.Header.Get("If-None-Match") != `"success"` {
				t.Error("successful response not cached")
			}
			w.WriteHeader(http.StatusNotModified)
		case 4:
			if r.Header.Get("If-None-Match") != "" || r.Header.Get("Authorization") != "Bearer other-secret" {
				t.Error("source cache or credential contamination")
			}
			w.Header().Set("ETag", `"other"`)
			fmt.Fprint(w, `{"value":"other"}`)
		}
	}))
	defer s.Close()
	first, _ := New("first", "github", s.URL, "first-secret", "Bearer", s.Client())
	second, _ := New("second", "github", s.URL, "other-secret", "Bearer", s.Client())
	target, _ := first.Endpoint("/private", nil)
	var body map[string]string
	if _, e := first.Get(context.Background(), target, &body); e == nil {
		t.Fatal("upstream error hidden")
	}
	for n := 0; n < 2; n++ {
		if _, e := first.Get(context.Background(), target, &body); e != nil || body["value"] != "ok" {
			t.Fatalf("successful cache=%v err=%v", body, e)
		}
	}
	if _, e := second.Get(context.Background(), target, &body); e != nil || body["value"] != "other" {
		t.Fatalf("source cache contaminated=%v err=%v", body, e)
	}
}

func TestUnsolicited304CannotBecomeEmptySuccess(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotModified) }))
	defer s.Close()
	tr, _ := New("source", "github", s.URL, "", "Bearer", s.Client())
	target, _ := tr.Endpoint("/runs", nil)
	var body map[string]string
	if _, e := tr.Get(context.Background(), target, &body); e == nil {
		t.Fatal("uncached304 accepted")
	}
}

func TestConditionalCacheBoundsEntriesAndBytes(t *testing.T) {
	for _, kind := range []string{"entries", "bytes"} {
		t.Run(kind, func(t *testing.T) {
			body := `{"value":"ok"}`
			count := 129
			if kind == "bytes" {
				body = `{"value":"` + strings.Repeat("x", 4<<20) + `"}`
				count = 2
			}
			replay := false
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if replay && r.Header.Get("If-None-Match") != "" {
					t.Error("bounded cache retained evicted first response")
				}
				w.Header().Set("ETag", `"tag"`)
				fmt.Fprint(w, body)
			}))
			defer s.Close()
			tr, _ := New("source", "github", s.URL, "", "Bearer", s.Client())
			for n := 0; n < count; n++ {
				target, _ := tr.Endpoint(fmt.Sprintf("/runs/%d", n), nil)
				var out map[string]string
				if _, e := tr.Get(context.Background(), target, &out); e != nil {
					t.Fatal(e)
				}
			}
			replay = true
			target, _ := tr.Endpoint("/runs/0", nil)
			var out map[string]string
			if _, e := tr.Get(context.Background(), target, &out); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestMalformedSuccessfulResponseDoesNotCacheETag(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("If-None-Match") != "" {
			t.Error("malformed response cached")
		}
		w.Header().Set("ETag", `"malformed"`)
		if calls == 1 {
			fmt.Fprint(w, `{"broken":`)
		} else {
			fmt.Fprint(w, `{"ok":true}`)
		}
	}))
	defer s.Close()
	tr, _ := New("source", "github", s.URL, "", "Bearer", s.Client())
	target, _ := tr.Endpoint("/runs", nil)
	var out map[string]any
	if _, e := tr.Get(context.Background(), target, &out); e == nil {
		t.Fatal("malformed response accepted")
	}
	if _, e := tr.Get(context.Background(), target, &out); e != nil || out["ok"] != true {
		t.Fatalf("retry=%v err=%v", out, e)
	}
}

func TestConcurrentConditionalResponsesOwnTheirDecodedValues(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"tag"`)
		fmt.Fprint(w, `{"runs":["original"]}`)
	}))
	defer s.Close()
	tr, _ := New("source", "github", s.URL, "", "Bearer", s.Client())
	target, _ := tr.Endpoint("/runs", nil)
	var seed map[string][]string
	if _, e := tr.Get(context.Background(), target, &seed); e != nil {
		t.Fatal(e)
	}
	done := make(chan bool, 20)
	for n := 0; n < 20; n++ {
		go func() {
			var out map[string][]string
			meta, e := tr.Get(context.Background(), target, &out)
			if e != nil || len(out["runs"]) != 1 || out["runs"][0] != "original" {
				t.Errorf("concurrent cache=%v err=%v", out, e)
			}
			if len(out["runs"]) > 0 {
				out["runs"][0] = "changed"
			}
			meta.Header.Set("ETag", `"changed"`)
			done <- true
		}()
	}
	for n := 0; n < 20; n++ {
		<-done
	}
}

func TestExplicitDiscoveryStopsAtRetryDeadlineAndKeepsPartialInventory(t *testing.T) {
	for _, status := range []int{429, 403, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch r.URL.Path {
				case "/repos/acme/first":
					fmt.Fprint(w, `{"id":42,"name":"first","owner":{"login":"acme"},"default_branch":"main"}`)
				case "/repos/acme/limited":
					w.Header().Set("Retry-After", "3600")
					w.WriteHeader(status)
					fmt.Fprint(w, `{"message":"private-secret"}`)
				default:
					t.Errorf("discovery continued after cooldown: %s", r.URL.Path)
					fmt.Fprint(w, `{"id":43,"name":"third","owner":{"login":"acme"}}`)
				}
			}))
			defer s.Close()
			tr, e := New("source", "github", s.URL, "secret", "Bearer", s.Client())
			if e != nil {
				t.Fatal(e)
			}
			before := time.Now().UTC()
			d, e := tr.Discover(context.Background(), ci.Selection{Repos: []string{"acme/first", "acme/limited", "acme/third"}}, 100)
			var pe *ci.ProviderError
			wantKind := "rate_limit"
			if status == 503 {
				wantKind = "unavailable"
			}
			if !errors.As(e, &pe) || pe.Kind != wantKind || pe.RetryAt == nil || pe.RetryAt.Before(before.Add(3599*time.Second)) {
				t.Fatalf("cooldown error discarded: discovery=%+v err=%v", d, e)
			}
			if d.Complete || len(d.Repositories) != 1 || d.Repositories[0].ID != "42" || len(d.Issues) != 1 || d.Issues[0].Repository != "acme/limited" || calls.Load() != 2 {
				t.Fatalf("partial inventory or stop lost: discovery=%+v requests=%d", d, calls.Load())
			}
		})
	}
}

func TestOrgDiscoveryRetryDeadlineStopsFurtherPagesAndSelections(t *testing.T) {
	for _, status := range []int{429, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path == "/orgs/acme/repos" && r.URL.Query().Get("page") == "1" {
					w.Header().Set("Link", `<?page=2&limit=50>; rel="next"`)
					fmt.Fprint(w, `[{"id":42,"name":"first","owner":{"login":"acme"},"default_branch":"main"}]`)
					return
				}
				if r.URL.Path == "/orgs/acme/repos" && r.URL.Query().Get("page") == "2" {
					w.Header().Set("Retry-After", "3600")
					w.WriteHeader(status)
					fmt.Fprint(w, `{"message":"private-secret"}`)
					return
				}
				t.Errorf("selection requested after org rate limit: %s", r.URL)
				fmt.Fprint(w, `[]`)
			}))
			defer s.Close()
			tr, e := New("source", "forgejo", s.URL, "secret", "token", s.Client())
			if e != nil {
				t.Fatal(e)
			}
			before := time.Now().UTC()
			d, e := tr.Discover(context.Background(), ci.Selection{Orgs: []string{"acme", "another"}, Repos: []string{"acme/explicit"}}, 50)
			var pe *ci.ProviderError
			wantKind := "rate_limit"
			if status == 503 {
				wantKind = "unavailable"
			}
			if !errors.As(e, &pe) || pe.Kind != wantKind || pe.RetryAt == nil || pe.RetryAt.Before(before.Add(3599*time.Second)) {
				t.Fatalf("org cooldown lost: discovery=%+v err=%v", d, e)
			}
			if d.Complete || len(d.Repositories) != 1 || d.Repositories[0].ID != "42" || len(d.Issues) != 1 || d.Issues[0].Repository != "acme" || calls.Load() != 2 {
				t.Fatalf("org partial inventory lost: %+v requests=%d", d, calls.Load())
			}
		})
	}
}

func TestNonRateDiscoverySelectionErrorsRemainIssuesAndContinue(t *testing.T) {
	for _, status := range []int{403, 404, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path == "/repos/acme/unavailable" {
					if status == 503 {
						w.Header().Set("Retry-After", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat))
					}
					w.WriteHeader(status)
					fmt.Fprint(w, `{"message":"private-secret"}`)
					return
				}
				fmt.Fprint(w, `{"id":42,"name":"first","owner":{"login":"acme"}}`)
			}))
			defer s.Close()
			tr, _ := New("source", "github", s.URL, "secret", "Bearer", s.Client())
			d, e := tr.Discover(context.Background(), ci.Selection{Repos: []string{"acme/unavailable", "acme/first"}}, 100)
			if e != nil || d.Complete || len(d.Repositories) != 1 || len(d.Issues) != 1 || calls.Load() != 2 {
				t.Fatalf("non-rate error stopped partial discovery: %+v requests=%d err=%v", d, calls.Load(), e)
			}
		})
	}
}
