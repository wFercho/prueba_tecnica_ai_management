// Command seed imports the delivered CSV files into the database.
//
// It is safe to run as often as you like: readings and events are keyed on their
// identity, so a second import replaces the hours it already holds rather than
// duplicating them. That matters because the importer is run by hand, on boot in
// development, and by anyone following the README.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/config"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/ingest"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store/postgres"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	dataDir := flag.String("data", "", "directory holding readings.csv and events.csv (overrides DATA_DIR)")
	databaseURL := flag.String("database", "", "PostgreSQL connection URL (overrides DATABASE_URL)")
	flag.Parse()

	cfg, err := config.FromEnviron()
	if err != nil {
		return err
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *databaseURL != "" {
		cfg.DatabaseURL = *databaseURL
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	readings, err := readReadings(cfg.ReadingsFile())
	if err != nil {
		return err
	}
	events, err := readEvents(cfg.EventsFile())
	if err != nil {
		return err
	}

	// Refused before the database is touched: a file that cannot be stored should not
	// leave half an import behind.
	if err := catalog.Validate(readings); err != nil {
		return fmt.Errorf("%s: %w", cfg.ReadingsFile(), err)
	}
	if err := catalog.ValidateEvents(events); err != nil {
		return fmt.Errorf("%s: %w", cfg.EventsFile(), err)
	}
	from, to, ok := catalog.Window(readings)
	if !ok {
		return fmt.Errorf("%s holds no readings", cfg.ReadingsFile())
	}

	// The delivered data carries no catalogue of its own, so the meters are derived
	// from the readings that mention them.
	meters := catalog.FromReadings(readings)
	log.Info("read the delivered data",
		"meters", len(meters), "readings", len(readings), "events", len(events),
		"from", from.Format(time.RFC3339), "to", to.Format(time.RFC3339))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", cfg.DatabaseURL, err)
	}
	defer db.Close()

	// The schema is applied here as well as on boot, so seeding into an empty
	// PostgreSQL is one command rather than two.
	if err := store.Migrate(ctx, db); err != nil {
		return err
	}

	if err := db.ReplaceDataset(ctx, meters, readings, events); err != nil {
		return err
	}

	// Read the window back rather than reporting what was sent: a silent partial
	// import is the failure an operator would otherwise meet on an empty dashboard.
	storedFrom, storedTo, err := db.ReadingWindow(ctx)
	if err != nil {
		return fmt.Errorf("the import left no readings behind: %w", err)
	}
	stored, err := db.AllReadings(ctx)
	if err != nil {
		return err
	}
	log.Info("imported", "meters", len(meters), "events", len(events),
		"readings", len(stored),
		"window", fmt.Sprintf("%s..%s", storedFrom.Format(time.RFC3339), storedTo.Format(time.RFC3339)))
	if len(stored) != len(readings) {
		log.Warn("the database holds a different number of readings than the file holds",
			"file", len(readings), "database", len(stored))
	}
	return nil
}

func readReadings(path string) ([]catalog.Reading, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	readings, err := ingest.ReadReadings(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return readings, nil
}

func readEvents(path string) ([]catalog.OperationalEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	events, err := ingest.ReadEvents(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return events, nil
}
