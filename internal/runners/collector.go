// Package runners collects and caches self-hosted runner status.
package runners

import (
	"context"
	"errors"
	"github.com/danielfrg/ciview/internal/ci"
	"sort"
	"sync"
	"time"
)

const requestTimeout = 20 * time.Second

// Scope is one organization or repository to read through a provider.
type Scope struct {
	Source   string
	Scope    string
	Target   ci.RunnerTarget
	Provider ci.RunnerProvider
}

type scopeState struct {
	Scope
	mu      sync.Mutex
	value   ci.RunnerScope
	retryAt *time.Time
}

// Collector polls each scope independently and serves cached copies. Reads
// never contact a provider.
type Collector struct {
	scopes   []*scopeState
	interval time.Duration
	now      func() time.Time
}

var _ ci.RunnerReader = (*Collector)(nil)

func New(scopes []Scope, interval time.Duration) *Collector {
	c := &Collector{interval: interval, now: time.Now}
	for _, s := range scopes {
		c.scopes = append(c.scopes, &scopeState{Scope: s, value: ci.RunnerScope{Scope: s.Scope, Source: s.Source, State: "loading", Runners: []ci.Runner{}}})
	}
	return c
}

// Start refreshes every interval until the context ends.
func (c *Collector) Start(ctx context.Context) {
	for {
		c.Refresh(ctx)
		timer := time.NewTimer(c.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// Refresh reads every scope once, concurrently. Failures are recorded on the
// scope and never returned: the previous runner list stays visible as stale.
func (c *Collector) Refresh(ctx context.Context) {
	var wg sync.WaitGroup
	for _, s := range c.scopes {
		wg.Add(1)
		go func(s *scopeState) {
			defer wg.Done()
			c.refresh(ctx, s)
		}(s)
	}
	wg.Wait()
}

func (c *Collector) refresh(ctx context.Context, s *scopeState) {
	s.mu.Lock()
	cooling := s.retryAt != nil && c.now().Before(*s.retryAt)
	s.mu.Unlock()
	if cooling || ctx.Err() != nil {
		return
	}
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	runners, err := s.Provider.ListRunners(reqCtx, s.Target)
	if ctx.Err() != nil {
		return
	}
	now := c.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.value.LastAttemptAt = &now
	if err == nil {
		s.value.State, s.value.Kind, s.value.Message = "healthy", "", ""
		s.value.AsOf = &now
		s.value.Runners = append([]ci.Runner{}, runners...)
		s.retryAt = nil
		return
	}
	kind := "unavailable"
	s.retryAt = nil
	var pe *ci.ProviderError
	if errors.As(err, &pe) {
		kind = pe.Kind
		if pe.RetryAt != nil {
			retry := pe.RetryAt.UTC()
			s.retryAt = &retry
		}
	}
	kind, message := describe(kind)
	s.value.Kind, s.value.Message = kind, message
	if s.value.AsOf != nil {
		s.value.State = "stale"
	} else {
		s.value.State = "unavailable"
	}
}

// describe maps a provider failure to fixed, credential-safe text.
func describe(kind string) (string, string) {
	switch kind {
	case "authentication", "auth", "unauthorized":
		return "authentication", "The token was rejected"
	case "permission", "forbidden":
		return "permission", "The token cannot list runners for this scope"
	case "rate_limit", "rate_limited":
		return "rate_limit", "Provider rate limit reached"
	case "not_found", "missing":
		return "not_found", "Runner scope was not found or is not visible to the token"
	default:
		return "unavailable", "Runners could not be read"
	}
}

// Runners returns independent copies, ordered by scope then source.
func (c *Collector) Runners(ctx context.Context) ([]ci.RunnerScope, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make([]ci.RunnerScope, 0, len(c.scopes))
	for _, s := range c.scopes {
		s.mu.Lock()
		v := s.value
		v.AsOf = cloneTime(v.AsOf)
		v.LastAttemptAt = cloneTime(v.LastAttemptAt)
		v.Runners = make([]ci.Runner, len(s.value.Runners))
		for i, r := range s.value.Runners {
			r.Labels = append([]string{}, r.Labels...)
			v.Runners[i] = r
		}
		s.mu.Unlock()
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		return out[i].Source < out[j].Source
	})
	return out, nil
}

func cloneTime(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	out := v.UTC()
	return &out
}
