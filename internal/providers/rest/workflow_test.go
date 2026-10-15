package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"ciview/internal/ci"
)

func TestWorkflowFilesUseBranchAndProviderDirectories(t *testing.T) {
	for _, tc := range []struct{ provider, directory string }{{"github", ".github/workflows"}, {"forgejo", ".forgejo/workflows"}, {"forgejo", ".gitea/workflows"}, {"forgejo", ".github/workflows"}} {
		t.Run(tc.provider+"/"+tc.directory, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("ref") != "release/next" {
					t.Errorf("wrong branch: %s", r.URL)
				}
				if r.URL.Path != "/repos/example/project/contents/"+tc.directory {
					http.NotFound(w, r)
					return
				}
				fmt.Fprintf(w, `[{"name":"ci.yml","path":%q,"type":"file"},{"name":"notes.txt","path":%q,"type":"file"},{"name":"nested.yml","path":%q,"type":"dir"}]`, tc.directory+"/ci.yml", tc.directory+"/notes.txt", tc.directory+"/nested.yml")
			}))
			defer s.Close()
			transport, err := New("sample", tc.provider, s.URL, "", "", s.Client())
			if err != nil {
				t.Fatal(err)
			}
			files, err := transport.ListWorkflowFiles(context.Background(), ci.Repository{Owner: "example", Name: "project"}, "release/next")
			want := "ci.yml"
			if tc.provider == "github" {
				want = tc.directory + "/ci.yml"
			}
			if err != nil || !reflect.DeepEqual(files, []string{want}) {
				t.Fatalf("files=%v err=%v", files, err)
			}
		})
	}
}

func TestWorkflowDirectoryMissingMustVerifyBranchAccess(t *testing.T) {
	for _, rootStatus := range []int{200, 403, 404, 500} {
		t.Run(fmt.Sprint(rootStatus), func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/repos/example/project/contents" {
					w.WriteHeader(rootStatus)
					fmt.Fprint(w, `[]`)
					return
				}
				http.NotFound(w, r)
			}))
			defer s.Close()
			transport, _ := New("sample", "github", s.URL, "", "", s.Client())
			files, err := transport.ListWorkflowFiles(context.Background(), ci.Repository{Owner: "example", Name: "project"}, "main")
			if rootStatus == 200 {
				if err != nil || len(files) != 0 {
					t.Fatalf("files=%v err=%v", files, err)
				}
			} else if err == nil {
				t.Fatal("unreadable branch treated as deleted workflows")
			}
		})
	}
}

func TestWorkflowDirectoryRejectsIncompleteOrInvalidCatalog(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `[{"name":"ci.yml","path":".github/workflows/other.yml","type":"file"}]`} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		transport, _ := New("sample", "github", s.URL, "", "", s.Client())
		_, err := transport.ListWorkflowFiles(context.Background(), ci.Repository{Owner: "example", Name: "project"}, "main")
		s.Close()
		if err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
