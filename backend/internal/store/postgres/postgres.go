// Package postgres implements store.Store against PostgreSQL 17 over pgx.
//
// It is the only package in the backend that knows SQL exists. Everything above it
// — the HTTP layer, the detector — depends on the interfaces in store.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

// DB is a store backed by a connection pool. It satisfies store.Store.
type DB struct {
	pool *pgxpool.Pool
}

// Open connects to PostgreSQL and verifies the connection. The verification is
// not ceremony: a compose stack is usually started with the API waiting on the
// database, and failing here with a clear message beats failing later on the
// first query with a timeout.
func Open(ctx context.Context, url string) (*DB, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", redact(url), err)
	}
	// Every timestamp is read in UTC.
	//
	// The delivered CSVs carry no zone and the ingest reads them as UTC. pgx would
	// otherwise decode each timestamptz into the process's local zone, so a window
	// the file writes as 2026-09-01T00:00:00Z would reach the dashboard as the
	// previous evening, and an episode the detector describes as starting at 14:00
	// would be drawn at 09:00. The instant is the same either way; only its rendering
	// changes, which is exactly the part an operator reads.
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		registerUTCCodecs(conn.TypeMap())
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping %s: %w", redact(url), err)
	}
	return &DB{pool: pool}, nil
}

// registerUTCCodecs makes timestamptz and timestamp decode into UTC. Each pooled
// connection has its own type map, so this runs per connection rather than once.
func registerUTCCodecs(m *pgtype.Map) {
	for _, name := range []string{"timestamptz", "timestamp"} {
		oid, ok := m.TypeForName(name)
		if !ok {
			continue
		}
		m.RegisterType(&pgtype.Type{Name: name, OID: oid.OID, Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC}})
	}
}

// Close releases the pool.
func (db *DB) Close() { db.pool.Close() }

// Exec applies one statement. It satisfies store.Execer, so store.Migrate can
// drive the schema through the same pool the API uses.
func (db *DB) Exec(ctx context.Context, sql string, args ...any) (int64, error) {
	tag, err := db.pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// redact hides the password in a connection string before it reaches a log.
//
// It parses the URL rather than looking for the first colon: in
// postgres://user:secret@host/db the first colon belongs to the scheme, so scanning
// for a separator would strip the scheme and print the password in full. pgx also
// accepts the key/value DSN form, where there is no URL to parse and the password
// is a plain field, so that form is redacted separately.
func redact(connection string) string {
	if parsed, err := url.Parse(connection); err == nil && parsed.Scheme != "" && parsed.User != nil {
		if _, hasPassword := parsed.User.Password(); hasPassword {
			parsed.User = url.UserPassword(parsed.User.Username(), "***")
		}
		return parsed.String()
	}
	return redactDSN(connection)
}

// redactDSN redacts the password field of a `key=value` connection string.
func redactDSN(connection string) string {
	fields := strings.Fields(connection)
	for i, field := range fields {
		key, value, found := strings.Cut(field, "=")
		if !found || !strings.EqualFold(key, "password") || value == "" {
			continue
		}
		fields[i] = key + "=***"
	}
	return strings.Join(fields, " ")
}

const meterColumns = `id, meter_id, name, location, created_at`

func (db *DB) Meters(ctx context.Context) ([]catalog.Meter, error) {
	rows, err := db.pool.Query(ctx, `SELECT `+meterColumns+` FROM meters ORDER BY meter_id`)
	if err != nil {
		return nil, fmt.Errorf("list meters: %w", err)
	}
	defer rows.Close()

	var meters []catalog.Meter
	for rows.Next() {
		var meter catalog.Meter
		if err := rows.Scan(&meter.ID, &meter.Code, &meter.Name, &meter.Location, &meter.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan meter: %w", err)
		}
		meters = append(meters, meter)
	}
	return meters, rows.Err()
}

func (db *DB) Meter(ctx context.Context, meterID string) (catalog.Meter, error) {
	var meter catalog.Meter
	err := db.pool.QueryRow(ctx,
		`SELECT `+meterColumns+` FROM meters WHERE meter_id = $1`, meterID).
		Scan(&meter.ID, &meter.Code, &meter.Name, &meter.Location, &meter.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.Meter{}, fmt.Errorf("meter %s: %w", meterID, store.ErrNotFound)
	}
	if err != nil {
		return catalog.Meter{}, fmt.Errorf("get meter %s: %w", meterID, err)
	}
	return meter, nil
}

// UpsertMeters inserts the catalogue, updating the descriptive fields of meters
// that already exist, so seeding twice does not fail and does not duplicate.
func (db *DB) UpsertMeters(ctx context.Context, meters []catalog.Meter) error {
	batch := &pgx.Batch{}
	for _, meter := range meters {
		batch.Queue(`
			INSERT INTO meters (meter_id, name, location)
			VALUES ($1, $2, $3)
			ON CONFLICT (meter_id) DO UPDATE SET name = EXCLUDED.name, location = EXCLUDED.location`,
			string(meter.Code), meter.Name, meter.Location)
	}
	results := db.pool.SendBatch(ctx, batch)
	defer results.Close()
	for range meters {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("upsert meters: %w", err)
		}
	}
	return results.Close()
}

// AppendReadings stores a batch in one round trip, keyed on (meter, hour) so a
// re-import replaces the hour rather than adding a second reading of it.
func (db *DB) AppendReadings(ctx context.Context, readings []catalog.Reading) error {
	if len(readings) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, reading := range readings {
		batch.Queue(`
			INSERT INTO readings (meter_id, timestamp, consumption_kwh, voltage_v, current_a, power_factor, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (meter_id, timestamp) DO UPDATE SET
				consumption_kwh = EXCLUDED.consumption_kwh,
				voltage_v       = EXCLUDED.voltage_v,
				current_a       = EXCLUDED.current_a,
				power_factor    = EXCLUDED.power_factor,
				status          = EXCLUDED.status`,
			string(reading.MeterCode), reading.Timestamp, reading.ConsumptionKWh,
			reading.VoltageV, reading.CurrentA, reading.PowerFactor, reading.IngestedStatus)
	}
	results := db.pool.SendBatch(ctx, batch)
	defer results.Close()
	for range readings {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("append readings: %w", err)
		}
	}
	return results.Close()
}

// AppendEvents stores a batch, keyed on (meter, hour, type) so one event reported
// twice stays one event.
func (db *DB) AppendEvents(ctx context.Context, events []catalog.OperationalEvent) error {
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, event := range events {
		batch.Queue(`
			INSERT INTO events (meter_id, timestamp, type, description)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (meter_id, timestamp, type) DO UPDATE SET
				description = EXCLUDED.description`,
			string(event.MeterCode), event.Timestamp, string(event.Type), event.Description)
	}
	results := db.pool.SendBatch(ctx, batch)
	defer results.Close()
	for range events {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("append events: %w", err)
		}
	}
	return results.Close()
}

const readingColumns = `meter_id, timestamp, consumption_kwh, voltage_v, current_a, power_factor, status`

func scanReadings(rows pgx.Rows) ([]catalog.Reading, error) {
	defer rows.Close()
	var readings []catalog.Reading
	for rows.Next() {
		var reading catalog.Reading
		var meterID string
		if err := rows.Scan(&meterID, &reading.Timestamp, &reading.ConsumptionKWh, &reading.VoltageV,
			&reading.CurrentA, &reading.PowerFactor, &reading.IngestedStatus); err != nil {
			return nil, fmt.Errorf("scan reading: %w", err)
		}
		reading.MeterCode = catalog.MeterCode(meterID)
		readings = append(readings, reading)
	}
	return readings, rows.Err()
}

func (db *DB) Readings(ctx context.Context, meterID string, from, to time.Time) ([]catalog.Reading, error) {
	// A zero bound means unbounded on that side, as the contract says. It cannot be
	// passed through: the zero time is year one, and `timestamp >= '0001-01-01'` would
	// silently work while `timestamp <= '0001-01-01'` returned nothing at all — which
	// is how a whole meter's history came back empty.
	query := `SELECT ` + readingColumns + ` FROM readings WHERE meter_id = $1`
	args := []any{meterID}
	if !from.IsZero() {
		args = append(args, from)
		query += fmt.Sprintf(" AND timestamp >= $%d", len(args))
	}
	if !to.IsZero() {
		args = append(args, to)
		query += fmt.Sprintf(" AND timestamp <= $%d", len(args))
	}
	query += ` ORDER BY timestamp`

	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("readings for %s: %w", meterID, err)
	}
	return scanReadings(rows)
}

func (db *DB) AllReadings(ctx context.Context) ([]catalog.Reading, error) {
	rows, err := db.pool.Query(ctx, `SELECT `+readingColumns+` FROM readings ORDER BY meter_id, timestamp`)
	if err != nil {
		return nil, fmt.Errorf("all readings: %w", err)
	}
	return scanReadings(rows)
}

func (db *DB) ReadingWindow(ctx context.Context) (from, to time.Time, err error) {
	err = db.pool.QueryRow(ctx, `SELECT min(timestamp), max(timestamp) FROM readings`).Scan(&from, &to)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, time.Time{}, fmt.Errorf("reading window: %w", store.ErrNotFound)
	}
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("reading window: %w", err)
	}
	return from, to, nil
}

func (db *DB) ConsumptionTotals(ctx context.Context) (map[string]float64, error) {
	rows, err := db.pool.Query(ctx, `SELECT meter_id, sum(consumption_kwh) FROM readings GROUP BY meter_id`)
	if err != nil {
		return nil, fmt.Errorf("consumption totals: %w", err)
	}
	defer rows.Close()

	totals := map[string]float64{}
	for rows.Next() {
		var meterID string
		var total float64
		if err := rows.Scan(&meterID, &total); err != nil {
			return nil, fmt.Errorf("scan total: %w", err)
		}
		totals[meterID] = total
	}
	return totals, rows.Err()
}

func (db *DB) Events(ctx context.Context) ([]catalog.OperationalEvent, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT meter_id, timestamp, type, description FROM events ORDER BY meter_id, timestamp`)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	var events []catalog.OperationalEvent
	for rows.Next() {
		var event catalog.OperationalEvent
		var meterID string
		if err := rows.Scan(&meterID, &event.Timestamp, &event.Type, &event.Description); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		event.MeterCode = catalog.MeterCode(meterID)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (db *DB) StartRun(ctx context.Context, windowStart, windowEnd time.Time) (int64, error) {
	var id int64
	err := db.pool.QueryRow(ctx, `
		INSERT INTO analysis_runs (state, window_start, window_end)
		VALUES ($1, $2, $3)
		RETURNING id`, store.RunRunning, windowStart, windowEnd).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("start run: %w", err)
	}
	return id, nil
}

func (db *DB) FinishRun(ctx context.Context, runID int64, state store.RunState, anomalyCount int, failure string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE analysis_runs
		SET state = $2, anomaly_count = $3, error = $4, finished_at = now()
		WHERE id = $1`, runID, state, anomalyCount, failure)
	if err != nil {
		return fmt.Errorf("finish run %d: %w", runID, err)
	}
	return nil
}

func (db *DB) LatestRun(ctx context.Context) (store.Run, error) {
	var run store.Run
	err := db.pool.QueryRow(ctx, `
		SELECT id, started_at, finished_at, state, anomaly_count, window_start, window_end, error
		FROM analysis_runs
		ORDER BY id DESC
		LIMIT 1`).
		Scan(&run.ID, &run.StartedAt, &run.FinishedAt, &run.State, &run.AnomalyCount,
			&run.WindowStart, &run.WindowEnd, &run.Error)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Run{}, fmt.Errorf("latest run: %w", store.ErrNotFound)
	}
	if err != nil {
		return store.Run{}, fmt.Errorf("latest run: %w", err)
	}
	return run, nil
}

// SaveAnomalies writes a run's anomalies in one transaction. A run whose
// anomalies are only partly persisted is a run the dashboard cannot trust, and a
// half-written anomaly list is worse than none: the count on the run would not
// match the rows.
func (db *DB) SaveAnomalies(ctx context.Context, runID int64, anomalies []analysis.Anomaly) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, anomaly := range anomalies {
		basis, err := json.Marshal(anomaly.ConfidenceBasis)
		if err != nil {
			return fmt.Errorf("marshal confidence basis: %w", err)
		}
		findings, err := json.Marshal(orEmptyArray(anomaly.Findings))
		if err != nil {
			return fmt.Errorf("marshal findings: %w", err)
		}
		series, err := json.Marshal(orEmptyArray(anomaly.DeviationSeries))
		if err != nil {
			return fmt.Errorf("marshal deviation series: %w", err)
		}
		var correlatedEvent any
		if anomaly.CorrelatedEvent != nil {
			encoded, err := json.Marshal(anomaly.CorrelatedEvent)
			if err != nil {
				return fmt.Errorf("marshal correlated event: %w", err)
			}
			correlatedEvent = encoded
		}
		corroborating := anomaly.Corroborating
		if corroborating == nil {
			corroborating = []string{}
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO anomalies (
				run_id, meter_id, window_start, window_end, affected_reading_count,
				type, severity, confidence, confidence_basis, deviation_percent,
				actual_kwh, baseline_kwh, corroborating, findings, deviation_series,
				correlated_event, reason, recommended_action, detected_by,
				explanation_source, explanation_status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21)`,
			runID, anomaly.MeterCode, anomaly.WindowStart, anomaly.WindowEnd, anomaly.AffectedReadings,
			anomaly.Type, anomaly.Severity, anomaly.Confidence, basis, anomaly.DeviationPercent,
			anomaly.ActualKWh, anomaly.BaselineKWh, corroborating, findings, series,
			correlatedEvent, anomaly.Reason, anomaly.RecommendedAction, anomaly.DetectedBy,
			store.SourceRules, store.ExplanationPending)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("anomaly for %s at %s is already recorded in run %d: %w",
					anomaly.MeterCode, anomaly.WindowStart, runID, err)
			}
			return fmt.Errorf("insert anomaly: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// orEmptyArray marshals a nil slice as [] rather than null, so the JSON columns
// are always the same shape and the frontend never has to handle a null array.
func orEmptyArray[T any](value []T) []T {
	if value == nil {
		return []T{}
	}
	return value
}

func (db *DB) Narrate(ctx context.Context, anomalyID int64, source store.ExplanationSource, reason, action string) error {
	return db.setProse(ctx, anomalyID, string(source), string(store.ExplanationReady), reason, action)
}

// MarkNarrationFailed leaves the rules prose in place and records the failure, so
// a failure is visible as failed rather than as an anomaly that never had an
// explanation (ADR-0006).
func (db *DB) MarkNarrationFailed(ctx context.Context, anomalyID int64) error {
	_, err := db.pool.Exec(ctx,
		`UPDATE anomalies SET explanation_status = $2 WHERE id = $1`,
		anomalyID, store.ExplanationFailed)
	if err != nil {
		return fmt.Errorf("mark narration failed for %d: %w", anomalyID, err)
	}
	return nil
}

func (db *DB) setProse(ctx context.Context, anomalyID int64, source, status, reason, action string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE anomalies
		SET reason = $2, recommended_action = $3, explanation_source = $4, explanation_status = $5
		WHERE id = $1`, anomalyID, reason, action, source, status)
	if err != nil {
		return fmt.Errorf("set prose for %d: %w", anomalyID, err)
	}
	return nil
}

func (db *DB) SetStatus(ctx context.Context, anomalyID int64, status store.AnomalyStatus) error {
	tag, err := db.pool.Exec(ctx, `UPDATE anomalies SET status = $2 WHERE id = $1`, anomalyID, status)
	if err != nil {
		return fmt.Errorf("set status for %d: %w", anomalyID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("anomaly %d: %w", anomalyID, store.ErrNotFound)
	}
	return nil
}

// severityRank and typeRank order the anomalies table the way an operator reads
// it: worst first, and within a severity, the most evident first. It is a fixed
// ranking in SQL rather than a computed mean of the confidence band, because an
// average across types ranks a dismissed false positive above an unexplained
// real anomaly (ADR-0010).
const anomalyOrder = `
	ORDER BY
		CASE severity WHEN 'HIGH' THEN 0 WHEN 'MEDIUM' THEN 1 ELSE 2 END,
		CASE type
			WHEN 'REAL_ANOMALY' THEN 0
			WHEN 'DATA_QUALITY' THEN 1
			WHEN 'EXPLAINABLE' THEN 2
			ELSE 3
		END,
		confidence DESC,
		window_start DESC`

const anomalyColumns = `
	id, run_id, meter_id, window_start, window_end, affected_reading_count, type, severity,
	confidence, confidence_basis, deviation_percent, actual_kwh, baseline_kwh,
	corroborating, findings, deviation_series, correlated_event, reason,
	recommended_action, detected_by, explanation_source, explanation_status, status`

func (db *DB) Anomalies(ctx context.Context) ([]analysis.Anomaly, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT `+anomalyColumns+`
		FROM anomalies
		WHERE run_id = (SELECT id FROM analysis_runs ORDER BY id DESC LIMIT 1)`+anomalyOrder)
	if err != nil {
		return nil, fmt.Errorf("list anomalies: %w", err)
	}
	defer rows.Close()

	var anomalies []analysis.Anomaly
	for rows.Next() {
		anomaly, err := scanAnomaly(rows)
		if err != nil {
			return nil, err
		}
		anomalies = append(anomalies, anomaly)
	}
	return anomalies, rows.Err()
}

func (db *DB) Anomaly(ctx context.Context, id int64) (store.AnomalyDetail, error) {
	rows, err := db.pool.Query(ctx, `SELECT `+anomalyColumns+` FROM anomalies WHERE id = $1`, id)
	if err != nil {
		return store.AnomalyDetail{}, fmt.Errorf("anomaly %d: %w", id, err)
	}
	defer rows.Close()
	if !rows.Next() {
		return store.AnomalyDetail{}, fmt.Errorf("anomaly %d: %w", id, store.ErrNotFound)
	}
	anomaly, err := scanAnomaly(rows)
	if err != nil {
		return store.AnomalyDetail{}, err
	}
	return anomaly, rows.Err()
}

func scanAnomaly(rows pgx.Rows) (analysis.Anomaly, error) {
	var (
		anomaly           analysis.Anomaly
		meterID           string
		basis, findings   []byte
		series            []byte
		correlatedEvent   []byte
		corroborating     []string
		explanationSource store.ExplanationSource
		explanationStatus store.ExplanationStatus
		status            store.AnomalyStatus
	)
	err := rows.Scan(&anomaly.ID, &anomaly.RunID, &meterID, &anomaly.WindowStart, &anomaly.WindowEnd, &anomaly.AffectedReadings,
		&anomaly.Type, &anomaly.Severity, &anomaly.Confidence, &basis, &anomaly.DeviationPercent,
		&anomaly.ActualKWh, &anomaly.BaselineKWh, &corroborating, &findings, &series, &correlatedEvent,
		&anomaly.Reason, &anomaly.RecommendedAction, &anomaly.DetectedBy,
		&explanationSource, &explanationStatus, &status)
	if err != nil {
		return analysis.Anomaly{}, fmt.Errorf("scan anomaly: %w", err)
	}

	anomaly.MeterCode = meterID
	anomaly.Corroborating = corroborating
	anomaly.ExplanationSource = explanationSource
	anomaly.ExplanationStatus = explanationStatus
	anomaly.Status = status
	if err := json.Unmarshal(basis, &anomaly.ConfidenceBasis); err != nil {
		return analysis.Anomaly{}, fmt.Errorf("decode confidence basis for anomaly %d: %w", anomaly.ID, err)
	}
	if err := json.Unmarshal(findings, &anomaly.Findings); err != nil {
		return analysis.Anomaly{}, fmt.Errorf("decode findings for anomaly %d: %w", anomaly.ID, err)
	}
	if err := json.Unmarshal(series, &anomaly.DeviationSeries); err != nil {
		return analysis.Anomaly{}, fmt.Errorf("decode deviation series for anomaly %d: %w", anomaly.ID, err)
	}
	if len(correlatedEvent) > 0 {
		var event analysis.EventSummary
		if err := json.Unmarshal(correlatedEvent, &event); err != nil {
			return analysis.Anomaly{}, fmt.Errorf("decode correlated event for anomaly %d: %w", anomaly.ID, err)
		}
		anomaly.CorrelatedEvent = &event
	}
	return anomaly, nil
}

func (db *DB) Run(ctx context.Context, runID int64) (store.Run, error) {
	var run store.Run
	err := db.pool.QueryRow(ctx, `
		SELECT id, started_at, finished_at, state, anomaly_count, window_start, window_end, error
		FROM analysis_runs
		WHERE id = $1`, runID).
		Scan(&run.ID, &run.StartedAt, &run.FinishedAt, &run.State, &run.AnomalyCount,
			&run.WindowStart, &run.WindowEnd, &run.Error)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Run{}, fmt.Errorf("run %d: %w", runID, store.ErrNotFound)
	}
	if err != nil {
		return store.Run{}, fmt.Errorf("run %d: %w", runID, err)
	}
	return run, nil
}

// DB satisfies the contract it was written against, at compile time.
var _ store.Store = (*DB)(nil)
