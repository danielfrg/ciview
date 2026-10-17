package mcp

import (
	"ciview/internal/ci"
	"context"
	"errors"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"sort"
	"strings"
	"time"
)

type Freshness struct {
	GeneratedAt         time.Time `json:"generated_at"`
	PollIntervalSeconds int       `json:"poll_interval_seconds"`
}
type Workflow struct {
	Key                     string        `json:"key"`
	Repository              ci.Repository `json:"repository"`
	WorkflowID              string        `json:"workflow_id"`
	WorkflowURL             string        `json:"workflow_url"`
	Name                    string        `json:"name"`
	BranchScope             string        `json:"branch_scope"`
	FileExists              *bool         `json:"file_exists"`
	LatestRun               *ci.Run       `json:"latest_run"`
	LastCompletedDurationMS *int64        `json:"last_completed_duration_ms"`
	DurationKind            string        `json:"duration_kind"`
	SuccessRate             *float64      `json:"success_rate"`
	EligibleRuns            int           `json:"eligible_runs"`
	Coverage                ci.Coverage   `json:"coverage"`
	Health                  ci.Health     `json:"health"`
}

func workflow(w ci.Workflow) Workflow {
	h := w.Health
	h.Message = ""
	if h.State != "" && h.State != "healthy" {
		h.Message = "Cached source status: " + h.State
	}
	return Workflow{w.Key, w.Repository, w.WorkflowID, w.WorkflowURL, w.Name, w.BranchScope, w.FileExists, w.LatestRun, w.LastCompletedDurationMS, w.DurationKind, w.SuccessRate, w.EligibleRuns, w.Coverage, h}
}

type ListInput struct {
	Paging
	Query      string `json:"query,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Source     string `json:"source,omitempty"`
	Repository string `json:"repository,omitempty"`
	Status     string `json:"status,omitempty"`
}
type ListOutput struct {
	Freshness
	Workflows  []Workflow `json:"workflows"`
	Total      int        `json:"total"`
	NextOffset *int       `json:"next_offset"`
}
type KeyInput struct {
	WorkflowKey string `json:"workflow_key"`
}
type WorkflowOutput struct {
	Freshness
	Workflow Workflow `json:"workflow"`
}
type RunsInput struct {
	Paging
	WorkflowKey string `json:"workflow_key"`
}
type RunsOutput struct {
	Freshness
	WorkflowKey string      `json:"workflow_key"`
	Runs        []ci.Run    `json:"runs"`
	Coverage    ci.Coverage `json:"coverage"`
	Health      ci.Health   `json:"health"`
	Total       int         `json:"total"`
	NextOffset  *int        `json:"next_offset"`
}
type SourceInput struct {
	Paging
	Source string `json:"source,omitempty"`
}
type Source struct {
	Name            string     `json:"name"`
	Provider        string     `json:"provider"`
	State           string     `json:"state"`
	RepositoryCount int        `json:"repository_count"`
	Issues          []ci.Issue `json:"issues"`
	IssuesTruncated bool       `json:"issues_truncated"`
	LastDiscoveryAt *time.Time `json:"last_discovery_at"`
}
type SourcesOutput struct {
	Freshness
	Sources    []Source `json:"sources"`
	Total      int      `json:"total"`
	NextOffset *int     `json:"next_offset"`
}

func freshness(v ci.Overview) Freshness { return Freshness{v.GeneratedAt, v.PollIntervalSeconds} }
func find(v ci.Overview, key string) (ci.Workflow, error) {
	for _, w := range v.Workflows {
		if w.Key == key {
			return w, nil
		}
	}
	return ci.Workflow{}, errors.New("Workflow not found")
}
func NewHandler(reader ci.OverviewReader, hosts []string) http.Handler {
	server := sdk.NewServer(&sdk.Implementation{Name: "ciview", Version: "0.1.0"}, nil)
	read := func(ctx context.Context) (ci.Overview, error) {
		if err := ctx.Err(); err != nil {
			return ci.Overview{}, errors.New("Overview request canceled")
		}
		v, e := reader.Overview(ctx)
		if e != nil {
			return ci.Overview{}, errors.New("Cached overview unavailable; try again")
		}
		return v, nil
	}
	sdk.AddTool(server, &sdk.Tool{Name: "list_workflows", Description: "Search cached workflow summaries. Filters combine; repository matches its exact key or full name, status matches the latest normalized run state.", Annotations: annotation()}, func(ctx context.Context, _ *sdk.CallToolRequest, in ListInput) (*sdk.CallToolResult, ListOutput, error) {
		out := ListOutput{Workflows: []Workflow{}}
		if _, _, _, err := in.Paging.bounds(0); err != nil {
			return nil, out, err
		}
		v, e := read(ctx)
		if e != nil {
			return nil, out, e
		}
		out.Freshness = freshness(v)
		items := []ci.Workflow{}
		words := strings.Fields(strings.ToLower(in.Query))
		for _, w := range v.Workflows {
			repo := w.Repository
			if in.Provider != "" && repo.Provider != in.Provider || in.Source != "" && repo.Source != in.Source || in.Repository != "" && in.Repository != string(repo.Key) && in.Repository != repo.FullName {
				continue
			}
			if in.Status != "" && (w.LatestRun == nil || w.LatestRun.State != in.Status) {
				continue
			}
			haystack := strings.ToLower(strings.Join([]string{w.Name, w.WorkflowID, repo.FullName, repo.Source, repo.Provider, w.BranchScope}, " "))
			matches := true
			for _, word := range words {
				if !strings.Contains(haystack, word) {
					matches = false
					break
				}
			}
			if !matches {
				continue
			}
			items = append(items, w)
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
		a, b, next, _ := in.Paging.bounds(len(items))
		out.Total = len(items)
		out.NextOffset = next
		for _, w := range items[a:b] {
			out.Workflows = append(out.Workflows, workflow(w))
		}
		return nil, out, nil
	})
	sdk.AddTool(server, &sdk.Tool{Name: "get_workflow", Description: "Get cached workflow identity, latest run, metrics, partial-history coverage and source health. Does not fetch providers.", Annotations: annotation()}, func(ctx context.Context, _ *sdk.CallToolRequest, in KeyInput) (*sdk.CallToolResult, WorkflowOutput, error) {
		v, e := read(ctx)
		if e != nil {
			return nil, WorkflowOutput{}, e
		}
		w, e := find(v, in.WorkflowKey)
		return nil, WorkflowOutput{freshness(v), workflow(w)}, e
	})
	sdk.AddTool(server, &sdk.Tool{Name: "list_runs", Description: "Read bounded cached history, newest first. Missing duration remains null; coverage may be partial.", Annotations: annotation()}, func(ctx context.Context, _ *sdk.CallToolRequest, in RunsInput) (*sdk.CallToolResult, RunsOutput, error) {
		out := RunsOutput{Runs: []ci.Run{}}
		if _, _, _, err := in.Paging.bounds(0); err != nil {
			return nil, out, err
		}
		v, e := read(ctx)
		if e != nil {
			return nil, out, e
		}
		w, e := find(v, in.WorkflowKey)
		if e != nil {
			return nil, out, e
		}
		runs := append([]ci.Run{}, w.Runs...)
		sort.Slice(runs, func(i, j int) bool {
			a, b := runs[i], runs[j]
			if !a.CreatedAt.Equal(b.CreatedAt) {
				return a.CreatedAt.After(b.CreatedAt)
			}
			if a.ID != b.ID {
				return a.ID < b.ID
			}
			return a.Attempt > b.Attempt
		})
		a, b, next, _ := in.Paging.bounds(len(runs))
		out.Freshness = freshness(v)
		out.WorkflowKey = w.Key
		out.Runs = runs[a:b]
		out.Coverage = w.Coverage
		out.Health = workflow(w).Health
		out.Total = len(runs)
		out.NextOffset = next
		return nil, out, nil
	})
	sdk.AddTool(server, &sdk.Tool{Name: "get_source_status", Description: "Read source discovery state and freshness with at most 20 sanitized issues per source. No refresh is triggered.", Annotations: annotation()}, func(ctx context.Context, _ *sdk.CallToolRequest, in SourceInput) (*sdk.CallToolResult, SourcesOutput, error) {
		out := SourcesOutput{Sources: []Source{}}
		if _, _, _, err := in.Paging.bounds(0); err != nil {
			return nil, out, err
		}
		v, e := read(ctx)
		if e != nil {
			return nil, out, e
		}
		out.Freshness = freshness(v)
		items := []ci.SourceState{}
		for _, s := range v.Sources {
			if in.Source == "" || s.Name == in.Source {
				items = append(items, s)
			}
		}
		if in.Source != "" && len(items) == 0 {
			return nil, out, errors.New("Source not found")
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
		a, b, next, _ := in.Paging.bounds(len(items))
		out.Total = len(items)
		out.NextOffset = next
		for _, s := range items[a:b] {
			issues := []ci.Issue{}
			for _, issue := range s.Issues[:min(20, len(s.Issues))] {
				issues = append(issues, ci.Issue{Repository: issue.Repository, Kind: issue.Kind, Message: "Source discovery issue"})
			}
			out.Sources = append(out.Sources, Source{s.Name, s.Provider, s.State, len(s.Repositories), issues, len(s.Issues) > 20, s.LastDiscoveryAt})
		}
		return nil, out, nil
	})
	return transport(server, hosts)
}
