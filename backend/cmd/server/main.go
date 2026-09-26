// Command server runs the API, and the built dashboard from the same origin.
//
// The API and the dashboard are served together on purpose: one origin in production
// means no CORS headers anywhere and no second thing to deploy. In development the
// Vite dev server proxies to this process instead, which is the same arrangement from
// the browser's point of view.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/config"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/httpapi"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/narrate"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/service"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromEnviron()
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevelValue()}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := connect(ctx, cfg.DatabaseURL, log)
	if err != nil {
		return err
	}
	defer db.Close()

	// Applied on every boot, and idempotent, so a database that already has the
	// schema costs one no-op rather than a failed start.
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}

	narrator := chooseNarrator(cfg, log)
	svc := service.New(db, narrator, analysis.DefaultDetectorConfig())
	svc.SetLogger(log)
	if err := svc.ProvisionUser(ctx, "admin@email.com", cfg.DemoPassword); err != nil {
		return fmt.Errorf("provision demo account: %w", err)
	}

	handler := httpapi.New(svc, log)
	// A missing frontend build is not a reason to refuse to serve the API.
	if err := handler.ServeDashboard(cfg.StaticDir, log); err != nil {
		return err
	}

	server := &http.Server{
		Addr:    cfg.Address(),
		Handler: handler,
		// A write is bounded because analysis is synchronous up to the point of
		// narration; a read is bounded more tightly so a stuck query cannot hold a
		// browser open indefinitely.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Serving in a goroutine so shutdown is driven from the signal, not from the
	// listener returning.
	served := make(chan error, 1)
	go func() {
		log.Info("listening", "address", server.Addr, "dashboard", cfg.StaticDir,
			"narration", enabledName(cfg))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			served <- err
			return
		}
		served <- nil
	}()

	select {
	case err := <-served:
		return err
	case <-ctx.Done():
	}

	// Shutdown stops new requests, lets in-flight ones finish, and waits for the
	// background narration so a run's explanations are not lost to a restart.
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down: %w", err)
	}
	svc.WaitForNarration()
	return <-served
}

// connect waits for PostgreSQL, because `docker compose up` starts the API and the
// database together and the database is usually not listening yet. Failing on the
// first refusal would make the documented start order a race.
func connect(ctx context.Context, url string, log *slog.Logger) (*postgres.DB, error) {
	const attempts = 30
	var lastErr error
	for attempt := range attempts {
		db, err := postgres.Open(ctx, url)
		if err == nil {
			if attempt > 0 {
				log.Info("the database became available", "attempts", attempt+1)
			}
			return db, nil
		}
		lastErr = err
		log.Warn("the database is not available yet", "attempt", attempt+1, "of", attempts, "error", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil, fmt.Errorf("gave up waiting for %s: %w", url, lastErr)
}

// chooseNarrator returns the narrator the configuration asks for.
//
// With no API key the deterministic narrator is used, and that is a supported
// deployment rather than a degraded one: the findings, their types, severities,
// confidences and actions are all already there, and only the wording of the
// explanation is the template's.
func chooseNarrator(cfg config.Config, log *slog.Logger) narrate.Narrator {
	if !cfg.NarrationEnabled() {
		log.Info("no OPENAI_API_KEY, so explanations will be the deterministic ones",
			"hint", "set OPENAI_API_KEY to have them narrated")
		return narrate.Rules{}
	}
	log.Info("explanations will be narrated", "model", cfg.Model)
	return narrate.OpenAI{
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	}
}

func enabledName(cfg config.Config) string {
	if cfg.NarrationEnabled() {
		return cfg.Model
	}
	return "rules"
}
