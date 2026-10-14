package ci

import (
	"context"
	"encoding/json"
	"time"
)

type RepositoryKey string

type Repository struct {
	Key           RepositoryKey `json:"key"`
	Source        string        `json:"source"`
	Provider      string        `json:"provider"`
	ID            string        `json:"id"`
	Owner         string        `json:"owner"`
	Name          string        `json:"name"`
	FullName      string        `json:"full_name"`
	DefaultBranch string        `json:"default_branch"`
	URL           string        `json:"url"`
}

type Run struct {
	ID               string     `json:"id"`
	WorkflowID       string     `json:"workflow_id"`
	WorkflowName     string     `json:"workflow_name"`
	WorkflowURL      string     `json:"workflow_url,omitempty"`
	Title            string     `json:"title"`
	Branch           string     `json:"branch"`
	HeadRepositoryID string     `json:"head_repository_id,omitempty"`
	Event            string     `json:"event"`
	URL              string     `json:"url"`
	Attempt          int        `json:"attempt"`
	State            string     `json:"state"`
	Terminal         bool       `json:"terminal"`
	RawStatus        string     `json:"raw_status"`
	RawConclusion    string     `json:"raw_conclusion"`
	CreatedAt        time.Time  `json:"created_at"`
	StartedAt        *time.Time `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at"`
	DurationMS       *int64     `json:"duration_ms"`
	DurationKind     string     `json:"duration_kind"`
}

type Coverage struct {
	Partial         bool       `json:"partial"`
	FetchedRuns     int        `json:"fetched_runs"`
	OldestFetchedAt *time.Time `json:"oldest_fetched_at"`
}

type Health struct {
	State         string     `json:"state"`
	Message       string     `json:"message"`
	LastAttemptAt time.Time  `json:"last_attempt_at"`
	LastSuccessAt *time.Time `json:"last_success_at"`
	RetryAt       *time.Time `json:"retry_at"`
}

type RepositorySnapshot struct {
	Repository Repository `json:"repository"`
	Runs       []Run      `json:"runs"`
	Coverage   Coverage   `json:"coverage"`
	Health     Health     `json:"health"`
}

type Issue struct {
	Repository string `json:"repository"`
	Kind       string `json:"kind"`
	Message    string `json:"message"`
}

type SourceState struct {
	Name            string          `json:"name"`
	Provider        string          `json:"provider"`
	State           string          `json:"state"`
	Repositories    []RepositoryKey `json:"repositories"`
	Issues          []Issue         `json:"issues"`
	LastDiscoveryAt *time.Time      `json:"last_discovery_at"`
}

type Inventory struct {
	Sources []SourceState `json:"sources"`
}
type Selection struct {
	Orgs  []string `json:"orgs"`
	Users []string `json:"users"`
	Repos []string `json:"repos"`
}
type Discovery struct {
	Repositories []Repository `json:"repositories"`
	Issues       []Issue      `json:"issues"`
	Complete     bool         `json:"complete"`
}
type RunPage struct {
	Runs []Run `json:"runs"`
	Next bool  `json:"next"`
}

type Provider interface {
	Discover(context.Context, Selection) (Discovery, error)
	ListRuns(context.Context, Repository, int) (RunPage, error)
	GetRun(context.Context, Repository, string) (Run, error)
	EnrichDuration(context.Context, Repository, Run) (Run, error)
}

type ProviderError struct {
	Kind, Message string
	RetryAt       *time.Time
}

func (e *ProviderError) Error() string {
	if e == nil {
		return "provider request failed"
	}
	switch e.Kind {
	case "authentication", "auth", "unauthorized":
		return "provider authentication failed"
	case "permission", "forbidden":
		return "provider permission denied"
	case "not_found", "missing":
		return "provider repository or run not found"
	case "rate_limit", "rate_limited":
		return "provider rate limit reached"
	case "unavailable", "upstream":
		return "provider unavailable"
	default:
		return "provider request failed"
	}
}

type SnapshotStore interface {
	Read(context.Context, RepositoryKey) (RepositorySnapshot, bool, error)
	Write(context.Context, RepositorySnapshot) error
	List(context.Context) ([]RepositorySnapshot, error)
}

type InventoryReader interface {
	Inventory(context.Context) (Inventory, error)
}

type Workflow struct {
	Key                     string     `json:"key"`
	Repository              Repository `json:"repository"`
	WorkflowID              string     `json:"workflow_id"`
	WorkflowURL             string     `json:"workflow_url"`
	Name                    string     `json:"name"`
	BranchScope             string     `json:"branch_scope"`
	Runs                    []Run      `json:"runs"`
	LatestRun               *Run       `json:"latest_run"`
	LastCompletedDurationMS *int64     `json:"last_completed_duration_ms"`
	DurationKind            string     `json:"duration_kind"`
	SuccessRate             *float64   `json:"success_rate"`
	EligibleRuns            int        `json:"eligible_runs"`
	Coverage                Coverage   `json:"coverage"`
	Health                  Health     `json:"health"`
}

type Overview struct {
	Workflows           []Workflow    `json:"workflows"`
	Sources             []SourceState `json:"sources"`
	GeneratedAt         time.Time     `json:"generated_at"`
	PollIntervalSeconds int           `json:"poll_interval_seconds"`
}

type OverviewReader interface {
	Overview(context.Context) (Overview, error)
}

func nonNil[T any](values []T) []T {
	if values == nil {
		return make([]T, 0)
	}
	return values
}

func (v RepositorySnapshot) MarshalJSON() ([]byte, error) {
	type plain RepositorySnapshot
	v.Runs = nonNil(v.Runs)
	return json.Marshal(plain(v))
}

func (v SourceState) MarshalJSON() ([]byte, error) {
	type plain SourceState
	v.Repositories = nonNil(v.Repositories)
	v.Issues = nonNil(v.Issues)
	return json.Marshal(plain(v))
}

func (v Inventory) MarshalJSON() ([]byte, error) {
	type plain Inventory
	v.Sources = nonNil(v.Sources)
	return json.Marshal(plain(v))
}

func (v Selection) MarshalJSON() ([]byte, error) {
	type plain Selection
	v.Orgs = nonNil(v.Orgs)
	v.Users = nonNil(v.Users)
	v.Repos = nonNil(v.Repos)
	return json.Marshal(plain(v))
}

func (v Discovery) MarshalJSON() ([]byte, error) {
	type plain Discovery
	v.Repositories = nonNil(v.Repositories)
	v.Issues = nonNil(v.Issues)
	return json.Marshal(plain(v))
}

func (v RunPage) MarshalJSON() ([]byte, error) {
	type plain RunPage
	v.Runs = nonNil(v.Runs)
	return json.Marshal(plain(v))
}

func (v Workflow) MarshalJSON() ([]byte, error) {
	type plain Workflow
	v.Runs = nonNil(v.Runs)
	return json.Marshal(plain(v))
}

func (v Overview) MarshalJSON() ([]byte, error) {
	type plain Overview
	v.Workflows = nonNil(v.Workflows)
	v.Sources = nonNil(v.Sources)
	return json.Marshal(plain(v))
}
