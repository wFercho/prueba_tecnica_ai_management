// Package store defines the persistence the API needs and embeds its schema.
//
// The API depends on the interfaces here, never on pgx or on SQL, so the HTTP
// layer can be tested against an in-memory implementation without a database and
// the domain can be reasoned about without either.
package store

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate applies the schema. Every statement is idempotent, so it is safe to
// call on every boot: an evaluator who runs the stack twice should not have to
// think about whether the database is fresh.
func Migrate(ctx context.Context, exec Execer) error {
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		statement, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := exec.Exec(ctx, string(statement)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

// Execer is the one capability Migrate needs from a database.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (int64, error)
}

// Store is everything the API reads or writes. It is one interface rather than
// five because the HTTP layer needs them together, and because a test
// implementation that satisfies all of them is a usable fake for the whole API.
type Store interface {
	MeterReader
	MeterWriter
	ReadingReader
	ReadingWriter
	EventReader
	EventWriter
	RunReader
	RunWriter
	AnomalyReader
	AnomalyWriter
}

// MeterReader reads the meter catalogue.
type MeterReader interface {
	// Meters returns the catalogue ordered by meter id, so the meters table has
	// a stable order across requests.
	Meters(ctx context.Context) ([]catalog.Meter, error)
	// Meter returns one meter, or ErrNotFound.
	Meter(ctx context.Context, meterID string) (catalog.Meter, error)
}

// MeterWriter writes the meter catalogue.
type MeterWriter interface {
	// UpsertMeters inserts the catalogue, updating the descriptive fields of
	// meters that already exist. Seeding is idempotent: a second run of the
	// importer must not fail, and must not duplicate the twelve meters.
	UpsertMeters(ctx context.Context, meters []catalog.Meter) error
}

// ReadingReader reads the hourly series.
type ReadingReader interface {
	// Readings returns a meter's readings between two instants inclusive, in time
	// order. A zero bound means unbounded on that side.
	Readings(ctx context.Context, meterID string, from, to time.Time) ([]catalog.Reading, error)
	// AllReadings returns every reading in the window, for the detector.
	AllReadings(ctx context.Context) ([]catalog.Reading, error)
	// ReadingWindow returns the earliest and latest reading across every meter,
	// which is the window the last run analysed.
	ReadingWindow(ctx context.Context) (from, to time.Time, err error)
	// ConsumptionTotals returns total consumption per meter over the window, for
	// the dashboard. It is one aggregate rather than 4,032 rows, because the
	// dashboard needs the sum and nothing else.
	ConsumptionTotals(ctx context.Context) (map[string]float64, error)
}

// ReadingWriter writes the hourly series.
type ReadingWriter interface {
	// AppendReadings stores a batch of readings. It is idempotent on
	// (meter, timestamp): re-importing an hour replaces it rather than duplicating
	// it. The importer is run by hand, on boot in development, and by anyone
	// following the README, so running it twice has to be safe — and a re-import is
	// also how a corrected reading arrives, which is why the second write wins.
	AppendReadings(ctx context.Context, readings []catalog.Reading) error
}

// EventReader reads reported operational events.
type EventReader interface {
	Events(ctx context.Context) ([]catalog.OperationalEvent, error)
}

// EventWriter writes reported operational events.
type EventWriter interface {
	// AppendEvents stores a batch of events, idempotent on
	// (meter, timestamp, type): the same event reported twice is one event, while two
	// different events at the same hour are two.
	AppendEvents(ctx context.Context, events []catalog.OperationalEvent) error
}

// RunReader reads what past runs did, so a run id handed out by the analyse
// endpoint can be polled while it works.
type RunReader interface {
	// Run returns one run, or ErrNotFound.
	Run(ctx context.Context, runID int64) (Run, error)
	// LatestRun returns the most recent run, or ErrNotFound when none has run.
	LatestRun(ctx context.Context) (Run, error)
}

// RunWriter records what the detector did.
type RunWriter interface {
	// StartRun opens a RUNNING run over a window and returns its id.
	StartRun(ctx context.Context, windowStart, windowEnd time.Time) (int64, error)
	// FinishRun closes a run: COMPLETED with the number of anomalies found, or
	// FAILED with the reason.
	FinishRun(ctx context.Context, runID int64, state RunState, anomalyCount int, failure string) error
}

// AnomalyWriter persists findings.
type AnomalyWriter interface {
	// SaveAnomalies writes a run's anomalies in one transaction. A run that
	// persists only some of its anomalies would be a run the dashboard cannot
	// trust, so it is all or nothing.
	SaveAnomalies(ctx context.Context, runID int64, anomalies []analysis.Anomaly) error
	// Narrate upgrades one anomaly's prose to what the narrator produced. It
	// records the source either way, so a rules explanation that survived a
	// failed narration is still identifiable as such.
	Narrate(ctx context.Context, anomalyID int64, source ExplanationSource, reason, action string) error
	// MarkNarrationFailed records that narration failed, leaving the rules prose
	// in place. A failed narration is visible as failed rather than absent
	// (ADR-0006).
	MarkNarrationFailed(ctx context.Context, anomalyID int64) error
	// SetStatus moves an anomaly between OPEN, ACKNOWLEDGED, RESOLVED and
	// DISMISSED, which is what the investigation view writes.
	SetStatus(ctx context.Context, anomalyID int64, status AnomalyStatus) error
}

// AnomalyReader reads findings.
type AnomalyReader interface {
	// Anomalies returns the latest run's anomalies, most urgent first.
	Anomalies(ctx context.Context) ([]analysis.Anomaly, error)
	// Anomaly returns one anomaly with its per-hour deviation series, for the
	// investigation view.
	Anomaly(ctx context.Context, id int64) (AnomalyDetail, error)
}

// ErrNotFound is returned when a requested row does not exist. Callers test for
// it with errors.Is, so a store can wrap it with whatever context it has.
var ErrNotFound = fmt.Errorf("not found")

// RunState is a run's lifecycle state.
type RunState string

const (
	RunRunning   RunState = "RUNNING"
	RunCompleted RunState = "COMPLETED"
	RunFailed    RunState = "FAILED"
)

// The status vocabularies live in catalog because the API, the store and the
// anomaly type all speak them, and a duplicate declaration in each would let them
// drift apart silently.
type (
	// AnomalyStatus is where an anomaly is in the investigation workflow.
	AnomalyStatus = catalog.AnomalyStatus
	// ExplanationSource is who wrote an anomaly's prose.
	ExplanationSource = catalog.ExplanationSource
	// ExplanationStatus is whether an anomaly's prose is the template or the
	// narrator's.
	ExplanationStatus = catalog.ExplanationStatus
)

const (
	StatusOpen         = catalog.StatusOpen
	StatusAcknowledged = catalog.StatusAcknowledged
	StatusResolved     = catalog.StatusResolved
	StatusDismissed    = catalog.StatusDismissed

	SourceRules = catalog.SourceRules
	SourceLLM   = catalog.SourceLLM

	ExplanationPending = catalog.ExplanationPending
	ExplanationReady   = catalog.ExplanationReady
	ExplanationFailed  = catalog.ExplanationFailed
)

// Run is one pass of the detector over a window.
type Run struct {
	ID           int64      `json:"id"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	State        RunState   `json:"state"`
	AnomalyCount int        `json:"anomaly_count"`
	WindowStart  time.Time  `json:"window_start"`
	WindowEnd    time.Time  `json:"window_end"`
	Error        string     `json:"error,omitempty"`
}

// AnomalyDetail is an anomaly with the evidence behind it, which is what the
// investigation view shows. The per-hour deviation series is part of the anomaly
// itself: the detector computed it and it is persisted, rather than recomputed at
// read time from a baseline that may since have changed (ADR-0007).
type AnomalyDetail = analysis.Anomaly

// sqlPlaceholders is a small helper for building IN clauses.
func sqlPlaceholders(n int) string {
	if n == 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("$%d,", n), ",")
}
