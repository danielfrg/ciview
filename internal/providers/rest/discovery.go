package rest

import (
	"ciview/internal/ci"
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
	Fork          bool   `json:"fork"`
	Archived      bool   `json:"archived"`
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
		if v.Fork || v.Archived {
			return
		}
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
	// A shared listing path preserves partial inventory, pagination checks and
	// source cooldowns for organizations, users and authenticated visibility.
	list := func(selections []string, endpoint string, query url.Values, selectionFor func(repository) string) error {
		report := func(err error) {
			for _, selection := range selections {
				issue(selection, err)
			}
		}
		pageTarget := func(page int) (string, error) {
			q := pageQuery(t.Provider, page, size)
			for key, values := range query {
				q[key] = append([]string(nil), values...)
			}
			return t.Endpoint(endpoint, q)
		}
		target, err := pageTarget(1)
		if err != nil {
			report(err)
			return nil
		}
		visited := map[string]bool{}
		for page := 1; page <= 100; page++ {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if visited[target] {
				report(Error("invalid_data", "Provider pagination repeated a page"))
				return nil
			}
			visited[target] = true
			var items []repository
			p, err := t.Get(ctx, target, &items)
			if err != nil {
				report(err)
				if discoveryCooldown(err) {
					return err
				}
				return nil
			}
			for _, item := range items {
				selection := selections[0]
				if selectionFor != nil {
					selection = selectionFor(item)
				}
				if selection != "" {
					add(item, selection)
				}
			}
			next, err := t.HasNext(p, page, size, len(items), -1)
			if err != nil {
				report(err)
				return nil
			}
			if !next {
				return nil
			}
			if page == 100 {
				report(Error("incomplete", "Repository discovery reached its page limit"))
				return nil
			}
			target, err = t.NextLink(p)
			if err != nil {
				report(err)
				return nil
			}
			if target == "" {
				target, err = pageTarget(page + 1)
			}
			if err != nil {
				report(err)
				return nil
			}
		}
		return nil
	}
	for _, org := range s.Orgs {
		if _, err := Segment(org); err != nil {
			issue(org, err)
			continue
		}
		if err := list([]string{org}, "/orgs/"+org+"/repos", nil, nil); err != nil {
			return d, err
		}
	}
	users := map[string]string{}
	var selectedUsers []string
	for _, user := range s.Users {
		if _, err := Segment(user); err != nil {
			issue(user, err)
			continue
		}
		key := strings.ToLower(user)
		if users[key] != "" {
			continue
		}
		users[key] = user
		selectedUsers = append(selectedUsers, user)
		// Forgejo includes private repositories visible to the authenticated
		// account here. GitHub exposes only public repositories on this route.
		if err := list([]string{user}, "/users/"+user+"/repos", nil, nil); err != nil {
			return d, err
		}
	}
	if t.Provider == "github" && t.token != "" && len(selectedUsers) > 0 {
		// Merge token-visible private repositories owned by any selected user.
		// Collaborator access covers private repositories owned by another user;
		// unrelated repositories never enter this source's inventory.
		query := url.Values{"visibility": {"all"}, "affiliation": {"owner,collaborator"}}
		selectionFor := func(item repository) string {
			return users[strings.ToLower(item.Owner.Login)]
		}
		if err := list(selectedUsers, "/user/repos", query, selectionFor); err != nil {
			return d, err
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
