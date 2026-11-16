package ci

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseRunnerScope(t *testing.T) {
	for _, tc := range []struct {
		raw       string
		org, repo string
		ok        bool
	}{
		{"org:inmatura", "inmatura", "", true},
		{"repo:danielfrg/homelab", "", "danielfrg/homelab", true},
		{"org:", "", "", false},
		{"org:a/b", "", "", false},
		{"repo:onlyowner", "", "", false},
		{"repo:a/b/c", "", "", false},
		{"repo:../x", "", "", false},
		{"team:x", "", "", false},
		{"inmatura", "", "", false},
	} {
		target, err := ParseRunnerScope(tc.raw)
		if (err == nil) != tc.ok || target.Org != tc.org || target.Repo != tc.repo {
			t.Errorf("%q => %+v, %v", tc.raw, target, err)
		}
	}
}

type fakeRunners struct{ value []RunnerScope }

func (f fakeRunners) Runners(context.Context) ([]RunnerScope, error) { return f.value, nil }

func TestOverviewIncludesRunnersAndSerializesEmptyLists(t *testing.T) {
	asOf := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	service := NewOverview(emptyStore{}, emptyInventory{}, 30, "default", time.Minute).WithRunners(fakeRunners{[]RunnerScope{{
		Scope: "org:inmatura", Source: "github", State: "healthy", AsOf: &asOf,
		Runners: []Runner{{ID: "7", Name: "r1", Status: "online", Busy: true}},
	}}})
	overview, err := service.Overview(context.Background())
	if err != nil || len(overview.Runners) != 1 || overview.Runners[0].Runners[0].Name != "r1" {
		t.Fatalf("overview=%+v err=%v", overview, err)
	}
	raw, _ := json.Marshal(overview)
	if !strings.Contains(string(raw), `"labels":[]`) || !strings.Contains(string(raw), `"runners":[{"scope":"org:inmatura"`) {
		t.Fatalf("json=%s", raw)
	}
	bare, _ := NewOverview(emptyStore{}, emptyInventory{}, 30, "default", time.Minute).Overview(context.Background())
	if raw, _ := json.Marshal(bare); !strings.Contains(string(raw), `"runners":[]`) {
		t.Fatalf("no runner reader must serialize an empty list: %s", raw)
	}
}

type emptyStore struct{}

func (emptyStore) Read(context.Context, RepositoryKey) (RepositorySnapshot, bool, error) {
	return RepositorySnapshot{}, false, nil
}
func (emptyStore) Write(context.Context, RepositorySnapshot) error { return nil }
func (emptyStore) List(context.Context) ([]RepositorySnapshot, error) {
	return nil, nil
}

type emptyInventory struct{}

func (emptyInventory) Inventory(context.Context) (Inventory, error) { return Inventory{}, nil }
