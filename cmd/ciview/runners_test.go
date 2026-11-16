package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"ciview/internal/ci"
	"ciview/internal/config"
	"ciview/internal/providers/github"
)

func TestRunnerCollectorUsesSourceTokenUnlessOverridden(t *testing.T) {
	var mu sync.Mutex
	auth := map[string]string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auth[r.URL.Path] = r.Header.Get("Authorization")
		mu.Unlock()
		fmt.Fprint(w, `{"runners":[{"id":1,"name":"r","status":"online","busy":false,"labels":[]}]}`)
	}))
	defer upstream.Close()
	t.Setenv("CIVIEW_TEST_RUNNER_TOKEN", "runner-token")
	source := config.Source{Name: "gh", Provider: "github", BaseURL: upstream.URL, Repos: []string{"acme/app"}}
	client, err := github.New("gh", upstream.URL, "source-token", upstream.Client())
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Viewer:  config.Viewer{RunnerInterval: "1m"},
		Sources: []config.Source{source},
		Runners: []config.RunnerScope{
			{Source: "gh", Scope: "org:acme"},
			{Source: "gh", Scope: "repo:acme/app", TokenEnv: "CIVIEW_TEST_RUNNER_TOKEN"},
		},
	}
	collector, err := runnerCollector(cfg, map[string]ci.Provider{"gh": client}, upstream.Client())
	if err != nil || collector == nil {
		t.Fatalf("collector=%v err=%v", collector, err)
	}
	collector.Refresh(context.Background())
	if auth["/orgs/acme/actions/runners"] != "Bearer source-token" || auth["/repos/acme/app/actions/runners"] != "Bearer runner-token" {
		t.Fatalf("auth=%v", auth)
	}
	got, _ := collector.Runners(context.Background())
	if len(got) != 2 || got[0].State != "healthy" || got[1].State != "healthy" {
		t.Fatalf("scopes=%+v", got)
	}
}

func TestRunnerCollectorIsNilWithoutScopesAndFailsSafelyOnMissingToken(t *testing.T) {
	collector, err := runnerCollector(config.Config{}, nil, nil)
	if err != nil || collector != nil {
		t.Fatalf("collector=%v err=%v", collector, err)
	}
	t.Setenv("CIVIEW_TEST_MISSING_RUNNER_TOKEN", "")
	cfg := config.Config{
		Sources: []config.Source{{Name: "gh", Provider: "github"}},
		Runners: []config.RunnerScope{{Source: "gh", Scope: "org:acme", TokenEnv: "CIVIEW_TEST_MISSING_RUNNER_TOKEN"}},
	}
	if _, err := runnerCollector(cfg, nil, nil); err == nil {
		t.Fatal("a missing runner token must fail startup like a missing source token")
	}
}
