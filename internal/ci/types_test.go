package ci_test

import (
	"encoding/json"
	"strings"
	"testing"

	"ciview/internal/ci"
)

func TestModelsEncodeSnakeCaseAndEmptyArrays(t *testing.T) {
	cases := []struct {
		name   string
		value  any
		fields []string
	}{
		{"overview", ci.Overview{Sources: []ci.SourceState{{}}, Workflows: []ci.Workflow{{}}}, []string{`"workflows":[`, `"sources":[`, `"repositories":[]`, `"issues":[]`, `"runs":[]`, `"latest_run":null`, `"success_rate":null`, `"last_completed_duration_ms":null`}},
		{"snapshot", ci.RepositorySnapshot{}, []string{`"runs":[]`, `"oldest_fetched_at":null`, `"last_success_at":null`}},
		{"selection", ci.Selection{}, []string{`"orgs":[]`, `"users":[]`, `"repos":[]`}},
		{"discovery", ci.Discovery{}, []string{`"repositories":[]`, `"issues":[]`}},
		{"page", ci.RunPage{}, []string{`"runs":[]`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bytes, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			for _, field := range tc.fields {
				if !strings.Contains(string(bytes), field) {
					t.Fatalf("missing %s in %s", field, bytes)
				}
			}
		})
	}
}

func TestProviderErrorDoesNotEchoUpstreamPayload(t *testing.T) {
	err := (&ci.ProviderError{Kind: "authentication", Message: "upstream token=private-secret"}).Error()
	if strings.Contains(err, "private-secret") || err == "" {
		t.Fatalf("provider error leaked payload: %q", err)
	}
}

func TestWorkflowURLIsAlwaysPresentAndUnknownRunURLIsOmitted(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   any
		present bool
	}{
		{"workflow", ci.Workflow{}, true},
		{"run", ci.Run{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			var wire map[string]any
			if err := json.Unmarshal(body, &wire); err != nil {
				t.Fatal(err)
			}
			value, present := wire["workflow_url"]
			if present != tc.present || present && value != "" {
				t.Fatalf("workflow URL presence=%v value=%#v wantPresent=%v", present, value, tc.present)
			}
		})
	}
}
