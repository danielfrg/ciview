package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ciview/internal/ci"
	"ciview/internal/config"
	"ciview/internal/httpapi"
	"ciview/internal/mcp"
	"ciview/internal/providers/forgejo"
	"ciview/internal/providers/github"
	"ciview/internal/refresh"
	"ciview/internal/snapshots"
	"ciview/internal/web"
)

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, envDefault("CIVIEW_CONFIG", "ciview.toml"), envDefault("CIVIEW_ADDR", "127.0.0.1:8080")); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, path, addr string) error {
	hosts, err := mcp.ParseAllowedHosts(os.Getenv("CIVIEW_MCP_ALLOWED_HOSTS"))
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	providers := make(map[string]ci.Provider, len(cfg.Sources))
	client := &http.Client{Timeout: 25 * time.Second}
	for _, source := range cfg.Sources {
		token, err := config.ResolveToken(source)
		if err != nil {
			return err
		}
		switch source.Provider {
		case "github":
			providers[source.Name], err = github.New(source.Name, source.BaseURL, token, client)
		case "forgejo":
			providers[source.Name], err = forgejo.New(source.Name, source.BaseURL, token, client)
		}
		if err != nil {
			return errors.New("Unable to configure source provider")
		}
	}
	if ctx.Err() != nil {
		return nil
	}
	store := snapshots.NewMemory()
	coordinator := refresh.New(cfg, providers, store)
	overview := ci.NewOverview(store, coordinator, cfg.Viewer.HistoryRuns, cfg.Viewer.Branch, cfg.Viewer.RefreshDuration())
	server := &http.Server{Addr: addr, Handler: httpapi.NewHandler(web.Assets(), overview, hosts...), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.New("Unable to listen on CIVIEW_ADDR")
	}
	refreshCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	refreshed := make(chan struct{})
	go func() { coordinator.Start(refreshCtx); close(refreshed) }()
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-served:
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		_ = server.Close()
	}
	<-refreshed
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return errors.New("HTTP server stopped unexpectedly")
	}
	return nil
}
