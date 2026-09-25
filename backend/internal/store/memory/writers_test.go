package memory

import (
	"context"
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

// Importing the same window twice must leave one copy of it, not two. The seeder is
// run by hand, on boot in a development script, and by anyone following the README,
// so re-running it has to be safe rather than quietly doubling the data the detector
// reasons about.

func TestAppendingReadingsTwiceKeepsOneRowPerHour(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()
	batch := []catalog.Reading{
		{
			MeterCode:      "M-1",
			Timestamp:      time.Date(2026, 9, 20, 3, 0, 0, 0, time.UTC),
			ConsumptionKWh: 42,
			VoltageV:       221,
			CurrentA:       110,
			PowerFactor:    0.94,
			IngestedStatus: "OK",
		},
	}

	if err := s.AppendReadings(ctx, batch); err != nil {
		t.Fatalf("AppendReadings: %v", err)
	}
	if err := s.AppendReadings(ctx, batch); err != nil {
		t.Fatalf("AppendReadings again: %v", err)
	}

	readings, err := s.Readings(ctx, "M-1", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Readings: %v", err)
	}
	seen := 0
	for _, reading := range readings {
		if reading.Timestamp.Equal(batch[0].Timestamp) {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("the hour appears %d times, want 1", seen)
	}
}

func TestAppendingAReadingReplacesTheHourItAlreadyHolds(t *testing.T) {
	// A re-import is also how a corrected reading arrives, so the second write has
	// to win rather than be ignored.
	s := seeded(t)
	ctx := context.Background()
	at := time.Date(2026, 9, 20, 4, 0, 0, 0, time.UTC)
	row := catalog.Reading{
		MeterCode: "M-1", Timestamp: at, ConsumptionKWh: 10,
		VoltageV: 220, CurrentA: 50, PowerFactor: 0.9, IngestedStatus: "OK",
	}
	if err := s.AppendReadings(ctx, []catalog.Reading{row}); err != nil {
		t.Fatalf("AppendReadings: %v", err)
	}
	row.ConsumptionKWh = 99
	row.IngestedStatus = "LATE"
	if err := s.AppendReadings(ctx, []catalog.Reading{row}); err != nil {
		t.Fatalf("AppendReadings again: %v", err)
	}

	readings, _ := s.Readings(ctx, "M-1", time.Time{}, time.Time{})
	for _, reading := range readings {
		if reading.Timestamp.Equal(at) {
			if reading.ConsumptionKWh != 99 {
				t.Errorf("consumption = %v, want the corrected 99", reading.ConsumptionKWh)
			}
			if reading.IngestedStatus != "LATE" {
				t.Errorf("status = %q, want the corrected LATE", reading.IngestedStatus)
			}
			return
		}
	}
	t.Fatal("the hour is missing altogether")
}

func TestAppendingEventsTwiceKeepsOneRowPerEvent(t *testing.T) {
	s := seeded(t)
	ctx := context.Background()
	batch := []catalog.OperationalEvent{{
		MeterCode:   "M-1",
		Timestamp:   time.Date(2026, 9, 20, 5, 0, 0, 0, time.UTC),
		Type:        catalog.EventTypeOperationalChange,
		Description: "New line started",
	}}

	if err := s.AppendEvents(ctx, batch); err != nil {
		t.Fatalf("AppendEvents: %v", err)
	}
	if err := s.AppendEvents(ctx, batch); err != nil {
		t.Fatalf("AppendEvents again: %v", err)
	}

	events, _ := s.Events(ctx)
	seen := 0
	for _, event := range events {
		if event.Description == "New line started" {
			seen++
		}
	}
	if seen != 1 {
		t.Errorf("the event appears %d times, want 1", seen)
	}
}

func TestWritingAnUnknownMeterIsRefusedRatherThanStored(t *testing.T) {
	// Readings reference a meter, so an importer that renames a meter between runs
	// must be told rather than allowed to write rows nothing will ever read.
	s := New()
	if err := s.UpsertMeters(context.Background(), []catalog.Meter{{Code: "M-1", Name: "One"}}); err != nil {
		t.Fatalf("UpsertMeters: %v", err)
	}
	ctx := context.Background()

	err := s.AppendReadings(ctx, []catalog.Reading{{
		MeterCode: "M-404", Timestamp: time.Now().UTC(), IngestedStatus: "OK",
	}})
	if err == nil {
		t.Error("a reading for an unknown meter was accepted")
	}
	if err := s.AppendEvents(ctx, []catalog.OperationalEvent{{
		MeterCode: "M-404", Timestamp: time.Now().UTC(), Type: catalog.EventTypeUnknown,
	}}); err == nil {
		t.Error("an event for an unknown meter was accepted")
	}
}

func TestAppendingNothingIsNotAFailure(t *testing.T) {
	// An importer that finds no rows should report success, not an error the operator
	// has to learn to ignore.
	s := seeded(t)
	if err := s.AppendReadings(context.Background(), nil); err != nil {
		t.Errorf("AppendReadings(nil) = %v, want no failure", err)
	}
	if err := s.AppendEvents(context.Background(), []catalog.OperationalEvent{}); err != nil {
		t.Errorf("AppendEvents(empty) = %v, want no failure", err)
	}
}

var _ store.Store = (*Store)(nil)
