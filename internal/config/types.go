package config

import "time"

type Config struct {
	Viewer  Viewer   `toml:"viewer"`
	Sources []Source `toml:"sources"`
}

type Viewer struct {
	RefreshInterval   string `toml:"refresh_interval"`
	DiscoveryInterval string `toml:"discovery_interval"`
	HistoryRuns       int    `toml:"history_runs"`
	HistoryPages      int    `toml:"history_pages"`
	Branch            string `toml:"branch"`
}

type Source struct {
	Name      string   `toml:"name"`
	Provider  string   `toml:"provider"`
	BaseURL   string   `toml:"base_url"`
	TokenEnv  string   `toml:"token_env"`
	TokenFile string   `toml:"token_file"`
	Orgs      []string `toml:"orgs"`
	Users     []string `toml:"users"`
	Repos     []string `toml:"repos"`
}

func (v Viewer) RefreshDuration() time.Duration {
	d, _ := time.ParseDuration(v.RefreshInterval)
	return d
}
func (v Viewer) DiscoveryDuration() time.Duration {
	d, _ := time.ParseDuration(v.DiscoveryInterval)
	return d
}
