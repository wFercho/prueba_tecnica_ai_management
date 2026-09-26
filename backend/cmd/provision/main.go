// Command provision creates an account administratively without exposing signup
// or account management through the HTTP API.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/config"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/service"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "provision:", err)
		os.Exit(1)
	}
}

func run() error {
	email := strings.TrimSpace(os.Getenv("ACCOUNT_EMAIL"))
	password := os.Getenv("ACCOUNT_PASSWORD")
	if email == "" || password == "" {
		return fmt.Errorf("se requieren ACCOUNT_EMAIL y ACCOUNT_PASSWORD")
	}
	cfg, err := config.FromEnviron()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}
	if err := service.New(db, nil, analysis.DefaultDetectorConfig()).ProvisionUser(ctx, email, password); err != nil {
		return err
	}
	fmt.Fprintln(os.Stdout, "Cuenta provisionada si no existía:", email)
	return nil
}
