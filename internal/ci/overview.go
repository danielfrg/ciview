package ci

import (
	"context"
	"sort"
	"strconv"
	"time"
)

// OverviewService computes a cached view from active inventory and stored snapshots.
type OverviewService struct {
	store           SnapshotStore
	inventory       InventoryReader
	historyRuns     int
	branch          string
	refreshInterval time.Duration
}

func NewOverview(store SnapshotStore, inventory InventoryReader, historyRuns int, branch string, refreshInterval time.Duration) *OverviewService {
	return &OverviewService{store: store, inventory: inventory, historyRuns: historyRuns, branch: branch, refreshInterval: refreshInterval}
}

func (s *OverviewService) Overview(ctx context.Context) (Overview, error) {
	inventory, err := s.inventory.Inventory(ctx)
	if err != nil {
		return Overview{}, err
	}
	result := Overview{Workflows: make([]Workflow, 0), Sources: make([]SourceState, 0, len(inventory.Sources)), GeneratedAt: time.Now().UTC(), PollIntervalSeconds: int(s.refreshInterval / time.Second)}
	seen := make(map[RepositoryKey]bool)
	for _, source := range inventory.Sources {
		state := source
		state.Repositories = append(make([]RepositoryKey, 0, len(source.Repositories)), source.Repositories...)
		state.Issues = append(make([]Issue, 0, len(source.Issues)), source.Issues...)
		state.LastDiscoveryAt = copyTime(source.LastDiscoveryAt)
		result.Sources = append(result.Sources, state)
		for _, key := range source.Repositories {
			if seen[key] {
				continue
			}
			seen[key] = true
			snapshot, found, err := s.store.Read(ctx, key)
			if err != nil {
				return Overview{}, err
			}
			if !found {
				state.Issues = append(state.Issues, Issue{Repository: string(key), Kind: "loading", Message: "Waiting for initial scan"})
				result.Sources[len(result.Sources)-1] = state
				continue
			}
			result.Workflows = append(result.Workflows, s.workflows(snapshot)...)
		}
	}
	sort.Slice(result.Workflows, func(i, j int) bool {
		a, b := result.Workflows[i], result.Workflows[j]
		if a.Repository.Source != b.Repository.Source {
			return a.Repository.Source < b.Repository.Source
		}
		if a.Repository.FullName != b.Repository.FullName {
			return a.Repository.FullName < b.Repository.FullName
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Key < b.Key
	})
	return result, nil
}

func (s *OverviewService) workflows(snapshot RepositorySnapshot) []Workflow {
	branchScope := s.branch
	if branchScope == "default" {
		branchScope = snapshot.Repository.DefaultBranch
	}
	groups := make(map[string][]Run)
	for _, run := range snapshot.Runs {
		if !MatchesBranchScope(s.branch, snapshot.Repository, run) {
			continue
		}
		groups[run.WorkflowID] = append(groups[run.WorkflowID], run)
	}
	rows := make([]Workflow, 0, len(groups))
	for workflowID, runs := range groups {
		// A rerun replaces the previous attempt of the same provider run ID.
		latestByID := make(map[string]Run, len(runs))
		for _, run := range runs {
			prior, found := latestByID[run.ID]
			if !found || run.Attempt > prior.Attempt || (run.Attempt == prior.Attempt && run.CreatedAt.After(prior.CreatedAt)) {
				latestByID[run.ID] = run
			}
		}
		sample := make([]Run, 0, len(latestByID))
		for _, run := range latestByID {
			sample = append(sample, run)
		}
		sort.Slice(sample, func(i, j int) bool {
			if !sample[i].CreatedAt.Equal(sample[j].CreatedAt) {
				return sample[i].CreatedAt.Before(sample[j].CreatedAt)
			}
			return sample[i].ID < sample[j].ID
		})
		if s.historyRuns > 0 && len(sample) > s.historyRuns {
			sample = sample[len(sample)-s.historyRuns:]
		}
		if len(sample) == 0 {
			continue
		}
		owned := make([]Run, len(sample))
		for i, run := range sample {
			owned[i] = copyRun(run)
		}
		latest := copyRun(owned[len(owned)-1])
		name := latest.WorkflowName
		if name == "" {
			name = workflowID
		}
		row := Workflow{Key: stableKey(snapshot.Repository.Key, workflowID), Repository: snapshot.Repository, WorkflowID: workflowID, Name: name, BranchScope: branchScope, Runs: owned, LatestRun: &latest, Coverage: copyCoverage(snapshot.Coverage), Health: copyHealth(snapshot.Health)}
		success, eligible := 0, 0
		selectedCompleted := false
		for i := len(owned) - 1; i >= 0; i-- {
			run := owned[i]
			if run.Terminal && !selectedCompleted {
				selectedCompleted = true
				row.LastCompletedDurationMS = copyInt64(run.DurationMS)
				row.DurationKind = run.DurationKind
			}
			if run.Terminal && run.State == "success" {
				success++
				eligible++
			}
			if run.Terminal && run.State == "failure" {
				eligible++
			}
		}
		row.EligibleRuns = eligible
		if eligible > 0 {
			rate := float64(success) / float64(eligible)
			row.SuccessRate = &rate
		}
		rows = append(rows, row)
	}
	return rows
}

func stableKey(repo RepositoryKey, workflowID string) string {
	key := string(repo)
	return strconv.Itoa(len(key)) + ":" + key + strconv.Itoa(len(workflowID)) + ":" + workflowID
}
func copyTime(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	out := v.UTC()
	return &out
}
func copyInt64(v *int64) *int64 {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}
func copyRun(v Run) Run {
	v.CreatedAt = v.CreatedAt.UTC()
	v.StartedAt = copyTime(v.StartedAt)
	v.FinishedAt = copyTime(v.FinishedAt)
	v.DurationMS = copyInt64(v.DurationMS)
	return v
}
func copyCoverage(v Coverage) Coverage { v.OldestFetchedAt = copyTime(v.OldestFetchedAt); return v }
func copyHealth(v Health) Health {
	v.LastAttemptAt = v.LastAttemptAt.UTC()
	v.LastSuccessAt = copyTime(v.LastSuccessAt)
	v.RetryAt = copyTime(v.RetryAt)
	return v
}
