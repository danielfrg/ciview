package rest

import (
	"civiewer/internal/ci"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type repository struct {
	ID       json.Number `json:"id"`
	Name     string      `json:"name"`
	FullName string      `json:"full_name"`
	Owner    struct {
		Login    string `json:"login"`
		Username string `json:"username"`
	} `json:"owner"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
}

func (t *Transport) repository(v repository) (ci.Repository, error) {
	id := v.ID.String()
	if n, e := strconv.ParseUint(id, 10, 64); e != nil || n == 0 {
		return ci.Repository{}, Error("invalid_data", "Provider repository identity was invalid")
	}
	owner := v.Owner.Login
	if owner == "" {
		owner = v.Owner.Username
	}
	if _, e := Segment(owner); e != nil {
		return ci.Repository{}, e
	}
	if _, e := Segment(v.Name); e != nil {
		return ci.Repository{}, e
	}
	fullName := v.FullName
	if fullName == "" {
		fullName = owner + "/" + v.Name
	}
	return ci.Repository{Key: StableKey(t.Source, t.Provider, t.Base.String(), id), Source: t.Source, Provider: t.Provider, ID: id, Owner: owner, Name: v.Name, FullName: fullName, DefaultBranch: v.DefaultBranch, URL: SafeURL(v.HTMLURL)}, nil
}

// Discover returns every verified repository already fetched even when a later
// page or selection fails. Complete=false tells inventory reconciliation to keep
// previously healthy repositories until a fully successful discovery. Actionable
// retry deadlines stop discovery and return the typed error alongside partial data.
func (t *Transport) Discover(ctx context.Context, s ci.Selection, size int) (ci.Discovery, error) {
	d := ci.Discovery{Repositories: []ci.Repository{}, Issues: []ci.Issue{}, Complete: true}
	seen := map[ci.RepositoryKey]bool{}
	issue := func(selection string, e error) {
		d.Complete = false
		kind, message := "unavailable", "Provider discovery failed"
		var pe *ci.ProviderError
		if errors.As(e, &pe) {
			kind, message = pe.Kind, pe.Message
		}
		d.Issues = append(d.Issues, ci.Issue{Repository: selection, Kind: kind, Message: message})
	}
	add := func(v repository, selection string) {
		r, e := t.repository(v)
		if e != nil {
			issue(selection, e)
			return
		}
		if !seen[r.Key] {
			seen[r.Key] = true
			d.Repositories = append(d.Repositories, r)
		}
	}
	for _, org := range s.Orgs {
		if ctx.Err() != nil {
			return d, ctx.Err()
		}
		if _, e := Segment(org); e != nil {
			issue(org, e)
			continue
		}
		target, _ := t.Endpoint("/orgs/"+org+"/repos", pageQuery(t.Provider, 1, size))
		visited := map[string]bool{}
		for page := 1; page <= 100; page++ {
			if visited[target] {
				issue(org, Error("invalid_data", "Provider pagination repeated a page"))
				break
			}
			visited[target] = true
			var items []repository
			p, e := t.Get(ctx, target, &items)
			if e != nil {
				issue(org, e)
				if discoveryCooldown(e) {
					return d, e
				}
				break
			}
			for _, item := range items {
				add(item, org)
			}
			next, e := t.HasNext(p, page, size, len(items), -1)
			if e != nil {
				issue(org, e)
				break
			}
			if !next {
				break
			}
			if page == 100 {
				issue(org, Error("incomplete", "Organization discovery reached its page limit"))
				break
			}
			target, e = t.NextLink(p)
			if e != nil {
				issue(org, e)
				break
			}
			if target == "" {
				target, _ = t.Endpoint("/orgs/"+org+"/repos", pageQuery(t.Provider, page+1, size))
			}
		}
	}
	for _, selection := range s.Repos {
		if ctx.Err() != nil {
			return d, ctx.Err()
		}
		parts := strings.Split(selection, "/")
		if len(parts) != 2 {
			issue(selection, Error("invalid_data", "Repository selection must use owner/name"))
			continue
		}
		endpoint, e := RepoPath(ci.Repository{Owner: parts[0], Name: parts[1]})
		if e != nil {
			issue(selection, e)
			continue
		}
		target, e := t.Endpoint(endpoint, nil)
		if e != nil {
			issue(selection, e)
			continue
		}
		var v repository
		if _, e = t.Get(ctx, target, &v); e != nil {
			issue(selection, e)
			if discoveryCooldown(e) {
				return d, e
			}
			continue
		}
		add(v, selection)
	}
	sort.Slice(d.Repositories, func(i, j int) bool { return d.Repositories[i].Key < d.Repositories[j].Key })
	return d, nil
}

func discoveryCooldown(err error) bool {
	var pe *ci.ProviderError
	if !errors.As(err, &pe) {
		return false
	}
	if pe.RetryAt != nil {
		return pe.RetryAt.After(time.Now())
	}
	return pe.Kind == "rate_limit" || pe.Kind == "rate_limited"
}

func pageQuery(provider string, page, size int) url.Values {
	key := "per_page"
	if provider == "forgejo" {
		key = "limit"
	}
	return url.Values{"page": {strconv.Itoa(page)}, key: {strconv.Itoa(size)}}
}
func PageQuery(provider string, page, size int) url.Values { return pageQuery(provider, page, size) }
