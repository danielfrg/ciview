package rest

import (
	"civiewer/internal/ci"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestUserDiscoveryIncludesOwnedRepositoriesWithoutOrganizationLookup(t *testing.T) {
	for _, provider := range []string{"github", "forgejo"} {
		t.Run(provider, func(t *testing.T) {
			prefix, sizeKey, size := "/api/v3", "per_page", 100
			if provider == "forgejo" {
				prefix, sizeKey, size = "/forge/api/v1", "limit", 50
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("anonymous request carried credentials")
				}
				switch r.URL.Path {
				case prefix + "/users/person/repos":
					if r.URL.Query().Get(sizeKey) != fmt.Sprint(size) {
						t.Errorf("wrong pagination: %s", r.URL)
					}
					w.Header().Set("X-Total-Count", "2")
					if r.URL.Query().Get("page") == "1" {
						fmt.Fprint(w, `[{"id":41,"name":"first","owner":{"login":"person"},"default_branch":"main"}]`)
					} else {
						fmt.Fprint(w, `[{"id":42,"name":"second","owner":{"username":"person"},"default_branch":"trunk"}]`)
					}
				case prefix + "/repos/person/first":
					fmt.Fprint(w, `{"id":41,"name":"first","owner":{"login":"person"},"default_branch":"main"}`)
				default:
					t.Errorf("unexpected account lookup: %s", r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			tr, err := New("source", provider, server.URL+prefix, "", "Bearer", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			d, err := tr.Discover(context.Background(), ci.Selection{Users: []string{"person"}, Repos: []string{"person/first"}}, size)
			if err != nil || !d.Complete || len(d.Issues) != 0 || len(d.Repositories) != 2 {
				t.Fatalf("discovery=%+v err=%v", d, err)
			}
			names := []string{d.Repositories[0].FullName, d.Repositories[1].FullName}
			sort.Strings(names)
			if !reflect.DeepEqual(names, []string{"person/first", "person/second"}) {
				t.Fatalf("repositories=%v", names)
			}
		})
	}
}

func TestGitHubUserDiscoveryMergesPrivateVisibleRepositoriesForSelectedOwners(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-secret" {
			t.Error("private discovery missing authentication")
		}
		switch r.URL.Path {
		case "/users/person/repos":
			fmt.Fprint(w, `[{"id":41,"name":"public","owner":{"login":"person"}}]`)
		case "/users/friend/repos":
			fmt.Fprint(w, `[]`)
		case "/user/repos":
			if r.URL.Query().Get("visibility") != "all" || r.URL.Query().Get("affiliation") != "owner,collaborator" || r.URL.Query().Get("type") != "" {
				t.Errorf("private discovery query=%s", r.URL.RawQuery)
			}
			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("Link", `<?affiliation=owner%2Ccollaborator&visibility=all&page=2&per_page=100>; rel="next"`)
				fmt.Fprint(w, `[{"id":41,"name":"public","owner":{"login":"person"}},{"id":42,"name":"private","owner":{"login":"PERSON"},"private":true},{"id":90,"name":"unselected","owner":{"login":"another"}}]`)
			} else {
				fmt.Fprint(w, `[{"id":43,"name":"shared-private","owner":{"login":"friend"},"private":true},{"id":91,"name":"organization","owner":{"login":"team"}}]`)
			}
		case "/orgs/team/repos":
			fmt.Fprint(w, `[{"id":91,"name":"organization","owner":{"login":"team"}}]`)
		case "/repos/person/private":
			fmt.Fprint(w, `{"id":42,"name":"private","owner":{"login":"PERSON"},"private":true}`)
		default:
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	tr, _ := New("source", "github", server.URL, "private-secret", "Bearer", server.Client())
	d, err := tr.Discover(context.Background(), ci.Selection{Users: []string{"person", "friend", "PERSON"}, Orgs: []string{"team"}, Repos: []string{"person/private"}}, 100)
	if err != nil || !d.Complete || len(d.Issues) != 0 {
		t.Fatalf("discovery=%+v err=%v", d, err)
	}
	var ids []string
	for _, repo := range d.Repositories {
		ids = append(ids, repo.ID)
	}
	sort.Strings(ids)
	if !reflect.DeepEqual(ids, []string{"41", "42", "43", "91"}) {
		t.Fatalf("selected private inventory=%v", ids)
	}
}

func TestForgejoUserDiscoveryReadsPrivateRepositoriesDirectly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token private-secret" {
			t.Error("private discovery missing authentication")
		}
		if r.URL.Path != "/forge/api/v1/users/person/repos" {
			t.Errorf("unexpected Forgejo endpoint: %s", r.URL)
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `[{"id":42,"name":"private","owner":{"login":"person"},"private":true,"html_url":"https://forge.example/forge/person/private","default_branch":"main"}]`)
	}))
	defer server.Close()
	tr, _ := New("source", "forgejo", server.URL+"/forge/api/v1", "private-secret", "token", server.Client())
	d, err := tr.Discover(context.Background(), ci.Selection{Users: []string{"person"}}, 50)
	if err != nil || !d.Complete || len(d.Repositories) != 1 || d.Repositories[0].ID != "42" || d.Repositories[0].URL != "https://forge.example/forge/person/private" {
		t.Fatalf("discovery=%+v err=%v", d, err)
	}
}

func TestGitHubPrivateUserDiscoveryFailureRetainsPublicInventoryAndContinuesExplicitRepos(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/person/repos":
			fmt.Fprint(w, `[{"id":41,"name":"public","owner":{"login":"person"}}]`)
		case "/users/friend/repos":
			fmt.Fprint(w, `[]`)
		case "/user/repos":
			http.Error(w, "private-secret", http.StatusForbidden)
		case "/repos/person/explicit":
			fmt.Fprint(w, `{"id":42,"name":"explicit","owner":{"login":"person"}}`)
		default:
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	tr, _ := New("source", "github", server.URL, "private-secret", "Bearer", server.Client())
	d, err := tr.Discover(context.Background(), ci.Selection{Users: []string{"person", "friend"}, Repos: []string{"person/explicit"}}, 100)
	if err != nil || d.Complete || len(d.Repositories) != 2 || len(d.Issues) != 2 {
		t.Fatalf("discovery=%+v err=%v", d, err)
	}
	for _, issue := range d.Issues {
		if issue.Kind != "permission" || strings.Contains(issue.Message, "private-secret") || (issue.Repository != "person" && issue.Repository != "friend") {
			t.Fatalf("unsafe issue=%+v", issue)
		}
	}
}

func TestUserDiscoveryCooldownKeepsEarlierPagesAndStopsOtherSelections(t *testing.T) {
	for _, provider := range []string{"github", "forgejo"} {
		t.Run(provider, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/users/person/repos" && r.URL.Query().Get("page") == "1" {
					w.Header().Set("Link", `<?page=2>; rel="next"`)
					fmt.Fprint(w, `[{"id":41,"name":"public","owner":{"login":"person"}}]`)
					return
				}
				if r.URL.Path == "/users/person/repos" && r.URL.Query().Get("page") == "2" {
					w.Header().Set("Retry-After", "3600")
					http.Error(w, "private-secret", http.StatusServiceUnavailable)
					return
				}
				t.Errorf("discovery continued after cooldown: %s", r.URL)
				http.NotFound(w, r)
			}))
			defer server.Close()
			tr, _ := New("source", provider, server.URL, "private-secret", "Bearer", server.Client())
			before := time.Now()
			d, err := tr.Discover(context.Background(), ci.Selection{Users: []string{"person", "friend"}, Repos: []string{"person/explicit"}}, 50)
			var pe *ci.ProviderError
			if !errors.As(err, &pe) || pe.RetryAt == nil || pe.RetryAt.Before(before.Add(3599*time.Second)) || d.Complete || len(d.Repositories) != 1 || len(d.Issues) != 1 || d.Issues[0].Repository != "person" {
				t.Fatalf("cooldown discovery=%+v err=%v", d, err)
			}
		})
	}
}

func TestMissingUserRemainsAnIssueWhileOtherSelectionsAreDiscovered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/missing/repos":
			http.NotFound(w, r)
		case "/users/person/repos":
			fmt.Fprint(w, `[{"id":42,"name":"project","owner":{"login":"person"}}]`)
		default:
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	tr, _ := New("source", "forgejo", server.URL, "", "token", server.Client())
	d, err := tr.Discover(context.Background(), ci.Selection{Users: []string{"missing", "person"}}, 50)
	if err != nil || d.Complete || len(d.Repositories) != 1 || len(d.Issues) != 1 || d.Issues[0].Repository != "missing" || d.Issues[0].Kind != "not_found" {
		t.Fatalf("discovery=%+v err=%v", d, err)
	}
}

func TestForgejoUserDiscoveryFollowsNextLinkAfterPermissionFilteredEmptyPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/person/repos" {
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Total-Count", "101")
		switch r.URL.Query().Get("page") {
		case "1":
			w.Header().Set("Link", `<?page=2&limit=50>; rel="next"`)
			fmt.Fprint(w, `[]`)
		case "2":
			w.Header().Set("Link", `<?page=3&limit=50>; rel="next"`)
			fmt.Fprint(w, `[]`)
		case "3":
			w.Header().Set("Link", `<?page=2&limit=50>; rel="prev"`)
			fmt.Fprint(w, `[{"id":42,"name":"shared","owner":{"login":"person"}}]`)
		default:
			t.Errorf("unexpected page: %s", r.URL)
			fmt.Fprint(w, `[]`)
		}
	}))
	defer server.Close()
	tr, _ := New("source", "forgejo", server.URL, "private-secret", "token", server.Client())
	d, err := tr.Discover(context.Background(), ci.Selection{Users: []string{"person"}}, 50)
	if err != nil || !d.Complete || len(d.Issues) != 0 || len(d.Repositories) != 1 || d.Repositories[0].ID != "42" {
		t.Fatalf("permission-filtered discovery=%+v err=%v", d, err)
	}
}

func TestInvalidPrivateUserRepositoryReportsItsSelectedOwner(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/person/repos", "/users/friend/repos":
			fmt.Fprint(w, `[]`)
		case "/user/repos":
			fmt.Fprint(w, `[{"id":0,"name":"invalid","owner":{"login":"FRIEND"}}]`)
		default:
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	tr, _ := New("source", "github", server.URL, "private-secret", "Bearer", server.Client())
	d, err := tr.Discover(context.Background(), ci.Selection{Users: []string{"person", "friend"}}, 100)
	if err != nil || d.Complete || len(d.Issues) != 1 || d.Issues[0].Repository != "friend" || d.Issues[0].Kind != "invalid_data" {
		t.Fatalf("wrong owner issue=%+v err=%v", d, err)
	}
}

func TestGitHubPrivateUserDiscoveryKeepsEarlierPagesOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/person/repos":
			fmt.Fprint(w, `[{"id":41,"name":"public","owner":{"login":"person"}}]`)
		case "/users/friend/repos":
			fmt.Fprint(w, `[]`)
		case "/user/repos":
			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("Link", `<?page=2&per_page=100&visibility=all&affiliation=owner%2Ccollaborator>; rel="next"`)
				fmt.Fprint(w, `[{"id":42,"name":"private","owner":{"login":"person"},"private":true}]`)
			} else {
				http.Error(w, "private-secret", http.StatusInternalServerError)
			}
		default:
			t.Errorf("unexpected request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	tr, _ := New("source", "github", server.URL, "private-secret", "Bearer", server.Client())
	d, err := tr.Discover(context.Background(), ci.Selection{Users: []string{"person", "friend"}}, 100)
	if err != nil || d.Complete || len(d.Repositories) != 2 || len(d.Issues) != 2 {
		t.Fatalf("partial private inventory lost=%+v err=%v", d, err)
	}
	var ids []string
	for _, repo := range d.Repositories {
		ids = append(ids, repo.ID)
	}
	sort.Strings(ids)
	if !reflect.DeepEqual(ids, []string{"41", "42"}) || d.Issues[0].Repository != "person" || d.Issues[1].Repository != "friend" {
		t.Fatalf("partial private inventory or owner notices=%+v", d)
	}
}

func TestDiscoveryExcludesArchivedRepositoriesAndForksFromEverySelection(t *testing.T) {
	for _, provider := range []string{"github", "forgejo"} {
		t.Run(provider, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/orgs/team/repos":
					fmt.Fprint(w, `[{"id":1,"name":"active","owner":{"login":"team"},"fork":false,"archived":false},{"id":2,"name":"fork","owner":{"login":"team"},"fork":true,"archived":false},{"id":3,"name":"archived","owner":{"login":"team"},"fork":false,"archived":true},{"id":4,"name":"both","owner":{"login":"team"},"fork":true,"archived":true}]`)
				case "/users/person/repos":
					fmt.Fprint(w, `[{"id":5,"name":"active","owner":{"login":"person"},"fork":false,"archived":false},{"id":6,"name":"fork","owner":{"login":"person"},"fork":true,"archived":false},{"id":7,"name":"archived","owner":{"login":"person"},"fork":false,"archived":true},{"id":8,"name":"both","owner":{"login":"person"},"fork":true,"archived":true}]`)
				case "/user/repos":
					if provider != "github" {
						t.Error("Forgejo should not need another private list")
					}
					fmt.Fprint(w, `[{"id":9,"name":"private","owner":{"login":"PERSON"},"private":true,"fork":false,"archived":false},{"id":10,"name":"private-fork","owner":{"login":"person"},"private":true,"fork":true,"archived":false},{"id":11,"name":"private-archived","owner":{"login":"person"},"private":true,"fork":false,"archived":true},{"id":12,"name":"private-both","owner":{"login":"person"},"private":true,"fork":true,"archived":true}]`)
				case "/repos/person/pinned":
					fmt.Fprint(w, `{"id":14,"name":"pinned","owner":{"login":"person"},"fork":false,"archived":false}`)
				case "/repos/person/pinned-fork":
					fmt.Fprint(w, `{"id":15,"name":"pinned-fork","owner":{"login":"person"},"fork":true,"archived":false}`)
				case "/repos/person/pinned-archived":
					fmt.Fprint(w, `{"id":16,"name":"pinned-archived","owner":{"login":"person"},"fork":false,"archived":true}`)
				case "/repos/person/pinned-both":
					fmt.Fprint(w, `{"id":17,"name":"pinned-both","owner":{"login":"person"},"fork":true,"archived":true}`)
				default:
					t.Errorf("unexpected request: %s", r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			tr, err := New("source", provider, server.URL, "private-secret", "Bearer", server.Client())
			if err != nil {
				t.Fatal(err)
			}
			selection := ci.Selection{Orgs: []string{"team"}, Users: []string{"person"}, Repos: []string{"person/pinned", "person/pinned-fork", "person/pinned-archived", "person/pinned-both"}}
			d, err := tr.Discover(context.Background(), selection, 100)
			if err != nil || !d.Complete || len(d.Issues) != 0 {
				t.Fatalf("filtered discovery=%+v err=%v", d, err)
			}
			var names []string
			for _, repo := range d.Repositories {
				names = append(names, repo.FullName)
			}
			sort.Strings(names)
			want := []string{"person/active", "person/pinned", "team/active"}
			if provider == "github" {
				want = []string{"PERSON/private", "person/active", "person/pinned", "team/active"}
			}
			if !reflect.DeepEqual(names, want) {
				t.Fatalf("active source repositories=%v want=%v", names, want)
			}
		})
	}
}

func TestDiscoveryContinuesPastPagesContainingOnlyExcludedRepositories(t *testing.T) {
	for _, provider := range []string{"github", "forgejo"} {
		t.Run(provider, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/users/person/repos" {
					t.Errorf("unexpected request: %s", r.URL)
					http.NotFound(w, r)
					return
				}
				w.Header().Set("X-Total-Count", "3")
				switch r.URL.Query().Get("page") {
				case "1":
					fmt.Fprint(w, `[{"id":1,"name":"fork","owner":{"login":"person"},"fork":true},{"id":2,"name":"archived","owner":{"login":"person"},"archived":true}]`)
				case "2":
					fmt.Fprint(w, `[{"id":3,"name":"active","owner":{"login":"person"},"fork":false,"archived":false}]`)
				default:
					t.Errorf("unexpected page: %s", r.URL)
					fmt.Fprint(w, `[]`)
				}
			}))
			defer server.Close()
			tr, _ := New("source", provider, server.URL, "", "Bearer", server.Client())
			d, err := tr.Discover(context.Background(), ci.Selection{Users: []string{"person"}}, 100)
			if err != nil || !d.Complete || len(d.Issues) != 0 || len(d.Repositories) != 1 || d.Repositories[0].FullName != "person/active" {
				t.Fatalf("later source repository lost=%+v err=%v", d, err)
			}
		})
	}
}
