package ci

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

// Runner is one self-hosted runner as reported by the provider. Status is the
// provider's own value ("online" or "offline"); Version is empty when the
// provider did not report one.
type Runner struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	OS      string   `json:"os"`
	Status  string   `json:"status"`
	Busy    bool     `json:"busy"`
	Labels  []string `json:"labels"`
	Version string   `json:"version"`
}

// RunnerScope is the cached runner list for one organization or repository.
// State is loading, healthy, stale (last read failed, rows retained) or
// unavailable (no successful read yet). Kind and Message describe the last
// failure and never contain provider text.
type RunnerScope struct {
	Scope         string     `json:"scope"`
	Source        string     `json:"source"`
	State         string     `json:"state"`
	Kind          string     `json:"kind"`
	Message       string     `json:"message"`
	AsOf          *time.Time `json:"as_of"`
	LastAttemptAt *time.Time `json:"last_attempt_at"`
	Runners       []Runner   `json:"runners"`
}

// RunnerTarget is a parsed runner scope: exactly one of Org or Repo is set.
type RunnerTarget struct{ Org, Repo string }

// RunnerProvider lists the self-hosted runners registered to a scope.
type RunnerProvider interface {
	ListRunners(context.Context, RunnerTarget) ([]Runner, error)
}

// RunnerReader returns cached runner state without contacting providers.
type RunnerReader interface {
	Runners(context.Context) ([]RunnerScope, error)
}

var runnerPart = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)

func validRunnerPart(s string) bool { return runnerPart.MatchString(s) && !strings.Contains(s, "..") }

// ParseRunnerScope accepts "org:<name>" or "repo:<owner>/<name>".
func ParseRunnerScope(raw string) (RunnerTarget, error) {
	invalid := errors.New("runner scope must be org:<name> or repo:<owner>/<name>")
	kind, value, found := strings.Cut(raw, ":")
	if !found {
		return RunnerTarget{}, invalid
	}
	switch kind {
	case "org":
		if validRunnerPart(value) {
			return RunnerTarget{Org: value}, nil
		}
	case "repo":
		owner, name, ok := strings.Cut(value, "/")
		if ok && validRunnerPart(owner) && validRunnerPart(name) {
			return RunnerTarget{Repo: value}, nil
		}
	}
	return RunnerTarget{}, invalid
}

func (v Runner) MarshalJSON() ([]byte, error) {
	type plain Runner
	v.Labels = nonNil(v.Labels)
	return json.Marshal(plain(v))
}

func (v RunnerScope) MarshalJSON() ([]byte, error) {
	type plain RunnerScope
	v.Runners = nonNil(v.Runners)
	return json.Marshal(plain(v))
}
