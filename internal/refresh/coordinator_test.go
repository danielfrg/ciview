package refresh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"ciview/internal/ci"
	"ciview/internal/config"
	"ciview/internal/snapshots"
)

type controlledProvider struct {
	mu                       sync.Mutex
	discovery                ci.Discovery
	discoveryErr             error
	discoverFn               func(context.Context, ci.Selection) (ci.Discovery, error)
	pages                    map[int]ci.RunPage
	listErr                  error
	got                      map[string]ci.Run
	enrichErr                error
	lists, discoveries, gets int
	listFn                   func(context.Context, ci.Repository, int) (ci.RunPage, error)
	enrichedIDs              []string
}

func (p *controlledProvider) Discover(ctx context.Context, s ci.Selection) (ci.Discovery, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.discoveries++
	if p.discoverFn != nil {
		return p.discoverFn(ctx, s)
	}
	return p.discovery, p.discoveryErr
}
func (p *controlledProvider) ListRuns(ctx context.Context, r ci.Repository, n int) (ci.RunPage, error) {
	p.mu.Lock()
	p.lists++
	f := p.listFn
	v, e := p.pages[n], p.listErr
	p.mu.Unlock()
	if f != nil {
		return f(ctx, r, n)
	}
	return v, e
}
func (p *controlledProvider) GetRun(ctx context.Context, r ci.Repository, id string) (ci.Run, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gets++
	return p.got[id], nil
}
func (p *controlledProvider) EnrichDuration(ctx context.Context, r ci.Repository, v ci.Run) (ci.Run, error) {
	p.mu.Lock()
	p.enrichedIDs = append(p.enrichedIDs, v.ID)
	p.mu.Unlock()
	if p.enrichErr != nil {
		return ci.Run{}, p.enrichErr
	}
	return v, nil
}
func testConfig() config.Config {
	return config.Config{Viewer: config.Viewer{RefreshInterval: "5s", DiscoveryInterval: "15m", HistoryRuns: 2, HistoryPages: 2, Branch: "default"}, Sources: []config.Source{{Name: "public", Provider: "github", Repos: []string{"example/project"}}}}
}
func repo() ci.Repository {
	return ci.Repository{Key: "opaque", Source: "public", Provider: "github", FullName: "example/project", DefaultBranch: "main"}
}
func run(id string, minute int, terminal bool) ci.Run {
	return ci.Run{ID: id, Attempt: 1, WorkflowID: "build", WorkflowName: "Build", Branch: "main", CreatedAt: time.Date(2026, 1, 1, 0, minute, 0, 0, time.UTC), Terminal: terminal, State: "running"}
}
func provider() *controlledProvider {
	return &controlledProvider{discovery: ci.Discovery{Complete: true, Repositories: []ci.Repository{repo()}}, pages: map[int]ci.RunPage{1: {Runs: []ci.Run{run("1", 1, true)}}}, got: map[string]ci.Run{}}
}
func setup(p *controlledProvider) (*Coordinator, *snapshots.Memory) {
	m := snapshots.NewMemory()
	return New(testConfig(), map[string]ci.Provider{"public": p}, m), m
}
func inventory(t *testing.T, c *Coordinator) ci.SourceState {
	t.Helper()
	v, e := c.Inventory(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	return v.Sources[0]
}
func read(t *testing.T, m *snapshots.Memory) ci.RepositorySnapshot {
	t.Helper()
	v, ok, e := m.Read(context.Background(), repo().Key)
	if e != nil || !ok {
		t.Fatalf("missing snapshot: %v", e)
	}
	return v
}

func TestInitialInventoryLoadingAndOwned(t *testing.T) {
	c, _ := setup(provider())
	v := inventory(t, c)
	if v.State != "loading" || len(v.Issues) != 1 || v.Issues[0].Repository != "example/project" {
		t.Fatalf("initial: %#v", v)
	}
	v.Issues[0].Message = "mutation"
	if inventory(t, c).Issues[0].Message == "mutation" {
		t.Fatal("shared issues")
	}
}
func TestDiscoveryIncompletePreservesInventoryAndRemovalExcludesSnapshot(t *testing.T) {
	p := provider()
	c, m := setup(p)
	ctx := context.Background()
	if e := c.Refresh(ctx); e != nil {
		t.Fatal(e)
	}
	p.discovery = ci.Discovery{Complete: false, Issues: []ci.Issue{{Kind: "permission", Message: "unavailable selection"}}}
	c.now = func() time.Time { return time.Now().Add(time.Hour) }
	if e := c.Refresh(ctx); e != nil {
		t.Fatal(e)
	}
	if len(inventory(t, c).Repositories) != 1 {
		t.Fatal("partial discovery cleared known repository")
	}
	p.discovery = ci.Discovery{Complete: true}
	c.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	c.Refresh(ctx)
	if len(inventory(t, c).Repositories) != 0 {
		t.Fatal("removed repository still active")
	}
	read(t, m)
}
func TestScanFailureRetainsStaleRunsAndSourceNotice(t *testing.T) {
	p := provider()
	c, m := setup(p)
	ctx := context.Background()
	c.Refresh(ctx)
	p.listErr = errors.New("private raw token")
	c.Refresh(ctx)
	v := read(t, m)
	if len(v.Runs) != 1 || v.Health.State != "stale" || v.Health.LastSuccessAt == nil {
		t.Fatalf("stale: %#v", v)
	}
	s := inventory(t, c)
	if s.State != "stale" || len(s.Issues) == 0 || s.Issues[0].Repository != "example/project" || s.Issues[0].Message == "private raw token" {
		t.Fatalf("source: %#v", s)
	}
}
func TestInitialPermissionFailureAndNoObservedBranchAreVisible(t *testing.T) {
	p := provider()
	p.listErr = &ci.ProviderError{Kind: "permission", Message: "raw token"}
	c, m := setup(p)
	c.Refresh(context.Background())
	s := inventory(t, c)
	if s.State != "unavailable" || len(s.Issues) == 0 || s.Issues[0].Kind != "permission" {
		t.Fatalf("permissions: %#v", s)
	}
	if len(read(t, m).Runs) != 0 {
		t.Fatal("fabricated runs")
	}
	p.listErr = nil
	p.pages = map[int]ci.RunPage{1: {Runs: []ci.Run{{ID: "other", Branch: "feature", CreatedAt: time.Now(), Terminal: true}}, Next: true}, 2: {Next: true}}
	c.Refresh(context.Background())
	s = inventory(t, c)
	if len(s.Issues) == 0 || s.Issues[0].Repository != "example/project" || s.Issues[0].Kind != "no_observed_runs" {
		t.Fatalf("missing empty scope notice: %#v", s)
	}
	if !read(t, m).Coverage.Partial {
		t.Fatal("lost limited coverage")
	}
}
func TestLimitedHistoryNoticeSurvivesEmptyBranchScope(t *testing.T) {
	for _, tc := range []struct {
		name      string
		branch    string
		partial   bool
		wantKinds []string
		wantRuns  int
	}{
		{name: "empty limited sample", branch: "feature", partial: true, wantKinds: []string{"no_observed_runs", "limited_history"}},
		{name: "empty complete sample", branch: "feature", wantKinds: []string{"no_observed_runs"}},
		{name: "nonempty limited sample", branch: "main", partial: true, wantRuns: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := provider()
			first, second := run("1", 1, true), run("2", 2, true)
			first.Branch, second.Branch = tc.branch, tc.branch
			p.pages = map[int]ci.RunPage{
				1: {Runs: []ci.Run{first}, Next: true},
				2: {Runs: []ci.Run{second}, Next: tc.partial},
			}
			c, m := setup(p)
			if err := c.Refresh(context.Background()); err != nil {
				t.Fatal(err)
			}
			snapshot := read(t, m)
			if len(snapshot.Runs) != tc.wantRuns || snapshot.Coverage.Partial != tc.partial {
				t.Fatalf("unexpected observed sample: %#v", snapshot)
			}
			source := inventory(t, c)
			if len(source.Issues) != len(tc.wantKinds) {
				t.Fatalf("source notices = %#v, want kinds %v", source.Issues, tc.wantKinds)
			}
			for i, kind := range tc.wantKinds {
				issue := source.Issues[i]
				if issue.Kind != kind || issue.Repository != "example/project" {
					t.Fatalf("source notice %d = %#v, want repository-scoped %s", i, issue, kind)
				}
				if kind == "no_observed_runs" && strings.Contains(issue.Message, "page limit") {
					t.Fatal("page-limit diagnostic remains tied to the hidden empty-sample notice")
				}
			}
		})
	}
}
func TestDiscoveryErrorIsSanitizedAndPreservesRows(t *testing.T) {
	p := provider()
	c, _ := setup(p)
	c.Refresh(context.Background())
	p.discoveryErr = errors.New("token=private")
	c.now = func() time.Time { return time.Now().Add(time.Hour) }
	c.Refresh(context.Background())
	s := inventory(t, c)
	if len(s.Repositories) != 1 || len(s.Issues) == 0 || s.Issues[0].Message == "token=private" || s.State != "partial" {
		t.Fatalf("discovery error: %#v", s)
	}
}
func TestGapBridgeOlderUnfinishedAndAttemptTiming(t *testing.T) {
	p := provider()
	p.pages = map[int]ci.RunPage{1: {Runs: []ci.Run{run("3", 3, true), run("2", 2, true), run("1", 1, false)}}}
	c, m := setup(p)
	c.Refresh(context.Background())
	if len(read(t, m).Runs) != 3 {
		t.Fatal("older unfinished run trimmed")
	}
	duration := int64(42)
	old := read(t, m)
	old.Runs[0].DurationMS = &duration
	m.Write(context.Background(), old)
	next := run("3", 3, true)
	next.Attempt = 2
	p.pages = map[int]ci.RunPage{1: {Runs: []ci.Run{run("5", 5, true)}, Next: true}, 2: {Runs: []ci.Run{run("4", 4, true), next}, Next: true}}
	p.got["1"] = run("1", 1, true)
	p.enrichErr = errors.New("failed jobs")
	c.Refresh(context.Background())
	v := read(t, m)
	if len(v.Runs) != 2 || v.Runs[0].ID != "5" || v.Runs[1].ID != "4" || p.gets != 1 || p.lists != 3 || !v.Coverage.Partial {
		t.Fatalf("merged: %#v gets=%d lists=%d", v, p.gets, p.lists)
	}
	if v.Health.State != "partial" {
		t.Fatal("timing failure not visible")
	}
}
func TestAttemptInvalidatesTimingButSameAttemptPreservesCache(t *testing.T) {
	p := provider()
	d := int64(10)
	v := run("1", 1, true)
	v.DurationMS = &d
	p.pages[1] = ci.RunPage{Runs: []ci.Run{v}}
	c, m := setup(p)
	c.Refresh(context.Background())
	v.DurationMS = nil
	p.pages[1] = ci.RunPage{Runs: []ci.Run{v}}
	c.Refresh(context.Background())
	if read(t, m).Runs[0].DurationMS == nil {
		t.Fatal("same attempt lost cached timing")
	}
	v.Attempt = 2
	p.pages[1] = ci.RunPage{Runs: []ci.Run{v}}
	c.Refresh(context.Background())
	if read(t, m).Runs[0].DurationMS != nil {
		t.Fatal("new attempt reused timing")
	}
}
func TestRateLimitDefersEntireSourceAndCancellation(t *testing.T) {
	p := provider()
	retry := time.Now().Add(time.Hour)
	p.listErr = &ci.ProviderError{Kind: "rate_limit", RetryAt: &retry}
	c, _ := setup(p)
	c.Refresh(context.Background())
	c.Refresh(context.Background())
	if p.lists != 1 || p.discoveries != 1 {
		t.Fatalf("cooldown hammered provider lists=%d discover=%d", p.lists, p.discoveries)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(c.Refresh(ctx), context.Canceled) {
		t.Fatal("canceled refresh not canceled")
	}
	done := make(chan struct{})
	go func() { c.Start(ctx); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start blocked shutdown")
	}
}
func TestIndependentRepositoryPublicationAndDeadline(t *testing.T) {
	p := provider()
	other := repo()
	other.Key = "fast"
	other.FullName = "example/fast"
	p.discovery.Repositories = append(p.discovery.Repositories, other)
	blocked := make(chan struct{})
	p.listFn = func(ctx context.Context, r ci.Repository, n int) (ci.RunPage, error) {
		if r.Key == "opaque" {
			close(blocked)
			<-ctx.Done()
			return ci.RunPage{}, ctx.Err()
		}
		return ci.RunPage{Runs: []ci.Run{run("ok", 1, true)}}, nil
	}
	c, m := setup(p)
	c.requestTimeout = 100 * time.Millisecond
	done := make(chan struct{})
	go func() { c.Refresh(context.Background()); close(done) }()
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("scan did not start")
	}
	deadline := time.Now().Add(90 * time.Millisecond)
	for {
		snapshot, found, _ := m.Read(context.Background(), other.Key)
		if found && len(snapshot.Runs) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("slow repo blocks fast publication")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("deadline not bounded")
	}
	if read(t, m).Health.State != "unavailable" {
		t.Fatal("timeout invisible")
	}
}

// Regressions: source cooldown must also stop a second page already in progress,
// and a repository's loading placeholder is not a published successful snapshot.
func TestRateLimitStopsOtherRepositoryPagination(t *testing.T) {
	p := provider()
	other := repo()
	other.Key = "other"
	other.FullName = "example/other"
	p.discovery.Repositories = append(p.discovery.Repositories, other)
	ctx := context.Background()
	c, m := setup(p)
	c.Refresh(ctx)
	pageOne := make(chan struct{})
	var once sync.Once
	p.listFn = func(ctx context.Context, r ci.Repository, page int) (ci.RunPage, error) {
		if r.Key == "opaque" {
			once.Do(func() { close(pageOne) })
			for !c.cooling(c.sources[0]) {
				select {
				case <-ctx.Done():
					return ci.RunPage{}, ctx.Err()
				default:
					time.Sleep(time.Millisecond)
				}
			}
			if page > 1 {
				t.Error("provider called after source cooldown")
			}
			return ci.RunPage{Runs: []ci.Run{run("new", 9, true)}, Next: true}, nil
		}
		<-pageOne
		retry := time.Now().Add(time.Hour)
		return ci.RunPage{}, &ci.ProviderError{Kind: "rate_limit", RetryAt: &retry}
	}
	c.Refresh(ctx)
	if len(read(t, m).Runs) == 0 {
		t.Fatal("cooldown erased retained history")
	}
}
func TestSourceFailureDoesNotBlockAnotherSourcePublication(t *testing.T) {
	p := provider()
	fast := provider()
	fastRepo := repo()
	fastRepo.Key = "fast-source"
	fastRepo.Source = "fast"
	fast.discovery.Repositories = []ci.Repository{fastRepo}
	cfg := testConfig()
	cfg.Sources = append(cfg.Sources, config.Source{Name: "fast", Provider: "github", Repos: []string{"example/project"}})
	blocked := make(chan struct{})
	p.listFn = func(ctx context.Context, r ci.Repository, n int) (ci.RunPage, error) {
		close(blocked)
		<-ctx.Done()
		return ci.RunPage{}, ctx.Err()
	}
	m := snapshots.NewMemory()
	c := New(cfg, map[string]ci.Provider{"public": p, "fast": fast}, m)
	c.requestTimeout = 150 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { c.Start(ctx); close(done) }()
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("slow source did not start")
	}
	deadline := time.Now().Add(120 * time.Millisecond)
	for {
		v, found, _ := m.Read(ctx, fastRepo.Key)
		if found && len(v.Runs) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("slow source blocks healthy source publication")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("canceled active scan did not exit")
	}
}

func TestPartialDiscoveryErrorKeepsVerifiedMetadataAndDefersSource(t *testing.T) {
	p := provider()
	c, m := setup(p)
	ctx := context.Background()
	if err := c.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	initial := inventory(t, c)
	lastDiscovery := *initial.LastDiscoveryAt
	scans := p.lists
	now := time.Now().UTC().Add(time.Hour)
	c.now = func() time.Time { return now }
	retry := now.Add(time.Hour)
	added := repo()
	added.Key = "new-repository"
	added.FullName = "example/new"
	p.discovery = ci.Discovery{Complete: true, Repositories: []ci.Repository{added}, Issues: []ci.Issue{{Repository: "example/limited", Kind: "rate_limit", Message: "raw private response"}}}
	p.discoveryErr = &ci.ProviderError{Kind: "rate_limit", RetryAt: &retry}
	if err := c.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	state := inventory(t, c)
	if len(state.Repositories) != 2 {
		t.Fatalf("verified partial discovery or prior inventory discarded: %#v", state)
	}
	if state.State != "partial" || state.LastDiscoveryAt == nil || !state.LastDiscoveryAt.Equal(lastDiscovery) {
		t.Fatalf("failed discovery declared successful: %#v", state)
	}
	haveLoading, haveSelection, haveRetry := false, false, false
	for _, issue := range state.Issues {
		if issue.Repository == "example/new" && issue.Kind == "loading" {
			haveLoading = true
		}
		if issue.Repository == "example/limited" && issue.Kind == "rate_limit" {
			haveSelection = true
		}
		if issue.Kind == "rate_limit" && strings.Contains(issue.Message, "retry") {
			haveRetry = true
		}
		if strings.Contains(issue.Message, "private") {
			t.Fatal("raw discovery error leaked")
		}
	}
	if !haveLoading || !haveSelection || !haveRetry {
		t.Fatalf("partial discovery notices missing: %#v", state.Issues)
	}
	snap, found, err := m.Read(ctx, added.Key)
	if err != nil || !found || snap.Repository.FullName != "example/new" || snap.Health.State != "loading" || len(snap.Runs) != 0 || snap.Health.RetryAt == nil || !snap.Health.RetryAt.Equal(retry) {
		t.Fatalf("new metadata not available during cooldown: %#v found=%v err=%v", snap, found, err)
	}
	if p.lists != scans {
		t.Fatal("discovery rate limit did not stop scans")
	}
	discoveries := p.discoveries
	if err := c.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if p.discoveries != discoveries || p.lists != scans {
		t.Fatal("cooldown did not defer entire source")
	}
	now = retry.Add(time.Second)
	p.discoveryErr = nil
	p.discovery = ci.Discovery{Complete: true, Repositories: []ci.Repository{added}}
	if err := c.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if len(inventory(t, c).Repositories) != 1 || p.discoveries != discoveries+1 {
		t.Fatal("successful retry did not reconcile inventory")
	}
}

func TestSourceRetryDeadlineDefersPollingAndPreservesStaleRuns(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	future, past := now.Add(time.Second), now.Add(-time.Second)
	for _, tc := range []struct {
		name, kind string
		retry      *time.Time
		wantCalls  int
	}{
		{"unavailable future deadline", "unavailable", &future, 1},
		{"unavailable expired deadline", "unavailable", &past, 2},
		{"rate limit without deadline", "rate_limit", nil, 1},
		{"rate limit expired deadline", "rate_limit", &past, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := provider()
			c, m := setup(p)
			current := now
			c.now = func() time.Time { return current }
			ctx := context.Background()
			if err := c.Refresh(ctx); err != nil {
				t.Fatal(err)
			}
			before := p.lists
			p.listErr = &ci.ProviderError{Kind: tc.kind, RetryAt: tc.retry}
			for i := 0; i < 2; i++ {
				if err := c.Refresh(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if got := p.lists - before; got != tc.wantCalls {
				t.Fatalf("poll requests before retry: got %d, want %d", got, tc.wantCalls)
			}
			snap := read(t, m)
			if snap.Health.State != "stale" || len(snap.Runs) != 1 || snap.Runs[0].ID != "1" {
				t.Fatalf("retry erased stale history: %#v", snap)
			}
			if tc.retry == &future {
				if snap.Health.RetryAt == nil || !snap.Health.RetryAt.Equal(future) {
					t.Fatalf("explicit short retry deadline lost: %#v", snap.Health)
				}
				current = future.Add(time.Nanosecond)
				p.listErr = nil
				if err := c.Refresh(ctx); err != nil {
					t.Fatal(err)
				}
				if p.lists-before != 2 || read(t, m).Health.State != "healthy" {
					t.Fatal("polling did not resume after the explicit deadline")
				}
			}
		})
	}
}

func TestRetentionAndDurationUseHeadRepositoryIdentity(t *testing.T) {
	for _, tc := range []struct{ scope, branch string }{{"default", "main"}, {"release", "release"}, {"all", "main"}} {
		t.Run(tc.scope, func(t *testing.T) {
			var runs []ci.Run
			payload := fmt.Sprintf(`[
				{"id":"local-pr","workflow_id":"checks","branch":%q,"head_repository_id":"42","event":"pull_request","created_at":"2026-10-14T12:00:00Z","terminal":true,"state":"success"},
				{"id":"fork-pr","workflow_id":"checks","branch":%q,"head_repository_id":"99","event":"pull_request","url":"https://github.com/acme/project/actions/runs/3","created_at":"2026-10-14T12:01:00Z","terminal":true,"state":"failure"},
				{"id":"unknown-pr","workflow_id":"checks","branch":%q,"event":"pull_request","created_at":"2026-10-14T12:02:00Z","terminal":true,"state":"attention"}
			]`, tc.branch, tc.branch, tc.branch)
			if err := json.Unmarshal([]byte(payload), &runs); err != nil {
				t.Fatal(err)
			}
			r := repo()
			r.ID = "42"
			p := provider()
			p.discovery.Repositories = []ci.Repository{r}
			p.pages[1] = ci.RunPage{Runs: runs}
			cfg := testConfig()
			cfg.Viewer.Branch = tc.scope
			cfg.Viewer.HistoryRuns = 10
			m := snapshots.NewMemory()
			c := New(cfg, map[string]ci.Provider{"public": p}, m)
			ctx := context.Background()
			if err := c.Refresh(ctx); err != nil {
				t.Fatal(err)
			}
			snap := read(t, m)
			if tc.scope == "all" {
				if len(snap.Runs) != 3 || snap.Runs[1].ID != "fork-pr" || snap.Runs[1].Branch != "main" || snap.Runs[1].URL != "https://github.com/acme/project/actions/runs/3" {
					t.Fatalf("all scope discarded original PR history: %#v", snap.Runs)
				}
				// Retained all-scope snapshots must still be safe for branch overviews.
				result, err := ci.NewOverview(m, c, 10, "default", time.Minute).Overview(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Workflows) != 1 || len(result.Workflows[0].Runs) != 1 || result.Workflows[0].LatestRun.ID != "local-pr" {
					t.Fatalf("retained all-scope snapshot polluted default overview: %#v", result.Workflows)
				}
			} else {
				if len(snap.Runs) != 1 || snap.Runs[0].ID != "local-pr" {
					t.Fatalf("branch retention mixed repositories: %#v", snap.Runs)
				}
				if len(p.enrichedIDs) != 1 || p.enrichedIDs[0] != "local-pr" {
					t.Fatalf("duration selected a foreign or unknown PR: %v", p.enrichedIDs)
				}
			}
		})
	}
}

func TestRefreshPublishesRepositoriesFromConfiguredUserSelection(t *testing.T) {
	p := provider()
	selected := repo()
	selected.FullName = "example-user/project"
	p.discoverFn = func(_ context.Context, selection ci.Selection) (ci.Discovery, error) {
		if len(selection.Users) != 1 || selection.Users[0] != "example-user" || len(selection.Orgs) != 1 || selection.Orgs[0] != "example-org" || len(selection.Repos) != 1 || selection.Repos[0] != "example-org/project" {
			return ci.Discovery{Complete: true}, nil
		}
		return ci.Discovery{Complete: true, Repositories: []ci.Repository{selected}}, nil
	}
	cfg := testConfig()
	cfg.Sources[0].Users = []string{"example-user"}
	cfg.Sources[0].Orgs = []string{"example-org"}
	cfg.Sources[0].Repos = []string{"example-org/project"}
	store := snapshots.NewMemory()
	c := New(cfg, map[string]ci.Provider{"public": p}, store)
	if err := c.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	state := inventory(t, c)
	if len(state.Repositories) != 1 || state.Repositories[0] != "opaque" {
		t.Fatalf("configured user repository missing from inventory: %#v", state)
	}
	snapshot := read(t, store)
	if snapshot.Repository.FullName != "example-user/project" || len(snapshot.Runs) != 1 || snapshot.Runs[0].ID != "1" {
		t.Fatalf("configured user repository runs missing: %#v", snapshot)
	}
}
