package rest

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"ciview/internal/ci"
)

// ListWorkflowFiles reads the live workflow directory at an exact branch, not
// the Actions registry, which retains deleted workflows and historical runs.
// GitHub identities are repository paths; Forgejo run identities are filenames.
func (t *Transport) ListWorkflowFiles(ctx context.Context, repository ci.Repository, branch string) ([]string, error) {
	if strings.TrimSpace(branch) == "" {
		return nil, Error("invalid_data", "Repository branch was unavailable")
	}
	p, err := RepoPath(repository)
	if err != nil {
		return nil, err
	}
	directories := []string{".github/workflows"}
	if t.Provider == "forgejo" {
		directories = []string{".forgejo/workflows", ".gitea/workflows", ".github/workflows"}
	}
	type entry struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}
	query := url.Values{"ref": {branch}}
	for _, directory := range directories {
		target, err := t.Endpoint(p+"/contents/"+directory, query)
		if err != nil {
			return nil, err
		}
		var entries []entry
		_, err = t.Get(ctx, target, &entries)
		if err != nil {
			var pe *ci.ProviderError
			if errors.As(err, &pe) && pe.Kind == "not_found" {
				continue
			}
			return nil, err
		}
		// GitHub directory listings are capped at 1,000 entries. Never publish an
		// incomplete catalog as proof that older workflows were removed.
		if entries == nil || (t.Provider == "github" && len(entries) >= 1000) {
			return nil, Error("invalid_data", "Provider workflow directory was incomplete")
		}
		files := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.Type != "file" || (!strings.HasSuffix(entry.Name, ".yml") && !strings.HasSuffix(entry.Name, ".yaml")) {
				continue
			}
			if _, err := Segment(entry.Name); err != nil || entry.Path != directory+"/"+entry.Name {
				return nil, Error("invalid_data", "Provider workflow file was invalid")
			}
			identity := entry.Path
			if t.Provider == "forgejo" {
				identity = entry.Name
			}
			files = append(files, identity)
		}
		return files, nil
	}
	// A directory 404 can also mean a missing branch or revoked repository
	// access. Confirm the branch root is readable before publishing no workflows.
	target, err := t.Endpoint(p+"/contents", query)
	if err != nil {
		return nil, err
	}
	var root []entry
	if _, err = t.Get(ctx, target, &root); err != nil {
		return nil, err
	}
	if root == nil {
		return nil, Error("invalid_data", "Provider branch contents were invalid")
	}
	return []string{}, nil
}
