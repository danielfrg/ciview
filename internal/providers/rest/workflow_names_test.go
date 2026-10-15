package rest

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ciview/internal/ci"
)

func TestWorkflowCatalogNamesAreOptionalAndBranchScoped(t *testing.T) {
	for _, directory := range []string{".forgejo/workflows", ".gitea/workflows", ".github/workflows"} {
		t.Run(directory, func(t *testing.T) {
			definitions := map[string]string{"docker.yml": "name: Docker\non: push\n", "escaped.yml": "name: \"Back Up \\u0049mages\"\n", "duplicate.yml": "name: Docker\n", "unnamed.yml": "on: push\n", "invalid.yml": "name: [bad\n", "object.yml": "name: {bad: value}\n", "multidoc.yml": "name: Wrong\n---\nname: Other\n", "large.yml": strings.Repeat("#", 65537)}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("ref") != "release/next" {
					t.Errorf("wrong branch %s", r.URL)
				}
				prefix := "/repos/example/project/contents/" + directory
				if r.URL.Path == prefix {
					var entries []map[string]string
					for file := range definitions {
						entries = append(entries, map[string]string{"name": file, "path": directory + "/" + file, "type": "file"})
					}
					entries = append(entries, map[string]string{"name": "denied.yml", "path": directory + "/denied.yml", "type": "file"})
					json.NewEncoder(w).Encode(entries)
					return
				}
				file := strings.TrimPrefix(r.URL.Path, prefix+"/")
				if s, ok := definitions[file]; ok {
					json.NewEncoder(w).Encode(map[string]any{"encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte(s)), "type": "file", "path": directory + "/" + file})
					return
				}
				http.NotFound(w, r)
			}))
			defer server.Close()
			transport, _ := New("forge", "forgejo", server.URL, "", "", server.Client())
			catalog, err := transport.ListWorkflowCatalog(context.Background(), ci.Repository{Owner: "example", Name: "project"}, "release/next")
			if err != nil || len(catalog.Files) != 9 {
				t.Fatalf("catalog=%+v err=%v", catalog, err)
			}
			if fmt.Sprint(catalog.Names) != "map[docker.yml:Docker duplicate.yml:Docker escaped.yml:Back Up Images]" {
				t.Fatalf("names=%v", catalog.Names)
			}
		})
	}
}

func TestWorkflowCatalogStopsOptionalMetadataOnThrottling(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/workflows") {
			fmt.Fprint(w, `[{"type":"file","name":"one.yml","path":".forgejo/workflows/one.yml"},{"type":"file","name":"two.yml","path":".forgejo/workflows/two.yml"}]`)
			return
		}
		calls++
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	transport, _ := New("forge", "forgejo", server.URL, "", "", server.Client())
	catalog, err := transport.ListWorkflowCatalog(context.Background(), ci.Repository{Owner: "example", Name: "project"}, "main")
	if err != nil || len(catalog.Files) != 2 || calls != 1 || catalog.MetadataError == nil {
		t.Fatalf("catalog=%+v calls=%d err=%v", catalog, calls, err)
	}
}
