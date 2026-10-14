package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"civiewer/internal/config"
)

func writeConfig(t *testing.T, token string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.toml")
	body := "[[sources]]\nname='public'\nprovider='github'\nrepos=['actions/checkout']\n" + token
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestConfiguredCredentialsFailStartupSafely(t *testing.T) {
	t.Setenv("CIVIEWER_TEST_MISSING_TOKEN", "")
	err := run(context.Background(), writeConfig(t, "token_env='CIVIEWER_TEST_MISSING_TOKEN'\n"), "127.0.0.1:0")
	if err == nil || !strings.Contains(err.Error(), "missing or empty") {
		t.Fatalf("missing token should fail startup: %v", err)
	}
	path := filepath.Join(t.TempDir(), "private-token")
	err = run(context.Background(), writeConfig(t, "token_file='"+path+"'\n"), "127.0.0.1:0")
	if err == nil || strings.Contains(err.Error(), path) {
		t.Fatalf("file failure leaks path: %v", err)
	}
}
func TestCanceledStartupAndEnvironmentDefaults(t *testing.T) {
	t.Setenv("CIVIEWER_CONFIG", "")
	t.Setenv("CIVIEWER_ADDR", "")
	if envDefault("CIVIEWER_CONFIG", "civiewer.toml") != "civiewer.toml" || envDefault("CIVIEWER_ADDR", "127.0.0.1:8080") != "127.0.0.1:8080" {
		t.Fatal("startup defaults differ")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, writeConfig(t, ""), "127.0.0.1:0") }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("canceled shutdown: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked")
	}
}
func TestExampleIsRunnableWithoutCredentials(t *testing.T) {
	cfg, err := config.Load("../../civiewer.example.toml")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sources) != 1 || cfg.Sources[0].TokenEnv != "" || cfg.Sources[0].TokenFile != "" || len(cfg.Sources[0].Repos) != 1 || cfg.Sources[0].Repos[0] != "actions/checkout" {
		t.Fatalf("example requires private configuration: %#v", cfg.Sources)
	}
}

func TestShutdownCancelsActiveProviderRequest(t *testing.T) {
	started := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer upstream.Close()
	cfgPath := writeConfig(t, "base_url='"+upstream.URL+"'\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, cfgPath, "127.0.0.1:0") }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("provider request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("shutdown failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown left provider request running")
	}
}
