// Package refresh owns background discovery and atomic repository publication.
package refresh

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"ciview/internal/ci"
	"ciview/internal/config"
)

const repositoryConcurrency = 4

type source struct {
	config             config.Source
	provider           ci.Provider
	gate               chan struct{}
	repositories       map[ci.RepositoryKey]ci.Repository
	discoveryIssues    []ci.Issue
	repositoryIssues   map[ci.RepositoryKey][]ci.Issue
	health             map[ci.RepositoryKey]string
	lastDiscovery      *time.Time
	discoveryAttempted bool
	discoveryFailed    bool
	retryAt            *time.Time
}

// Coordinator serializes each source while allowing sources and repositories
// to publish independently. Inventory values never share mutable storage.
type Coordinator struct {
	cfg            config.Config
	store          ci.SnapshotStore
	mu             sync.RWMutex
	sources        []*source
	now            func() time.Time
	requestTimeout time.Duration
}

func New(cfg config.Config, providers map[string]ci.Provider, store ci.SnapshotStore) *Coordinator {
	c := &Coordinator{cfg: cfg, store: store, now: time.Now, requestTimeout: 30 * time.Second}
	for _, sc := range cfg.Sources {
		s := &source{config: sc, provider: providers[sc.Name], gate: make(chan struct{}, 1), repositories: map[ci.RepositoryKey]ci.Repository{}, repositoryIssues: map[ci.RepositoryKey][]ci.Issue{}, health: map[ci.RepositoryKey]string{}}
		for _, name := range sc.Repos {
			s.discoveryIssues = append(s.discoveryIssues, ci.Issue{Repository: name, Kind: "loading", Message: "Waiting for initial discovery"})
		}
		c.sources = append(c.sources, s)
	}
	return c
}

func (c *Coordinator) Inventory(ctx context.Context) (ci.Inventory, error) {
	if err := ctx.Err(); err != nil {
		return ci.Inventory{}, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := ci.Inventory{Sources: make([]ci.SourceState, 0, len(c.sources))}
	for _, s := range c.sources {
		v := ci.SourceState{Name: s.config.Name, Provider: s.config.Provider, State: "healthy", Repositories: make([]ci.RepositoryKey, 0, len(s.repositories)), Issues: append([]ci.Issue{}, s.discoveryIssues...)}
		if s.lastDiscovery != nil {
			t := *s.lastDiscovery
			v.LastDiscoveryAt = &t
		}
		if !s.discoveryAttempted {
			v.State = "loading"
		} else if len(s.discoveryIssues) > 0 {
			v.State = "partial"
		}
		states := map[string]int{}
		for key := range s.repositories {
			v.Repositories = append(v.Repositories, key)
			states[s.health[key]]++
		}
		sort.Slice(v.Repositories, func(i, j int) bool { return v.Repositories[i] < v.Repositories[j] })
		for _, key := range v.Repositories {
			v.Issues = append(v.Issues, s.repositoryIssues[key]...)
		}
		if len(s.repositories) > 0 {
			switch {
			case states["unavailable"] == len(s.repositories):
				v.State = "unavailable"
			case states["stale"] == len(s.repositories):
				v.State = "stale"
			case states["loading"] == len(s.repositories) && len(s.discoveryIssues) == 0:
				v.State = "loading"
			case states["stale"]+states["unavailable"]+states["partial"]+states["loading"] > 0:
				v.State = "partial"
			}
		} else if s.discoveryAttempted && len(s.discoveryIssues) > 0 {
			v.State = "unavailable"
		}
		result.Sources = append(result.Sources, v)
	}
	return result, nil
}

// Start runs an independent polling loop per source and returns after cancellation.
func (c *Coordinator) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for _, s := range c.sources {
		wg.Add(1)
		go func(s *source) {
			defer wg.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				_ = c.refreshSource(ctx, s)
				timer := time.NewTimer(c.cfg.Viewer.RefreshDuration())
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
		}(s)
	}
	wg.Wait()
}

// Refresh runs one collection pass. Provider failures are recorded in health;
// cancellation and store failures are returned to the caller.
func (c *Coordinator) Refresh(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(c.sources))
	for _, s := range c.sources {
		wg.Add(1)
		go func(s *source) {
			defer wg.Done()
			if err := c.refreshSource(ctx, s); err != nil {
				errs <- err
			}
		}(s)
	}
	wg.Wait()
	close(errs)
	var collected []error
	for err := range errs {
		collected = append(collected, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.Join(collected...)
}

func (c *Coordinator) cooling(s *source) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return s.retryAt != nil && c.now().Before(*s.retryAt)
}
func (c *Coordinator) deferSource(s *source, err error) {
	var pe *ci.ProviderError
	if !errors.As(err, &pe) {
		return
	}
	now := c.now()
	var retry time.Time
	switch {
	case pe.RetryAt != nil && pe.RetryAt.After(now):
		retry = pe.RetryAt.UTC()
	case pe.RetryAt == nil && (pe.Kind == "rate_limit" || pe.Kind == "rate_limited"):
		retry = now.Add(c.cfg.Viewer.RefreshDuration())
	default:
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if s.retryAt == nil || retry.After(*s.retryAt) {
		s.retryAt = &retry
	}
}

func (c *Coordinator) refreshSource(ctx context.Context, s *source) error {
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if c.cooling(s) {
		return nil
	}
	c.mu.RLock()
	discover := s.discoveryFailed || s.lastDiscovery == nil || c.now().Sub(*s.lastDiscovery) >= c.cfg.Viewer.DiscoveryDuration()
	c.mu.RUnlock()
	if s.provider == nil {
		c.mu.Lock()
		s.discoveryAttempted = true
		s.discoveryFailed = true
		s.discoveryIssues = []ci.Issue{{Kind: "unavailable", Message: "Source provider is unavailable"}}
		c.mu.Unlock()
		return nil
	}
	if discover {
		call, cancel := context.WithTimeout(ctx, c.requestTimeout)
		d, err := s.provider.Discover(call, ci.Selection{Orgs: s.config.Orgs, Users: s.config.Users, Repos: s.config.Repos})
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// A failed discovery can contain repositories verified before the failure.
		// Establish a cooldown before reconciliation or any run collection.
		c.deferSource(s, err)
		if err := c.reconcileDiscovery(ctx, s, d, err); err != nil {
			return err
		}
		if c.cooling(s) {
			return nil
		}

	}
	c.mu.RLock()
	repos := make([]ci.Repository, 0, len(s.repositories))
	for _, r := range s.repositories {
		repos = append(repos, r)
	}
	c.mu.RUnlock()
	sort.Slice(repos, func(i, j int) bool { return repos[i].Key < repos[j].Key })
	jobs := make(chan ci.Repository)
	errs := make(chan error, len(repos))
	var wg sync.WaitGroup
	for i := 0; i < repositoryConcurrency && i < len(repos); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				if ctx.Err() != nil {
					continue
				}
				if c.cooling(s) {
					continue
				}
				if err := c.scan(ctx, s, r, discover); err != nil {
					errs <- err
				}
			}
		}()
	}
	for _, r := range repos {
		select {
		case jobs <- r:
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()
	close(errs)
	var collected []error
	for err := range errs {
		collected = append(collected, err)
	}
	return errors.Join(collected...)
}

func (c *Coordinator) scan(ctx context.Context, s *source, r ci.Repository, reconcile bool) error {
	call, cancel := context.WithTimeout(ctx, c.requestTimeout)
	defer cancel()
	prior, found, err := c.store.Read(call, r.Key)
	if err != nil {
		return errors.New("snapshot store read failed")
	}
	now := c.now().UTC()
	// Read current workflow files to annotate historical rows. Only opted-in
	// sources exclude missing files. All-branch views have no single ref to check.
	var workflowFiles map[string]bool
	var catalogFiles []string
	var catalogNames map[string]string
	var catalogBranch string
	var catalogError error
	if c.cfg.Viewer.Branch != "all" {
		branch := c.cfg.Viewer.Branch
		if branch == "default" {
			branch = r.DefaultBranch
		}
		if err := c.cooldownError(s); err != nil {
			return c.fail(ctx, s, r, prior, found, now, err)
		}
		var files []string
		var err error
		if provider, ok := s.provider.(ci.WorkflowCatalogProvider); ok {
			var catalog ci.WorkflowCatalog
			catalog, err = provider.ListWorkflowCatalog(call, r, branch)
			files, catalogNames = catalog.Files, catalog.Names
			c.deferSource(s, catalog.MetadataError)
		} else {
			files, err = s.provider.ListWorkflowFiles(call, r, branch)
		}
		if err != nil {
			if s.config.ExistingWorkflowsOnly {
				return c.fail(ctx, s, r, prior, found, now, err)
			}
			catalogError = err
			c.deferSource(s, err)
		} else {
			catalogFiles, catalogBranch = files, branch
			workflowFiles = make(map[string]bool, len(files))
			for _, file := range files {
				workflowFiles[file] = true
			}
		}
	}
	visibleWorkflow := func(run ci.Run) bool {
		if r.Provider == "github" && !s.config.ShowDependabotUpdates && strings.HasPrefix(run.WorkflowFile, "dynamic/dependabot/") {
			return false
		}
		return !s.config.ExistingWorkflowsOnly || workflowFiles == nil || workflowFiles[run.WorkflowFile]
	}
	old := map[string]ci.Run{}
	for _, v := range prior.Runs {
		if !visibleWorkflow(v) {
			continue
		}
		old[v.ID] = v
	}
	collected := map[string]ci.Run{}
	coverage := ci.Coverage{}
	for page := 1; page <= c.cfg.Viewer.HistoryPages; page++ {
		if err := call.Err(); err != nil {
			return c.fail(ctx, s, r, prior, found, now, err)
		}
		if err := c.cooldownError(s); err != nil {
			return c.fail(ctx, s, r, prior, found, now, err)
		}
		p, err := s.provider.ListRuns(call, r, page)
		if err != nil {
			return c.fail(ctx, s, r, prior, found, now, err)
		}
		bridge := false
		for _, v := range p.Runs {
			coverage.FetchedRuns++
			if coverage.OldestFetchedAt == nil || v.CreatedAt.Before(*coverage.OldestFetchedAt) {
				t := v.CreatedAt.UTC()
				coverage.OldestFetchedAt = &t
			}
			if !visibleWorkflow(v) {
				continue
			}
			if _, ok := old[v.ID]; ok {
				bridge = true
			}
			mergeRun(collected, v, old)
		}
		if !p.Next {
			break
		}
		if page == c.cfg.Viewer.HistoryPages {
			coverage.Partial = true
			break
		}
		if !reconcile && prior.Health.LastSuccessAt != nil && bridge {
			coverage.Partial = prior.Coverage.Partial
			break
		}
	}
	for id, v := range old {
		if _, ok := collected[id]; !ok {
			if !v.Terminal {
				if err := call.Err(); err != nil {
					return c.fail(ctx, s, r, prior, found, now, err)
				}
				if err := c.cooldownError(s); err != nil {
					return c.fail(ctx, s, r, prior, found, now, err)
				}
				updated, err := s.provider.GetRun(call, r, id)
				if err != nil {
					return c.fail(ctx, s, r, prior, found, now, err)
				}
				mergeRun(collected, updated, old)
			} else {
				collected[id] = v
			}
		}
	}
	runs := make([]ci.Run, 0, len(collected))
	for _, v := range collected {
		runs = append(runs, v)
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].ID > runs[j].ID
		}
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})
	counts := map[string]int{}
	retained := make([]ci.Run, 0, len(runs))
	for _, v := range runs {
		if !ci.MatchesBranchScope(c.cfg.Viewer.Branch, r, v) {
			continue
		}
		counts[v.WorkflowID]++
		if counts[v.WorkflowID] <= c.cfg.Viewer.HistoryRuns || !v.Terminal {
			retained = append(retained, v)
		}
	}
	health := ci.Health{State: "healthy", LastAttemptAt: now, LastSuccessAt: &now}
	issues := []ci.Issue{}
	if catalogError != nil {
		kind, message := safeError(catalogError)
		health.State = "partial"
		health.Message = "Workflow file status unavailable"
		issues = append(issues, ci.Issue{Repository: r.FullName, Kind: kind, Message: "Workflow file status unavailable: " + message})
	}
	enriched := map[string]bool{}
	for i, v := range retained {
		if !v.Terminal || enriched[v.WorkflowID] {
			continue
		}
		enriched[v.WorkflowID] = true
		if v.DurationMS != nil && *v.DurationMS >= 0 {
			continue
		}
		var timed ci.Run
		err := c.cooldownError(s)
		if err == nil {
			timed, err = s.provider.EnrichDuration(call, r, v)
		}
		if err != nil {
			c.deferSource(s, err)
			kind, message := safeError(err)
			health.State = "partial"
			health.Message = "Execution timing unavailable"
			issues = append(issues, ci.Issue{Repository: r.FullName, Kind: kind, Message: "Execution timing unavailable: " + message})
			continue
		}
		// Enrichment supplies timing only, never run identity or status.
		retained[i].DurationMS = timed.DurationMS
		retained[i].DurationKind = timed.DurationKind
		retained[i].StartedAt = timed.StartedAt
		retained[i].FinishedAt = timed.FinishedAt
	}
	if len(retained) == 0 {
		issues = append(issues, ci.Issue{Repository: r.FullName, Kind: "no_observed_runs", Message: "No observed runs in the selected branch scope"})
		if coverage.Partial {
			issues = append(issues, ci.Issue{Repository: r.FullName, Kind: "limited_history", Message: "History scan reached its page limit"})
		}
	}
	c.mu.RLock()
	if s.retryAt != nil && c.now().Before(*s.retryAt) {
		t := *s.retryAt
		health.RetryAt = &t
	}
	c.mu.RUnlock()
	snap := ci.RepositorySnapshot{Repository: r, Runs: retained, Coverage: coverage, Health: health, WorkflowNames: catalogNames, WorkflowFiles: catalogFiles, WorkflowFilesBranch: catalogBranch}
	if err := c.store.Write(ctx, snap); err != nil {
		return errors.New("snapshot store write failed")
	}
	c.mu.Lock()
	s.health[r.Key] = health.State
	s.repositoryIssues[r.Key] = issues
	c.mu.Unlock()
	return nil
}

func mergeRun(values map[string]ci.Run, v ci.Run, old map[string]ci.Run) {
	if previous, ok := old[v.ID]; ok {
		if previous.Attempt > v.Attempt {
			v = previous
		} else if previous.Attempt == v.Attempt && v.Terminal && previous.Terminal && v.DurationMS == nil && previous.DurationMS != nil && *previous.DurationMS >= 0 {
			v.DurationMS = previous.DurationMS
			v.DurationKind = previous.DurationKind
			v.StartedAt = previous.StartedAt
			v.FinishedAt = previous.FinishedAt
		}
	}
	if existing, ok := values[v.ID]; !ok || v.Attempt >= existing.Attempt {
		values[v.ID] = v
	}
}
func safeError(err error) (string, string) {
	if errors.Is(err, context.DeadlineExceeded) {
		return "unavailable", "Provider request timed out"
	}
	if errors.Is(err, context.Canceled) {
		return "unavailable", "Provider request canceled"
	}
	var pe *ci.ProviderError
	if errors.As(err, &pe) {
		return pe.Kind, pe.Error()
	}
	return "unavailable", "Provider request failed"
}
func (c *Coordinator) fail(ctx context.Context, s *source, r ci.Repository, prior ci.RepositorySnapshot, found bool, now time.Time, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	c.deferSource(s, err)
	kind, message := safeError(err)
	if !found {
		prior = ci.RepositorySnapshot{Repository: r, Runs: []ci.Run{}}
	}
	prior.Repository = r
	prior.Health.State = "unavailable"
	if prior.Health.LastSuccessAt != nil {
		prior.Health.State = "stale"
	}
	prior.Health.Message = message
	prior.Health.LastAttemptAt = now
	c.mu.RLock()
	if s.retryAt != nil {
		t := *s.retryAt
		prior.Health.RetryAt = &t
	}
	c.mu.RUnlock()
	if writeErr := c.store.Write(ctx, prior); writeErr != nil {
		return fmt.Errorf("snapshot store write failed")
	}
	c.mu.Lock()
	s.health[r.Key] = prior.Health.State
	s.repositoryIssues[r.Key] = []ci.Issue{{Repository: r.FullName, Kind: kind, Message: message}}
	c.mu.Unlock()
	return nil
}

func (c *Coordinator) cooldownError(s *source) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if s.retryAt != nil && c.now().Before(*s.retryAt) {
		t := *s.retryAt
		return &ci.ProviderError{Kind: "rate_limit", RetryAt: &t}
	}
	return nil
}

// reconcileDiscovery publishes verified metadata even after partial discovery.
// Only an error-free complete discovery may remove previously active repositories.
func (c *Coordinator) reconcileDiscovery(ctx context.Context, s *source, d ci.Discovery, discoveryErr error) error {
	issues := append([]ci.Issue{}, d.Issues...)
	for i := range issues {
		if issues[i].Kind != "loading" && issues[i].Kind != "no_observed_runs" && issues[i].Kind != "limited_history" {
			_, issues[i].Message = safeError(&ci.ProviderError{Kind: issues[i].Kind})
		}
	}
	if discoveryErr != nil {
		d.Complete = false
		kind, message := safeError(discoveryErr)
		c.mu.RLock()
		if s.retryAt != nil && c.now().Before(*s.retryAt) {
			message += "; retry after " + s.retryAt.UTC().Format(time.RFC3339)
		}
		c.mu.RUnlock()
		issues = append(issues, ci.Issue{Kind: kind, Message: message})
	}
	if !d.Complete && len(issues) == 0 {
		issues = append(issues, ci.Issue{Kind: "partial", Message: "Repository discovery is incomplete; retaining previously observed repositories"})
	}
	newRepos := map[ci.RepositoryKey]ci.Repository{}
	for _, r := range d.Repositories {
		newRepos[r.Key] = r
		_, found, err := c.store.Read(ctx, r.Key)
		if err != nil {
			return errors.New("snapshot store read failed")
		}
		if !found {
			health := ci.Health{State: "loading"}
			c.mu.RLock()
			if s.retryAt != nil && c.now().Before(*s.retryAt) {
				t := *s.retryAt
				health.RetryAt = &t
			}
			c.mu.RUnlock()
			snap := ci.RepositorySnapshot{Repository: r, Runs: []ci.Run{}, Health: health}
			if err := c.store.Write(ctx, snap); err != nil {
				return errors.New("snapshot store write failed")
			}
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !d.Complete {
		for key, r := range s.repositories {
			if _, ok := newRepos[key]; !ok {
				newRepos[key] = r
			}
		}
	}
	if s.retryAt != nil && c.now().Before(*s.retryAt) {
		// Selections after an early cooldown return have not been verified.
		// Preserve their explicit names rather than silently dropping them.
		for _, selection := range s.config.Repos {
			covered := false
			for _, r := range newRepos {
				if strings.EqualFold(selection, r.FullName) {
					covered = true
					break
				}
			}
			if !covered {
				for _, issue := range issues {
					if strings.EqualFold(selection, issue.Repository) {
						covered = true
						break
					}
				}
			}
			if !covered {
				issues = append(issues, ci.Issue{Repository: selection, Kind: "loading", Message: "Repository discovery deferred until source retry"})
			}
		}
	}
	s.repositories = newRepos
	s.discoveryIssues = issues
	s.discoveryAttempted = true
	s.discoveryFailed = discoveryErr != nil
	if discoveryErr == nil {
		t := c.now().UTC()
		s.lastDiscovery = &t
	}
	for key, r := range newRepos {
		if _, ok := s.health[key]; !ok {
			s.health[key] = "loading"
			s.repositoryIssues[key] = []ci.Issue{{Repository: r.FullName, Kind: "loading", Message: "Waiting for initial run scan"}}
		}
	}
	for key := range s.health {
		if _, ok := newRepos[key]; !ok {
			delete(s.health, key)
			delete(s.repositoryIssues, key)
		}
	}
	return nil
}
