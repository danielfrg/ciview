// Package github normalizes the GitHub Actions REST API.
package github

import (
	"ciview/internal/ci"
	"ciview/internal/providers/rest"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Client struct {
	transport *rest.Transport
	mu        sync.Mutex
	timings   map[ci.RepositoryKey]timing
}
type timing struct {
	duration   *int64
	start, end *time.Time
}

var _ ci.Provider = (*Client)(nil)

func New(sourceName, baseURL, token string, client *http.Client) (*Client, error) {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	t, e := rest.New(sourceName, "github", baseURL, token, "Bearer", client)
	if e != nil {
		return nil, e
	}
	return &Client{transport: t, timings: map[ci.RepositoryKey]timing{}}, nil
}
func (c *Client) Discover(ctx context.Context, s ci.Selection) (ci.Discovery, error) {
	return c.transport.Discover(ctx, s, 100)
}

type runResponse struct {
	ID             json.Number `json:"id"`
	WorkflowID     json.Number `json:"workflow_id"`
	Name           string      `json:"name"`
	Path           string      `json:"path"`
	DisplayTitle   string      `json:"display_title"`
	HeadBranch     string      `json:"head_branch"`
	HeadRepository *struct {
		ID json.Number `json:"id"`
	} `json:"head_repository"`
	Event      string `json:"event"`
	HTMLURL    string `json:"html_url"`
	RunAttempt int    `json:"run_attempt"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	CreatedAt  string `json:"created_at"`
	StartedAt  string `json:"run_started_at"`
}

func normalize(repo ci.Repository, v runResponse) (ci.Run, error) {
	if n, e := strconv.ParseUint(v.ID.String(), 10, 64); e != nil || n == 0 {
		return ci.Run{}, rest.Error("invalid_data", "Provider run identity was invalid")
	}
	workflow := v.WorkflowID.String()
	if workflow == "" || workflow == "0" {
		workflow = v.Path
	}
	if workflow == "" {
		return ci.Run{}, rest.Error("invalid_data", "Provider workflow identity was unavailable")
	}
	name := v.Name
	if name == "" {
		name = v.Path
	}
	if name == "" {
		name = workflow
	}
	attempt := v.RunAttempt
	if attempt < 1 {
		attempt = 1
	}
	headRepositoryID := ""
	if v.HeadRepository != nil {
		if n, err := strconv.ParseUint(v.HeadRepository.ID.String(), 10, 64); err == nil && n > 0 {
			headRepositoryID = v.HeadRepository.ID.String()
		}
	}
	r := ci.Run{ID: v.ID.String(), WorkflowID: workflow, WorkflowName: name, Title: v.DisplayTitle, Branch: v.HeadBranch, HeadRepositoryID: headRepositoryID, Event: v.Event, URL: rest.SafeURL(v.HTMLURL), WorkflowURL: rest.WorkflowURL(repo.URL, "github", v.Path), Attempt: attempt, RawStatus: v.Status, RawConclusion: v.Conclusion, StartedAt: rest.Timestamp(v.StartedAt)}
	if created := rest.Timestamp(v.CreatedAt); created != nil {
		r.CreatedAt = *created
	}
	r.State, r.Terminal = state(v.Status, v.Conclusion)
	return r, nil
}
func state(status, conclusion string) (string, bool) {
	if status == "completed" {
		switch conclusion {
		case "success":
			return "success", true
		case "failure", "timed_out", "startup_failure":
			return "failure", true
		case "cancelled":
			return "canceled", true
		case "skipped":
			return "skipped", true
		case "neutral":
			return "neutral", true
		case "action_required", "stale":
			return "attention", true
		default:
			return "unknown", true
		}
	}
	switch status {
	case "queued", "pending", "requested":
		return "queued", false
	case "waiting":
		return "waiting", false
	case "in_progress":
		return "running", false
	default:
		return "unknown", false
	}
}
func (c *Client) ListRuns(ctx context.Context, r ci.Repository, page int) (ci.RunPage, error) {
	if page < 1 {
		return ci.RunPage{}, rest.Error("invalid_data", "Provider page must be positive")
	}
	p, e := rest.RepoPath(r)
	if e != nil {
		return ci.RunPage{}, e
	}
	target, e := c.transport.Endpoint(p+"/actions/runs", rest.PageQuery("github", page, 100))
	if e != nil {
		return ci.RunPage{}, e
	}
	var body struct {
		Runs  []runResponse `json:"workflow_runs"`
		Total *int          `json:"total_count"`
	}
	meta, e := c.transport.Get(ctx, target, &body)
	if e != nil {
		return ci.RunPage{}, e
	}
	if body.Runs == nil {
		return ci.RunPage{}, rest.Error("invalid_data", "Provider run page was invalid")
	}
	out := ci.RunPage{Runs: []ci.Run{}}
	for _, v := range body.Runs {
		run, e := normalize(r, v)
		if e != nil {
			return ci.RunPage{}, e
		}
		out.Runs = append(out.Runs, run)
	}
	total := -1
	if body.Total != nil {
		total = *body.Total
	}
	out.Next, e = c.transport.HasNext(meta, page, 100, len(body.Runs), total)
	return out, e
}
func (c *Client) GetRun(ctx context.Context, r ci.Repository, id string) (ci.Run, error) {
	p, e := rest.RepoPath(r)
	if e != nil {
		return ci.Run{}, e
	}
	if _, e = rest.Segment(id); e != nil {
		return ci.Run{}, e
	}
	target, e := c.transport.Endpoint(p+"/actions/runs/"+id, nil)
	if e != nil {
		return ci.Run{}, e
	}
	var v runResponse
	if _, e = c.transport.Get(ctx, target, &v); e != nil {
		return ci.Run{}, e
	}
	return normalize(r, v)
}

// EnrichDuration computes execution span only from the selected attempt's jobs.
// Successful timing is cached by source repository, run, and attempt; unsuccessful
// fetches never cache an error or modify the caller's run value.
func (c *Client) EnrichDuration(ctx context.Context, repo ci.Repository, run ci.Run) (ci.Run, error) {
	original := run
	run.DurationMS = nil
	run.DurationKind = ""
	run.FinishedAt = nil
	if !run.Terminal {
		return run, nil
	}
	if run.Attempt < 1 {
		run.Attempt = 1
	}
	key := rest.StableKey(string(repo.Key), repo.Owner, repo.Name, run.ID, strconv.Itoa(run.Attempt))
	c.mu.Lock()
	cached, ok := c.timings[key]
	c.mu.Unlock()
	if ok {
		return applyTiming(run, cached), nil
	}
	p, e := rest.RepoPath(repo)
	if e != nil {
		return original, e
	}
	if _, e = rest.Segment(run.ID); e != nil {
		return original, e
	}
	var earliest, latest *time.Time
	seen := map[string]bool{}
	fetched, total := 0, -1
	for page := 1; page <= 5; page++ {
		target, e := c.transport.Endpoint(p+"/actions/runs/"+run.ID+"/attempts/"+strconv.Itoa(run.Attempt)+"/jobs", rest.PageQuery("github", page, 100))
		if e != nil {
			return original, e
		}
		var body struct {
			Total *int `json:"total_count"`
			Jobs  []struct {
				ID          json.Number `json:"id"`
				Status      string      `json:"status"`
				Conclusion  string      `json:"conclusion"`
				StartedAt   string      `json:"started_at"`
				CompletedAt string      `json:"completed_at"`
			} `json:"jobs"`
		}
		meta, e := c.transport.Get(ctx, target, &body)
		if e != nil {
			return original, e
		}
		// The API's count is required on every page: a short response alone
		// cannot prove that all executed jobs were returned.
		if body.Total == nil || *body.Total < 0 || body.Jobs == nil {
			return run, nil
		}
		if total >= 0 && total != *body.Total {
			return run, nil
		}
		total = *body.Total

		for _, job := range body.Jobs {
			id := job.ID.String()
			if id == "" || seen[id] {
				return run, nil
			}
			seen[id] = true
			fetched++
			start, end := rest.Timestamp(job.StartedAt), rest.Timestamp(job.CompletedAt)
			if job.StartedAt == "" && job.CompletedAt == "" && (job.Conclusion == "skipped" || job.Conclusion == "cancelled") {
				continue
			}
			if start == nil || end == nil || end.Before(*start) || job.Status != "completed" {
				return run, nil
			}
			if earliest == nil || start.Before(*earliest) {
				earliest = start
			}
			if latest == nil || end.After(*latest) {
				latest = end
			}
		}
		next, e := c.transport.HasNext(meta, page, 100, len(body.Jobs), total)
		if e != nil {
			return original, e
		}
		if !next {
			if total >= 0 && total != fetched {
				return run, nil
			}
			if earliest == nil || latest == nil {
				return run, nil
			}
			duration := latest.Sub(*earliest).Milliseconds()
			v := timing{duration: &duration, start: earliest, end: latest}
			c.mu.Lock()
			if len(c.timings) >= 2048 {
				clear(c.timings)
			}
			c.timings[key] = v
			c.mu.Unlock()
			return applyTiming(run, v), nil
		}
	}
	return run, nil
}
func applyTiming(r ci.Run, v timing) ci.Run {
	if v.duration != nil {
		duration := *v.duration
		r.DurationMS = &duration
		r.DurationKind = "job_span"
	}
	if v.start != nil {
		start := *v.start
		r.StartedAt = &start
	}
	if v.end != nil {
		end := *v.end
		r.FinishedAt = &end
	}
	return r
}
