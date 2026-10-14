package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadDefaultsAndStrictFields(t *testing.T) {
	path := writeConfig(t, "[[sources]]\nname='public'\nprovider='github'\nrepos=['acme/app']\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Viewer.RefreshDuration() != time.Minute || cfg.Viewer.DiscoveryDuration() != 15*time.Minute || cfg.Viewer.HistoryRuns != 30 || cfg.Viewer.HistoryPages != 5 || cfg.Viewer.Branch != "default" {
		t.Fatalf("unexpected defaults: %+v", cfg.Viewer)
	}
	_, err = Load(writeConfig(t, "[viewer]\nunknown_limit=1\n[[sources]]\nname='public'\nprovider='github'\nrepos=['acme/app']\n"))
	if err == nil || !strings.Contains(err.Error(), "unknown configuration field at line") {
		t.Fatalf("unknown field accepted: %v", err)
	}
}

func TestValidateRejectsMalformedSourcesAndViewer(t *testing.T) {
	base := func() Config {
		return Config{Viewer: Viewer{RefreshInterval: "60s", DiscoveryInterval: "15m", HistoryRuns: 30, HistoryPages: 5, Branch: "default"}, Sources: []Source{{Name: "public", Provider: "github", Repos: []string{"acme/app"}}}}
	}
	for _, tc := range []struct {
		name   string
		change func(*Config)
	}{
		{"short refresh", func(c *Config) { c.Viewer.RefreshInterval = "4s" }},
		{"large discovery", func(c *Config) { c.Viewer.DiscoveryInterval = "25h" }},
		{"zero runs", func(c *Config) { c.Viewer.HistoryRuns = 0 }},
		{"too many pages", func(c *Config) { c.Viewer.HistoryPages = 21 }},
		{"blank branch", func(c *Config) { c.Viewer.Branch = " " }},
		{"unsafe branch", func(c *Config) { c.Viewer.Branch = "feature..bad" }},
		{"duplicate source", func(c *Config) { c.Sources = append(c.Sources, c.Sources[0]) }},
		{"empty selection", func(c *Config) { c.Sources[0].Repos = nil }},
		{"malformed repo", func(c *Config) { c.Sources[0].Repos = []string{"acme/app/extra"} }},
		{"bad provider", func(c *Config) { c.Sources[0].Provider = "gitlab" }},
		{"bad URL", func(c *Config) { c.Sources[0].BaseURL = "https://user:pass@example.test?q=x" }},
		{"both credentials", func(c *Config) { c.Sources[0].TokenEnv = "A"; c.Sources[0].TokenFile = "/tmp/token" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base()
			tc.change(&c)
			if err := c.Validate(); err == nil {
				t.Fatalf("invalid config accepted: %+v", c)
			}
		})
	}
}

func TestLoadDoesNotEchoMalformedValues(t *testing.T) {
	for _, tc := range []struct{ name, value, marker string }{
		{"oversized integer", "99999999999999999999999999999999999999", "99999999999999999999999999999999999999"},
		{"unterminated string", `"private-synthetic-marker`, "private-synthetic-marker"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writeConfig(t, "[[sources]]\nname='public'\nprovider='github'\nrepos=['acme/app']\ntoken_env="+tc.value+"\n")
			_, err := Load(path)
			if err == nil {
				t.Fatal("malformed TOML accepted")
			}
			if strings.Contains(err.Error(), tc.marker) {
				t.Fatalf("parser error echoed supplied value: %s", err)
			}
		})
	}
}

func TestValidateRejectsInvalidBranchComponents(t *testing.T) {
	base := Config{Viewer: Viewer{RefreshInterval: "60s", DiscoveryInterval: "15m", HistoryRuns: 30, HistoryPages: 5, Branch: "feature/branch"}, Sources: []Source{{Name: "public", Provider: "github", Repos: []string{"acme/app"}}}}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid branch rejected: %v", err)
	}
	for _, branch := range []string{"feature/.hidden", "feature.lock/test", "feature/test.lock"} {
		t.Run(branch, func(t *testing.T) {
			cfg := base
			cfg.Viewer.Branch = branch
			if err := cfg.Validate(); err == nil {
				t.Fatalf("invalid branch %q accepted", branch)
			}
		})
	}
}

func TestResolveTokenFailsClosedWithoutDisclosure(t *testing.T) {
	t.Setenv("CIVIEWER_TEST_TOKEN", "private-token-value")
	token, err := ResolveToken(Source{TokenEnv: "CIVIEWER_TEST_TOKEN"})
	if err != nil || token != "private-token-value" {
		t.Fatalf("environment token: %q, %v", token, err)
	}
	if _, err = ResolveToken(Source{TokenEnv: "CIVIEWER_TEST_MISSING"}); err == nil {
		t.Fatal("missing environment token accepted")
	}
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("  file-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	token, err = ResolveToken(Source{TokenFile: path})
	if err != nil || token != "file-secret" {
		t.Fatalf("file token: %q, %v", token, err)
	}
	if err := os.WriteFile(path, []byte(" \n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = ResolveToken(Source{TokenFile: path})
	if err == nil || strings.Contains(err.Error(), "file-secret") {
		t.Fatalf("empty file token error: %v", err)
	}
	if token, err = ResolveToken(Source{}); err != nil || token != "" {
		t.Fatalf("public source: %q, %v", token, err)
	}
}

func TestLoadUserRepositorySelections(t *testing.T) {
	for _, tc := range []struct {
		name, provider, extra string
	}{
		{"github user only", "github", ""},
		{"forgejo user only", "forgejo", "base_url='https://forge.example.test'\n"},
		{"mixed selectors", "github", "orgs=['example-org']\nrepos=['example-org/project']\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "[[sources]]\nname='public'\nprovider='" + tc.provider + "'\nusers=['example-user']\n" + tc.extra
			if _, err := Load(writeConfig(t, body)); err != nil {
				t.Fatalf("valid user selection rejected: %v", err)
			}
		})
	}
}

func TestLoadRejectsMalformedUserNames(t *testing.T) {
	for _, user := range []string{"", "example/project", "../example", " example", "example..user", "example?token=private-marker"} {
		t.Run(user, func(t *testing.T) {
			body := "[[sources]]\nname='public'\nprovider='github'\nusers=['" + user + "']\n"
			_, err := Load(writeConfig(t, body))
			if err == nil || !strings.Contains(err.Error(), ".users contains an invalid user") {
				t.Fatalf("malformed user selection should be rejected by validation: %v", err)
			}
			if strings.Contains(err.Error(), "private-marker") {
				t.Fatalf("validation error disclosed supplied value: %v", err)
			}
		})
	}
}
