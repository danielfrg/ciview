// Package forgejo normalizes the Forgejo v15+ Actions REST API.
package forgejo

import (
	"ciview/internal/ci"
	"ciview/internal/providers/rest"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

type Client struct{ transport *rest.Transport }

var _ ci.Provider = (*Client)(nil)

func New(sourceName, baseURL, token string, client *http.Client) (*Client, error) {
	// Validate the root before appending the API path so a query or fragment cannot
	// hide the configured prefix or change request routing.
	root, e := rest.New(sourceName, "forgejo", baseURL, token, "token", client)
	if e != nil {
		return nil, e
	}
	t, e := rest.New(sourceName, "forgejo", strings.TrimSuffix(root.Base.String(), "/")+"/api/v1", token, "token", client)
	if e != nil {
		return nil, e
	}
	return &Client{transport: t}, nil
}
func (c *Client) Discover(ctx context.Context, s ci.Selection) (ci.Discovery, error) {
	return c.transport.Discover(ctx, s, 50)
}

type runResponse struct {
	ID         json.Number `json:"id"`
	WorkflowID string      `json:"workflow_id"`
	Title      string      `json:"title"`
	PrettyRef  string      `json:"prettyref"`
	Event      string      `json:"event"`
	HTMLURL    string      `json:"html_url"`
	Status     string      `json:"status"`
	Duration   *int64      `json:"duration"`
	Created    string      `json:"created"`
	Started    string      `json:"started"`
	Stopped    string      `json:"stopped"`
}

func normalize(repo ci.Repository, v runResponse) (ci.Run, error) {
	if n, e := strconv.ParseUint(v.ID.String(), 10, 64); e != nil || n == 0 {
		return ci.Run{}, rest.Error("invalid_data", "Provider run identity was invalid")
	}
	if v.WorkflowID == "" {
		return ci.Run{}, rest.Error("invalid_data", "Provider workflow identity was unavailable")
	}
	r := ci.Run{ID: v.ID.String(), WorkflowID: v.WorkflowID, WorkflowName: v.WorkflowID, Title: v.Title, Branch: strings.TrimPrefix(v.PrettyRef, "refs/heads/"), Event: v.Event, URL: rest.SafeURL(v.HTMLURL), WorkflowURL: rest.WorkflowURL(repo.URL, "forgejo", v.WorkflowID), Attempt: 1, RawStatus: v.Status, StartedAt: rest.Timestamp(v.Started)}
	if created := rest.Timestamp(v.Created); created != nil {
		r.CreatedAt = *created
	}
	r.State, r.Terminal = state(v.Status)
	if r.Terminal {
		r.FinishedAt = rest.Timestamp(v.Stopped)
		// Forgejo serializes time.Duration as an integer count of nanoseconds.
		// Zero is the provider's unavailable/unexecuted timing value.
		if v.Duration != nil && *v.Duration > 0 {
			ms := *v.Duration / 1000000
			r.DurationMS = &ms
			r.DurationKind = "execution"
		}
	}
	return r, nil
}
func state(status string) (string, bool) {
	switch status {
	case "waiting":
		return "queued", false
	case "running":
		return "running", false
	case "blocked":
		return "waiting", false
	case "success":
		return "success", true
	case "failure":
		return "failure", true
	case "cancelled":
		return "canceled", true
	case "skipped":
		return "skipped", true
	case "unknown":
		return "unknown", false
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
	target, e := c.transport.Endpoint(p+"/actions/runs", rest.PageQuery("forgejo", page, 50))
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
	out.Next, e = c.transport.HasNext(meta, page, 50, len(body.Runs), total)
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
func (c *Client) EnrichDuration(_ context.Context, _ ci.Repository, r ci.Run) (ci.Run, error) {
	return r, nil
}
