package ingest

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// layouts are the timestamp formats the delivered files use, tried in order. The
// readings carry seconds and the events do not.
var layouts = []string{
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	time.RFC3339,
}

func parseTimestamp(raw string) (time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %q", raw)
}

func parseFloat(raw, field string) (float64, error) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, fmt.Errorf("field %s: %w", field, err)
	}
	return parsed, nil
}

// ReadReadings decodes readings.csv. The ingested status is carried through
// verbatim: it is the source's claim, not a verdict, and all 4,032 delivered rows
// say OK, including the corrupt ones.
func ReadReadings(r io.Reader) ([]catalog.Reading, error) {
	rows, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read readings: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("readings: file is empty")
	}

	var readings []catalog.Reading
	for i, row := range rows[1:] {
		if len(row) < 7 {
			return nil, fmt.Errorf("readings row %d: got %d columns, want 7", i+2, len(row))
		}
		timestamp, err := parseTimestamp(row[1])
		if err != nil {
			return nil, fmt.Errorf("readings row %d: %w", i+2, err)
		}
		consumption, err := parseFloat(row[2], "consumption_kwh")
		if err != nil {
			return nil, fmt.Errorf("readings row %d: %w", i+2, err)
		}
		voltage, err := parseFloat(row[3], "voltage_v")
		if err != nil {
			return nil, fmt.Errorf("readings row %d: %w", i+2, err)
		}
		current, err := parseFloat(row[4], "current_a")
		if err != nil {
			return nil, fmt.Errorf("readings row %d: %w", i+2, err)
		}
		powerFactor, err := parseFloat(row[5], "power_factor")
		if err != nil {
			return nil, fmt.Errorf("readings row %d: %w", i+2, err)
		}
		readings = append(readings, catalog.Reading{
			MeterCode:      catalog.MeterCode(strings.TrimSpace(row[0])),
			Timestamp:      timestamp,
			ConsumptionKWh: consumption,
			VoltageV:       voltage,
			CurrentA:       current,
			PowerFactor:    powerFactor,
			IngestedStatus: strings.TrimSpace(row[6]),
		})
	}
	return readings, nil
}

// ReadEvents decodes events.csv.
func ReadEvents(r io.Reader) ([]catalog.OperationalEvent, error) {
	rows, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("events: file is empty")
	}

	var events []catalog.OperationalEvent
	for i, row := range rows[1:] {
		if len(row) < 4 {
			return nil, fmt.Errorf("events row %d: got %d columns, want 4", i+2, len(row))
		}
		timestamp, err := parseTimestamp(row[1])
		if err != nil {
			return nil, fmt.Errorf("events row %d: %w", i+2, err)
		}
		events = append(events, catalog.OperationalEvent{
			MeterCode:   catalog.MeterCode(strings.TrimSpace(row[0])),
			Timestamp:   timestamp,
			Type:        catalog.EventType(strings.TrimSpace(row[2])),
			Description: strings.TrimSpace(row[3]),
		})
	}
	return events, nil
}
