package runners

import (
	"ciview/internal/ci"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeProvider struct {
	mu      sync.Mutex
	runners []ci.Runner
	err     error
	calls   int
}

func (f *fakeProvider) ListRunners(context.Context, ci.RunnerTarget) ([]ci.Runner, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return append([]ci.Runner(nil), f.runners...), f.err
}
func (f *fakeProvider) set(runners []ci.Runner, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runners, f.err = runners, err
}

func newCollector(p *fakeProvider, now *time.Time) *Collector {
	c := New([]Scope{{Source: "github", Scope: "org:acme", Target: ci.RunnerTarget{Org: "acme"}, Provider: p}}, time.Minute)
	c.now = func() time.Time { return *now }
	return c
}
func only(t *testing.T, c *Collector) ci.RunnerScope {
	t.Helper()
	got, err := c.Runners(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatalf("runners=%+v err=%v", got, err)
	}
	return got[0]
}

func TestScopeIsLoadingUntilFirstRead(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	got := only(t, newCollector(&fakeProvider{}, &now))
	if got.State != "loading" || got.AsOf != nil || len(got.Runners) != 0 || got.Scope != "org:acme" || got.Source != "github" {
		t.Fatalf("scope=%+v", got)
	}
}

func TestRefreshPublishesRunnersAndFreshness(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	p := &fakeProvider{runners: []ci.Runner{{ID: "1", Name: "r1", Status: "online", Busy: true}}}
	c := newCollector(p, &now)
	c.Refresh(context.Background())
	got := only(t, c)
	if got.State != "healthy" || got.AsOf == nil || !got.AsOf.Equal(now) || len(got.Runners) != 1 || !got.Runners[0].Busy || got.Kind != "" || got.Message != "" {
		t.Fatalf("scope=%+v", got)
	}
	got.Runners[0].Name = "mutated"
	if only(t, c).Runners[0].Name != "r1" {
		t.Fatal("callers must not share collector storage")
	}
}

func TestFailureRetainsLastRunnersAsStaleWithSafeMessage(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	p := &fakeProvider{runners: []ci.Runner{{ID: "1", Name: "r1", Status: "online"}}}
	c := newCollector(p, &now)
	c.Refresh(context.Background())
	first := now
	now = now.Add(time.Minute)
	p.set(nil, &ci.ProviderError{Kind: "permission", Message: "token ghp_secret lacks scope"})
	c.Refresh(context.Background())
	got := only(t, c)
	if got.State != "stale" || got.Kind != "permission" || len(got.Runners) != 1 || !got.AsOf.Equal(first) || !got.LastAttemptAt.Equal(now) {
		t.Fatalf("scope=%+v", got)
	}
	if strings.Contains(got.Message, "ghp_secret") || got.Message == "" {
		t.Fatalf("message must be fixed text: %q", got.Message)
	}
	p.set([]ci.Runner{{ID: "2", Name: "r2", Status: "offline"}}, nil)
	now = now.Add(time.Minute)
	c.Refresh(context.Background())
	if got := only(t, c); got.State != "healthy" || got.Kind != "" || got.Runners[0].Name != "r2" {
		t.Fatalf("recovery scope=%+v", got)
	}
}

func TestFailureWithoutPriorReadIsUnavailable(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	c := newCollector(&fakeProvider{err: errors.New("dial tcp 10.0.0.1: boom")}, &now)
	c.Refresh(context.Background())
	got := only(t, c)
	if got.State != "unavailable" || got.Kind != "unavailable" || got.AsOf != nil || strings.Contains(got.Message, "10.0.0.1") {
		t.Fatalf("scope=%+v", got)
	}
}

func TestRateLimitedScopeWaitsUntilRetryAt(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	retry := now.Add(10 * time.Minute)
	p := &fakeProvider{err: &ci.ProviderError{Kind: "rate_limit", RetryAt: &retry}}
	c := newCollector(p, &now)
	c.Refresh(context.Background())
	now = now.Add(time.Minute)
	c.Refresh(context.Background())
	if p.calls != 1 {
		t.Fatalf("provider called %d times during cooldown", p.calls)
	}
	now = retry
	c.Refresh(context.Background())
	if p.calls != 2 {
		t.Fatalf("provider not retried after cooldown: %d", p.calls)
	}
}

func TestCanceledRefreshIsNotRecordedAsFailure(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	c := newCollector(&fakeProvider{err: context.Canceled}, &now)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Refresh(ctx)
	if got := only(t, c); got.State != "loading" || got.Kind != "" {
		t.Fatalf("scope=%+v", got)
	}
}

func TestScopesAreSortedAndIndependent(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	good, bad := &fakeProvider{runners: []ci.Runner{{ID: "1", Name: "r", Status: "online"}}}, &fakeProvider{err: &ci.ProviderError{Kind: "not_found"}}
	c := New([]Scope{
		{Source: "github", Scope: "repo:acme/zed", Target: ci.RunnerTarget{Repo: "acme/zed"}, Provider: bad},
		{Source: "github", Scope: "org:acme", Target: ci.RunnerTarget{Org: "acme"}, Provider: good},
	}, time.Minute)
	c.now = func() time.Time { return now }
	c.Refresh(context.Background())
	got, _ := c.Runners(context.Background())
	if len(got) != 2 || got[0].Scope != "org:acme" || got[0].State != "healthy" || got[1].Scope != "repo:acme/zed" || got[1].State != "unavailable" {
		t.Fatalf("scopes=%+v", got)
	}
}
