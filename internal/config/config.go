package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

var selectionPart = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)
var envName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Load reads startup configuration. Credentials remain references until runtime wiring.
func Load(path string) (Config, error) {
	if path == "" {
		path = "civiewer.toml"
	}
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open configuration: %w", err)
	}
	defer file.Close()
	cfg := Config{Viewer: Viewer{RefreshInterval: "60s", DiscoveryInterval: "15m", HistoryRuns: 30, HistoryPages: 5, Branch: "default"}}
	decoder := toml.NewDecoder(file).DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		var unknown *toml.StrictMissingError
		if errors.As(err, &unknown) && len(unknown.Errors) > 0 {
			row, column := unknown.Errors[0].Position()
			return Config{}, fmt.Errorf("unknown configuration field at line %d, column %d", row, column)
		}
		var decode *toml.DecodeError
		if errors.As(err, &decode) {
			row, column := decode.Position()
			return Config{}, fmt.Errorf("invalid TOML at line %d, column %d", row, column)
		}
		return Config{}, fmt.Errorf("invalid TOML configuration")
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate rejects invalid settings without reading or exposing credentials.
func (c Config) Validate() error {
	if err := validInterval("refresh_interval", c.Viewer.RefreshInterval); err != nil {
		return err
	}
	if err := validInterval("discovery_interval", c.Viewer.DiscoveryInterval); err != nil {
		return err
	}
	if c.Viewer.HistoryRuns < 1 || c.Viewer.HistoryRuns > 100 {
		return fmt.Errorf("history_runs must be from 1 to 100")
	}
	if c.Viewer.HistoryPages < 1 || c.Viewer.HistoryPages > 20 {
		return fmt.Errorf("history_pages must be from 1 to 20")
	}
	if !validBranch(c.Viewer.Branch) {
		return fmt.Errorf("branch must be default, all, or a valid branch name")
	}
	if len(c.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
	}
	seen := make(map[string]bool, len(c.Sources))
	for i, source := range c.Sources {
		label := fmt.Sprintf("sources[%d]", i)
		name := strings.TrimSpace(source.Name)
		if name == "" {
			return fmt.Errorf("%s.name must not be blank", label)
		}
		if seen[strings.ToLower(name)] {
			return fmt.Errorf("duplicate source name %q", name)
		}
		seen[strings.ToLower(name)] = true
		if source.Provider != "github" && source.Provider != "forgejo" {
			return fmt.Errorf("%s.provider must be github or forgejo", label)
		}
		if source.Provider == "forgejo" && source.BaseURL == "" {
			return fmt.Errorf("%s.base_url is required for forgejo", label)
		}
		if source.BaseURL != "" {
			u, err := url.Parse(source.BaseURL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
				return fmt.Errorf("%s.base_url must be an HTTP(S) instance URL without credentials, query, or fragment", label)
			}
		}
		if len(source.Orgs)+len(source.Repos) == 0 {
			return fmt.Errorf("%s must select an organization or repository", label)
		}
		for _, org := range source.Orgs {
			if !validPart(org) {
				return fmt.Errorf("%s.orgs contains an invalid organization", label)
			}
		}
		for _, repo := range source.Repos {
			parts := strings.Split(repo, "/")
			if len(parts) != 2 || !validPart(parts[0]) || !validPart(parts[1]) {
				return fmt.Errorf("%s.repos must use owner/name syntax", label)
			}
		}
		if source.TokenEnv != "" && source.TokenFile != "" {
			return fmt.Errorf("%s must use only one credential reference", label)
		}
		if source.TokenEnv != "" && !envName.MatchString(source.TokenEnv) {
			return fmt.Errorf("%s.token_env must name an environment variable", label)
		}
		if source.TokenFile != "" && strings.TrimSpace(source.TokenFile) == "" {
			return fmt.Errorf("%s.token_file must not be blank", label)
		}
	}
	return nil
}

func validInterval(name, raw string) error {
	d, err := time.ParseDuration(raw)
	if err != nil || d < 5*time.Second || d > 24*time.Hour {
		return fmt.Errorf("%s must be a duration from 5s to 24h", name)
	}
	return nil
}

func validPart(raw string) bool {
	return selectionPart.MatchString(raw) && raw != "." && raw != ".." && !strings.Contains(raw, "..")
}

func validBranch(raw string) bool {
	if raw == "default" || raw == "all" {
		return true
	}
	if raw == "" || strings.TrimSpace(raw) != raw || strings.HasPrefix(raw, "-") || strings.HasPrefix(raw, "/") || strings.HasSuffix(raw, "/") || strings.HasSuffix(raw, ".") || strings.Contains(raw, "..") || strings.Contains(raw, "//") || strings.Contains(raw, "@{") || strings.Contains(raw, "\\") || strings.HasSuffix(raw, ".lock") {
		return false
	}
	for _, component := range strings.Split(raw, "/") {
		if strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	for _, r := range raw {
		if r <= ' ' || r == 127 || strings.ContainsRune("~^:?*[", r) {
			return false
		}
	}
	return true
}

// ResolveToken returns a server-only token or an error that omits the secret value.
func ResolveToken(source Source) (string, error) {
	if source.TokenEnv != "" && source.TokenFile != "" {
		return "", fmt.Errorf("source %q has multiple credential references", source.Name)
	}
	if source.TokenEnv != "" {
		if !envName.MatchString(source.TokenEnv) {
			return "", fmt.Errorf("source %q has an invalid token_env", source.Name)
		}
		value := strings.TrimSpace(os.Getenv(source.TokenEnv))
		if value == "" {
			return "", fmt.Errorf("source %q token environment variable is missing or empty", source.Name)
		}
		return value, nil
	}
	if source.TokenFile != "" {
		bytes, err := os.ReadFile(source.TokenFile)
		if err != nil {
			return "", fmt.Errorf("source %q token file is unreadable", source.Name)
		}
		value := strings.TrimSpace(string(bytes))
		if value == "" {
			return "", fmt.Errorf("source %q token file is empty", source.Name)
		}
		return value, nil
	}
	return "", nil
}
