package snapshots

import (
	"context"
	"sync"
	"testing"
	"time"

	"civiewer/internal/ci"
)

func TestMemoryReadWriteOwnsValuesAndReplacesAtomically(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	if _, ok, err := store.Read(ctx, "source/repo"); err != nil || ok {
		t.Fatalf("missing read: %v %v", ok, err)
	}
	started := time.Date(2026, 10, 14, 1, 2, 3, 0, time.UTC)
	duration := int64(7000)
	input := ci.RepositorySnapshot{Repository: ci.Repository{Key: "source/repo"}, Runs: []ci.Run{{ID: "one", StartedAt: &started, DurationMS: &duration}}, Coverage: ci.Coverage{OldestFetchedAt: &started}}
	if err := store.Write(ctx, input); err != nil {
		t.Fatal(err)
	}
	input.Runs[0].ID = "mutated"
	*input.Runs[0].DurationMS = 1
	got, ok, err := store.Read(ctx, "source/repo")
	if err != nil || !ok {
		t.Fatalf("read: %v %v", ok, err)
	}
	if got.Runs[0].ID != "one" || *got.Runs[0].DurationMS != 7000 {
		t.Fatalf("write retained caller memory: %+v", got.Runs)
	}
	got.Runs[0].ID = "changed"
	*got.Runs[0].StartedAt = time.Time{}
	listed, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Runs[0].ID != "one" || listed[0].Runs[0].StartedAt.IsZero() {
		t.Fatalf("read/list leaked memory: %+v", listed)
	}
	if err := store.Write(ctx, ci.RepositorySnapshot{Repository: ci.Repository{Key: "source/repo"}, Runs: []ci.Run{{ID: "two"}}}); err != nil {
		t.Fatal(err)
	}
	listed, err = store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || len(listed[0].Runs) != 1 || listed[0].Runs[0].ID != "two" {
		t.Fatalf("replacement not atomic: %+v", listed)
	}
}

func TestMemoryConcurrentReadersAndWriters(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				snap := ci.RepositorySnapshot{Repository: ci.Repository{Key: ci.RepositoryKey("repo")}, Runs: []ci.Run{{ID: "run"}}}
				if err := store.Write(ctx, snap); err != nil {
					t.Error(err)
				}
				got, _, err := store.Read(ctx, "repo")
				if err != nil {
					t.Error(err)
				}
				if len(got.Runs) > 0 {
					got.Runs[0].ID = "changed"
				}
				list, err := store.List(ctx)
				if err != nil {
					t.Error(err)
				}
				if len(list) > 0 && len(list[0].Runs) > 0 {
					list[0].Runs[0].ID = "changed"
				}
			}
		}(n)
	}
	wg.Wait()
	got, ok, err := store.Read(ctx, "repo")
	if err != nil || !ok || got.Runs[0].ID != "run" {
		t.Fatalf("concurrent value corruption: %+v %v", got, err)
	}
}

func TestMemoryNormalizesStoredTimestampsToUTC(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	local := time.Date(2026, 10, 14, 8, 0, 0, 0, time.FixedZone("offset", -5*60*60))
	snap := ci.RepositorySnapshot{Repository: ci.Repository{Key: "repo"}, Runs: []ci.Run{{CreatedAt: local, StartedAt: &local}}, Coverage: ci.Coverage{OldestFetchedAt: &local}, Health: ci.Health{LastAttemptAt: local, LastSuccessAt: &local}}
	if err := store.Write(ctx, snap); err != nil {
		t.Fatal(err)
	}
	got, _, err := store.Read(ctx, "repo")
	if err != nil {
		t.Fatal(err)
	}
	if got.Runs[0].CreatedAt.Location() != time.UTC || got.Runs[0].StartedAt.Location() != time.UTC || got.Coverage.OldestFetchedAt.Location() != time.UTC || got.Health.LastAttemptAt.Location() != time.UTC || got.Health.LastSuccessAt.Location() != time.UTC {
		t.Fatalf("non-UTC storage timestamps: %+v", got)
	}
	if got.Runs[0].CreatedAt.Hour() != 13 {
		t.Fatalf("timestamp instant changed: %v", got.Runs[0].CreatedAt)
	}
}
