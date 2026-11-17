package github

import (
	"context"
	"errors"
	"fmt"
	"github.com/danielfrg/ciview/internal/ci"
	"net/http"
	"strings"
	"testing"
)

func TestListRunnersOrgPaginatesAndNormalizes(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-secret" {
			t.Errorf("missing auth")
		}
		if r.URL.Path != "/orgs/acme/actions/runners" {
			t.Errorf("unexpected request %s", r.URL)
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("page") == "" {
			w.Header().Set("Link", `</orgs/acme/actions/runners?per_page=100&page=2>; rel="next"`)
			fmt.Fprint(w, `{"total_count":2,"runners":[{"id":7,"name":"b-runner","os":"Linux","status":"online","busy":true,"labels":[{"id":1,"name":"self-hosted","type":"read-only"},{"id":2,"name":"acme-ops","type":"custom"}],"version":"2.338.2"}]}`)
			return
		}
		fmt.Fprint(w, `{"total_count":2,"runners":[{"id":3,"name":"a-runner","os":"Linux","status":"offline","busy":false,"labels":[]}]}`)
	})
	runners, err := c.ListRunners(context.Background(), ci.RunnerTarget{Org: "acme"})
	if err != nil || len(runners) != 2 {
		t.Fatalf("runners=%+v err=%v", runners, err)
	}
	if runners[0].Name != "a-runner" || runners[1].Name != "b-runner" {
		t.Fatalf("runners must be sorted by name: %+v", runners)
	}
	b := runners[1]
	if b.ID != "7" || b.Status != "online" || !b.Busy || b.Version != "2.338.2" || strings.Join(b.Labels, ",") != "self-hosted,acme-ops" {
		t.Fatalf("normalized=%+v", b)
	}
	if runners[0].Version != "" || runners[0].Status != "offline" {
		t.Fatalf("missing version must stay empty: %+v", runners[0])
	}
}

func TestListRunnersRepoPath(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/project/actions/runners" {
			t.Errorf("unexpected request %s", r.URL)
		}
		fmt.Fprint(w, `{"total_count":0,"runners":[]}`)
	})
	runners, err := c.ListRunners(context.Background(), ci.RunnerTarget{Repo: "acme/project"})
	if err != nil || len(runners) != 0 {
		t.Fatalf("runners=%+v err=%v", runners, err)
	}
}

func TestListRunnersMapsFailuresWithoutProviderText(t *testing.T) {
	for status, kind := range map[int]string{401: "authentication", 403: "permission", 404: "not_found", 500: "unavailable"} {
		c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) { http.Error(w, "private-secret details", status) })
		_, err := c.ListRunners(context.Background(), ci.RunnerTarget{Org: "acme"})
		var pe *ci.ProviderError
		if !errors.As(err, &pe) || pe.Kind != kind || strings.Contains(err.Error(), "private-secret") {
			t.Errorf("status %d => %v", status, err)
		}
	}
}

func TestListRunnersRejectsInvalidPayloadAndUnsafeTargets(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"runners":[{"id":0,"name":"","status":"online"}]}`)
	})
	if _, err := c.ListRunners(context.Background(), ci.RunnerTarget{Org: "acme"}); err == nil {
		t.Fatal("invalid runner identity must fail the read")
	}
	if _, err := c.ListRunners(context.Background(), ci.RunnerTarget{}); err == nil {
		t.Fatal("empty target must fail")
	}
	if _, err := c.ListRunners(context.Background(), ci.RunnerTarget{Org: "a/b"}); err == nil {
		t.Fatal("unsafe org must fail")
	}
}

func TestListRunnersFailsWhenPaginationNeverEnds(t *testing.T) {
	c, _ := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Link", `</orgs/acme/actions/runners?page=2>; rel="next"`)
		fmt.Fprint(w, `{"runners":[]}`)
	})
	if _, err := c.ListRunners(context.Background(), ci.RunnerTarget{Org: "acme"}); err == nil {
		t.Fatal("endless pagination must fail rather than report a partial list as complete")
	}
}
