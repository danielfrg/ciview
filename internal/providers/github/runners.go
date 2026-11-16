package github

import (
	"ciview/internal/ci"
	"ciview/internal/providers/rest"
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
)

const maxRunnerPages = 10

var _ ci.RunnerProvider = (*Client)(nil)

type runnerResponse struct {
	ID      json.Number `json:"id"`
	Name    string      `json:"name"`
	OS      string      `json:"os"`
	Status  string      `json:"status"`
	Busy    bool        `json:"busy"`
	Version string      `json:"version"`
	Labels  []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// ListRunners reads every self-hosted runner registered to an organization or
// repository. A list that cannot be read completely fails rather than being
// reported as complete.
func (c *Client) ListRunners(ctx context.Context, target ci.RunnerTarget) ([]ci.Runner, error) {
	var base string
	switch {
	case target.Org != "" && target.Repo == "":
		org, err := rest.Segment(target.Org)
		if err != nil {
			return nil, err
		}
		base = "/orgs/" + org + "/actions/runners"
	case target.Repo != "" && target.Org == "":
		owner, name, ok := strings.Cut(target.Repo, "/")
		if !ok {
			return nil, rest.Error("invalid_data", "Invalid runner scope")
		}
		repo, err := rest.RepoPath(ci.Repository{Owner: owner, Name: name})
		if err != nil {
			return nil, err
		}
		base = repo + "/actions/runners"
	default:
		return nil, rest.Error("invalid_data", "Invalid runner scope")
	}
	next, err := c.transport.Endpoint(base, url.Values{"per_page": {"100"}})
	if err != nil {
		return nil, err
	}
	runners := []ci.Runner{}
	for pages := 0; next != ""; pages++ {
		if pages >= maxRunnerPages {
			return nil, rest.Error("invalid_data", "Provider runner list was incomplete")
		}
		var body struct {
			Runners []runnerResponse `json:"runners"`
		}
		page, err := c.transport.Get(ctx, next, &body)
		if err != nil {
			return nil, err
		}
		for _, raw := range body.Runners {
			runner, err := normalizeRunner(raw)
			if err != nil {
				return nil, err
			}
			runners = append(runners, runner)
		}
		if next, err = c.transport.NextLink(page); err != nil {
			return nil, err
		}
	}
	sort.Slice(runners, func(i, j int) bool {
		if runners[i].Name != runners[j].Name {
			return runners[i].Name < runners[j].Name
		}
		return runners[i].ID < runners[j].ID
	})
	return runners, nil
}

func normalizeRunner(v runnerResponse) (ci.Runner, error) {
	id := v.ID.String()
	if id == "" || id == "0" || v.Name == "" || (v.Status != "online" && v.Status != "offline") {
		return ci.Runner{}, rest.Error("invalid_data", "Provider runner identity was invalid")
	}
	labels := make([]string, 0, len(v.Labels))
	for _, label := range v.Labels {
		if label.Name != "" {
			labels = append(labels, label.Name)
		}
	}
	return ci.Runner{ID: id, Name: v.Name, OS: v.OS, Status: v.Status, Busy: v.Busy, Labels: labels, Version: v.Version}, nil
}
