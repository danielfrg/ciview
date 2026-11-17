package main

import (
	"errors"
	"net/http"

	"github.com/danielfrg/ciview/internal/ci"
	"github.com/danielfrg/ciview/internal/config"
	"github.com/danielfrg/ciview/internal/providers/github"
	"github.com/danielfrg/ciview/internal/runners"
)

// runnerCollector builds the runner collector, or nil when no runner scopes are
// configured. A scope with its own token reference gets a dedicated client; the
// rest reuse their source's provider.
func runnerCollector(cfg config.Config, providers map[string]ci.Provider, client *http.Client) (*runners.Collector, error) {
	if len(cfg.Runners) == 0 {
		return nil, nil
	}
	scopes := make([]runners.Scope, 0, len(cfg.Runners))
	for _, runner := range cfg.Runners {
		target, err := ci.ParseRunnerScope(runner.Scope)
		if err != nil {
			return nil, errors.New("Unable to configure runner scope")
		}
		var source config.Source
		for _, candidate := range cfg.Sources {
			if candidate.Name == runner.Source {
				source = candidate
			}
		}
		var provider ci.RunnerProvider
		if runner.TokenEnv != "" || runner.TokenFile != "" {
			token, err := config.ResolveToken(config.Source{Name: runner.Scope, TokenEnv: runner.TokenEnv, TokenFile: runner.TokenFile})
			if err != nil {
				return nil, err
			}
			if provider, err = github.New(source.Name, source.BaseURL, token, client); err != nil {
				return nil, errors.New("Unable to configure runner scope")
			}
		} else {
			var ok bool
			if provider, ok = providers[runner.Source].(ci.RunnerProvider); !ok {
				return nil, errors.New("Unable to configure runner scope")
			}
		}
		scopes = append(scopes, runners.Scope{Source: runner.Source, Scope: runner.Scope, Target: target, Provider: provider})
	}
	return runners.New(scopes, cfg.Viewer.RunnerDuration()), nil
}
