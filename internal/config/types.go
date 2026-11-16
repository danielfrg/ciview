package config

import "time"

type Config struct {
	Viewer  Viewer        `toml:"viewer"`
	Sources []Source      `toml:"sources"`
	Runners []RunnerScope `toml:"runners"`
}

// RunnerScope selects one organization or repository whose self-hosted runners
// are read through a GitHub source. A token reference here overrides the
// source's token, so runner reads can use a separately scoped credential.
type RunnerScope struct {
	Source    string `toml:"source"`
	Scope     string `toml:"scope"`
	TokenEnv  string `toml:"token_env"`
	TokenFile string `toml:"token_file"`
}

type Viewer struct {
	RefreshInterval   string `toml:"refresh_interval"`
	DiscoveryInterval string `toml:"discovery_interval"`
	RunnerInterval    string `toml:"runner_interval"`
	HistoryRuns       int    `toml:"history_runs"`
	HistoryPages      int    `toml:"history_pages"`
	Branch            string `toml:"branch"`
}

type Source struct {
	ShowDependabotUpdates bool     `toml:"show_dependabot_updates"`
	ExistingWorkflowsOnly bool     `toml:"existing_workflows_only"`
	Name                  string   `toml:"name"`
	Provider              string   `toml:"provider"`
	BaseURL               string   `toml:"base_url"`
	TokenEnv              string   `toml:"token_env"`
	TokenFile             string   `toml:"token_file"`
	Orgs                  []string `toml:"orgs"`
	Users                 []string `toml:"users"`
	Repos                 []string `toml:"repos"`
}

func (v Viewer) RefreshDuration() time.Duration {
	d, _ := time.ParseDuration(v.RefreshInterval)
	return d
}
func (v Viewer) DiscoveryDuration() time.Duration {
	d, _ := time.ParseDuration(v.DiscoveryInterval)
	return d
}
func (v Viewer) RunnerDuration() time.Duration {
	d, err := time.ParseDuration(v.RunnerInterval)
	if err != nil || d <= 0 {
		return time.Minute
	}
	return d
}
