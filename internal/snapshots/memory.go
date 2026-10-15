package snapshots

import (
	"context"
	"sort"
	"sync"
	"time"

	"ciview/internal/ci"
)

// Memory stores one atomically replaceable, independently owned value per repository.
type Memory struct {
	mu     sync.RWMutex
	values map[ci.RepositoryKey]ci.RepositorySnapshot
}

func NewMemory() *Memory { return &Memory{values: make(map[ci.RepositoryKey]ci.RepositorySnapshot)} }

func (m *Memory) Read(ctx context.Context, key ci.RepositoryKey) (ci.RepositorySnapshot, bool, error) {
	if err := ctx.Err(); err != nil {
		return ci.RepositorySnapshot{}, false, err
	}
	m.mu.RLock()
	value, ok := m.values[key]
	owned := cloneSnapshot(value)
	m.mu.RUnlock()
	return owned, ok, nil
}

func (m *Memory) Write(ctx context.Context, value ci.RepositorySnapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	owned := cloneSnapshot(value)
	m.mu.Lock()
	m.values[value.Repository.Key] = owned
	m.mu.Unlock()
	return nil
}

func (m *Memory) List(ctx context.Context) ([]ci.RepositorySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	values := make([]ci.RepositorySnapshot, 0, len(m.values))
	for _, value := range m.values {
		values = append(values, cloneSnapshot(value))
	}
	m.mu.RUnlock()
	sort.Slice(values, func(i, j int) bool { return values[i].Repository.Key < values[j].Repository.Key })
	return values, nil
}

func cloneSnapshot(v ci.RepositorySnapshot) ci.RepositorySnapshot {
	out := v
	out.Runs = make([]ci.Run, len(v.Runs))
	for i, run := range v.Runs {
		out.Runs[i] = run
		out.Runs[i].CreatedAt = run.CreatedAt.UTC()
		out.Runs[i].StartedAt = cloneTime(run.StartedAt)
		out.Runs[i].FinishedAt = cloneTime(run.FinishedAt)
		out.Runs[i].DurationMS = cloneInt64(run.DurationMS)
	}
	out.Coverage.OldestFetchedAt = cloneTime(v.Coverage.OldestFetchedAt)
	out.Health.LastAttemptAt = v.Health.LastAttemptAt.UTC()
	out.Health.LastSuccessAt = cloneTime(v.Health.LastSuccessAt)
	out.Health.RetryAt = cloneTime(v.Health.RetryAt)
	return out
}
func cloneTime(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	out := v.UTC()
	return &out
}
func cloneInt64(v *int64) *int64 {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}
