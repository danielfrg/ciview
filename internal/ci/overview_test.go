package ci_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"civiewer/internal/ci"
	"civiewer/internal/snapshots"
)

type fixtureInventory struct{ state ci.Inventory }

func (f fixtureInventory) Inventory(context.Context) (ci.Inventory, error) { return f.state, nil }

func TestOverviewUsesActiveInventoryAndHonestSampleMetrics(t *testing.T) {
	ctx := context.Background()
	store := snapshots.NewMemory()
	base := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	ms := int64(9100)
	repo := ci.Repository{Key: "github/12", Source: "github", Provider: "github", ID: "12", Owner: "acme", Name: "app", FullName: "acme/app", DefaultBranch: "main"}
	runs := []ci.Run{
		{ID: "1", WorkflowID: "wf", WorkflowName: "checks", Branch: "main", CreatedAt: base.Add(time.Hour), State: "success", Terminal: true, DurationMS: &ms, DurationKind: "execution"},
		{ID: "2", WorkflowID: "wf", WorkflowName: "checks", Branch: "main", CreatedAt: base.Add(2 * time.Hour), State: "canceled", Terminal: true},
		{ID: "3", WorkflowID: "wf", WorkflowName: "checks", Branch: "main", CreatedAt: base.Add(3 * time.Hour), State: "failure", Terminal: true},
		{ID: "4", WorkflowID: "wf", WorkflowName: "checks", Branch: "main", CreatedAt: base.Add(4 * time.Hour), State: "running", Terminal: false},
		{ID: "5", WorkflowID: "wf", WorkflowName: "checks", Branch: "feature", CreatedAt: base.Add(5 * time.Hour), State: "success", Terminal: true},
		{ID: "6", WorkflowID: "wf", WorkflowName: "checks", Branch: "main", CreatedAt: base.Add(6 * time.Hour), State: "success", Terminal: true},
		{ID: "6", WorkflowID: "wf", WorkflowName: "checks", Branch: "main", CreatedAt: base.Add(6 * time.Hour), Attempt: 2, State: "running", Terminal: false},
	}
	if err := store.Write(ctx, ci.RepositorySnapshot{Repository: repo, Runs: runs}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, ci.RepositorySnapshot{Repository: ci.Repository{Key: "removed", Source: "github"}, Runs: []ci.Run{{WorkflowID: "old", ID: "old"}}}); err != nil {
		t.Fatal(err)
	}
	inventory := fixtureInventory{ci.Inventory{Sources: []ci.SourceState{{Name: "github", Provider: "github", State: "partial", Repositories: []ci.RepositoryKey{"github/12", "loading"}, Issues: []ci.Issue{{Repository: "acme/private", Kind: "unavailable", Message: "Permission denied"}}}}}}
	result, err := ci.NewOverview(store, inventory, 5, "default", time.Minute).Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Workflows) != 1 || len(result.Sources) != 1 || len(result.Sources[0].Issues) != 2 {
		t.Fatalf("active rows or issues wrong: %+v", result)
	}
	if result.Sources[0].Issues[1].Kind != "loading" || result.Sources[0].Issues[1].Repository != "loading" {
		t.Fatalf("missing active repository loading notice: %+v", result.Sources[0].Issues)
	}
	w := result.Workflows[0]
	if w.Key == "" || w.BranchScope != "main" || w.LatestRun == nil || w.LatestRun.ID != "6" || w.LatestRun.Attempt != 2 || w.LatestRun.State != "running" {
		t.Fatalf("latest run/identity wrong: %+v", w)
	}
	if len(w.Runs) != 5 || w.Runs[0].ID != "1" || w.Runs[4].ID != "6" {
		t.Fatalf("sample order/rerun wrong: %+v", w.Runs)
	}
	if w.EligibleRuns != 2 || w.SuccessRate == nil || *w.SuccessRate != 0.5 {
		t.Fatalf("rate denominator wrong: %v/%v", w.SuccessRate, w.EligibleRuns)
	}
	if w.LastCompletedDurationMS != nil {
		t.Fatalf("used older duration: %v", *w.LastCompletedDurationMS)
	}
	if result.PollIntervalSeconds != 60 || result.GeneratedAt.Location() != time.UTC {
		t.Fatalf("overview metadata: %+v", result)
	}
}

func TestOverviewGroupsStableKeysAndNullEmptyMetrics(t *testing.T) {
	ctx := context.Background()
	store := snapshots.NewMemory()
	base := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	for _, repo := range []ci.Repository{{Key: "source-a/repo", Source: "a", FullName: "same/app", DefaultBranch: "main"}, {Key: "source-b/repo", Source: "b", FullName: "same/app", DefaultBranch: "main"}} {
		if err := store.Write(ctx, ci.RepositorySnapshot{Repository: repo, Runs: []ci.Run{{ID: "1", WorkflowID: "wf", WorkflowName: "build", Branch: "main", CreatedAt: base, State: "skipped", Terminal: true}}}); err != nil {
			t.Fatal(err)
		}
	}
	inventory := fixtureInventory{ci.Inventory{Sources: []ci.SourceState{{Name: "a", Repositories: []ci.RepositoryKey{"source-a/repo"}}, {Name: "b", Repositories: []ci.RepositoryKey{"source-b/repo"}}}}}
	result, err := ci.NewOverview(store, inventory, 30, "all", time.Minute).Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Workflows) != 2 || result.Workflows[0].Key == result.Workflows[1].Key {
		t.Fatalf("group collision: %+v", result.Workflows)
	}
	for _, w := range result.Workflows {
		if w.SuccessRate != nil || w.EligibleRuns != 0 || w.LastCompletedDurationMS != nil || w.BranchScope != "all" {
			t.Fatalf("empty metrics wrong: %+v", w)
		}
	}
}

func TestOverviewBranchScopeUsesHeadRepositoryIdentity(t *testing.T) {
	for _, tc := range []struct{ scope, branch string }{{"default", "main"}, {"release", "release"}, {"all", "main"}} {
		t.Run(tc.scope, func(t *testing.T) {
			ctx := context.Background()
			store := snapshots.NewMemory()
			repo := ci.Repository{Key: "source/repo", Provider: "github", ID: "42", DefaultBranch: "main"}
			var runs []ci.Run
			payload := fmt.Sprintf(`[
				{"id":"push","workflow_id":"checks","branch":%q,"event":"push","created_at":"2026-10-14T12:00:00Z","terminal":true,"state":"success","duration_ms":500,"duration_kind":"job_span"},
				{"id":"local-pr","workflow_id":"checks","branch":%q,"head_repository_id":"42","event":"pull_request","created_at":"2026-10-14T12:01:00Z","terminal":true,"state":"success","duration_ms":1000,"duration_kind":"job_span"},
				{"id":"fork-pr","workflow_id":"checks","branch":%q,"head_repository_id":"99","event":"pull_request","url":"https://github.com/acme/project/actions/runs/3","created_at":"2026-10-14T12:02:00Z","terminal":true,"state":"failure","duration_ms":9000,"duration_kind":"job_span"},
				{"id":"unknown-pr","workflow_id":"checks","branch":%q,"event":"pull_request","created_at":"2026-10-14T12:03:00Z","terminal":true,"state":"attention"}
			]`, tc.branch, tc.branch, tc.branch, tc.branch)
			if err := json.Unmarshal([]byte(payload), &runs); err != nil {
				t.Fatal(err)
			}
			if err := store.Write(ctx, ci.RepositorySnapshot{Repository: repo, Runs: runs}); err != nil {
				t.Fatal(err)
			}
			inventory := fixtureInventory{ci.Inventory{Sources: []ci.SourceState{{Repositories: []ci.RepositoryKey{repo.Key}}}}}
			result, err := ci.NewOverview(store, inventory, 10, tc.scope, time.Minute).Overview(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Workflows) != 1 {
				t.Fatalf("rows=%#v", result.Workflows)
			}
			row := result.Workflows[0]
			if tc.scope == "all" {
				if len(row.Runs) != 4 || row.LatestRun == nil || row.LatestRun.ID != "unknown-pr" || row.EligibleRuns != 3 || row.SuccessRate == nil || *row.SuccessRate != 2.0/3.0 {
					t.Fatalf("all scope discarded PR history: %#v", row)
				}
				if row.Runs[2].ID != "fork-pr" || row.Runs[2].Branch != "main" || row.Runs[2].URL != "https://github.com/acme/project/actions/runs/3" {
					t.Fatalf("all scope changed original PR branch or link: %#v", row.Runs)
				}
			} else if len(row.Runs) != 2 || row.Runs[0].ID != "push" || row.Runs[1].ID != "local-pr" || row.LatestRun == nil || row.LatestRun.ID != "local-pr" || row.EligibleRuns != 2 || row.SuccessRate == nil || *row.SuccessRate != 1 || row.LastCompletedDurationMS == nil || *row.LastCompletedDurationMS != 1000 {
				t.Fatalf("foreign or unknown PR contaminated branch metrics: %#v", row)
			}
		})
	}
}

func TestOverviewUnknownPullRequestProvenanceIsConservative(t *testing.T) {
	for _, event := range []string{"pull_request", "pull_request_target", "pull_request_review", "pull_request_review_comment", "push", ""} {
		for _, repoID := range []string{"42", ""} {
			t.Run(event+"/repository="+repoID, func(t *testing.T) {
				ctx := context.Background()
				store := snapshots.NewMemory()
				repo := ci.Repository{Key: "source/repo", Provider: "github", ID: repoID, DefaultBranch: "main"}
				run := ci.Run{ID: "1", WorkflowID: "checks", Branch: "main", Event: event, Terminal: true, State: "success"}
				if err := store.Write(ctx, ci.RepositorySnapshot{Repository: repo, Runs: []ci.Run{run}}); err != nil {
					t.Fatal(err)
				}
				inventory := fixtureInventory{ci.Inventory{Sources: []ci.SourceState{{Repositories: []ci.RepositoryKey{repo.Key}}}}}
				result, err := ci.NewOverview(store, inventory, 10, "default", time.Minute).Overview(ctx)
				if err != nil {
					t.Fatal(err)
				}
				wantRows := 0
				if event == "push" || event == "" {
					wantRows = 1
				}
				if len(result.Workflows) != wantRows {
					t.Fatalf("unknown provenance branch rows=%d want=%d", len(result.Workflows), wantRows)
				}
			})
		}
	}
}
