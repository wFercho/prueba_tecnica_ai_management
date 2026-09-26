package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// ReplaceDataset resets derived runs and source rows as one transaction. Users
// and sessions are never involved, and a failed CSV write rolls the reset back.
func (db *DB) ReplaceDataset(ctx context.Context, meters []catalog.Meter, readings []catalog.Reading, events []catalog.OperationalEvent) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin data import: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, query := range []string{
		`DELETE FROM analysis_runs`, // cascades to anomalies
		`DELETE FROM events`, `DELETE FROM readings`, `DELETE FROM meters`,
	} {
		if _, err := tx.Exec(ctx, query); err != nil {
			return fmt.Errorf("clear old demo data: %w", err)
		}
	}
	for _, meter := range meters {
		if _, err := tx.Exec(ctx, `INSERT INTO meters (meter_id, name, location) VALUES ($1, $2, $3)`, meter.Code, meter.Name, meter.Location); err != nil {
			return fmt.Errorf("import meter %s: %w", meter.Code, err)
		}
	}
	if len(readings) > 0 {
		_, err = tx.CopyFrom(ctx, pgx.Identifier{"readings"},
			[]string{"meter_id", "timestamp", "consumption_kwh", "voltage_v", "current_a", "power_factor", "status"},
			pgx.CopyFromSlice(len(readings), func(i int) ([]any, error) {
				r := readings[i]
				return []any{r.MeterCode, r.Timestamp, r.ConsumptionKWh, r.VoltageV, r.CurrentA, r.PowerFactor, r.IngestedStatus}, nil
			}))
		if err != nil {
			return fmt.Errorf("import readings: %w", err)
		}
	}
	for _, event := range events {
		if _, err := tx.Exec(ctx, `INSERT INTO events (meter_id, timestamp, type, description) VALUES ($1, $2, $3, $4)`,
			event.MeterCode, event.Timestamp, event.Type, event.Description); err != nil {
			return fmt.Errorf("import event: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit data import: %w", err)
	}
	return nil
}
