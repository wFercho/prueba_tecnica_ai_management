package ingest

import (
	"strings"
	"testing"
)

func TestReadReadingsCarriesIngestedStatusVerbatim(t *testing.T) {
	// Ingested status is the source's claim, not a verdict. Preserving it
	// verbatim is what keeps it evidence about the ingest pipeline.
	file := "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status\n" +
		"M-101,2026-09-01 00:00:00,23.5,221.9,101.28,0.954,OK\n" +
		"M-112,2026-09-13 00:00:00,22.57,241.2,164.8,0.980,DEGRADED\n"

	readings, err := ReadReadings(strings.NewReader(file))
	if err != nil {
		t.Fatalf("ReadReadings: %v", err)
	}
	if len(readings) != 2 {
		t.Fatalf("got %d readings, want 2", len(readings))
	}
	if readings[0].IngestedStatus != "OK" {
		t.Errorf("status = %q, want OK", readings[0].IngestedStatus)
	}
	if readings[1].IngestedStatus != "DEGRADED" {
		t.Errorf("status = %q, want DEGRADED", readings[1].IngestedStatus)
	}
}

func TestReadReadingsRejectsShortRow(t *testing.T) {
	file := "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status\n" +
		"M-101,2026-09-01 00:00:00,23.5\n"

	if _, err := ReadReadings(strings.NewReader(file)); err == nil {
		t.Fatal("got no error, want one for a truncated row")
	}
}

func TestReadReadingsRejectsUnparseableNumber(t *testing.T) {
	file := "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status\n" +
		"M-101,2026-09-01 00:00:00,nineteen,221.9,101.28,0.954,OK\n"

	if _, err := ReadReadings(strings.NewReader(file)); err == nil {
		t.Fatal("got no error, want one for an unparseable consumption")
	}
}

func TestReadReadingsAcceptsBothTimestampShapes(t *testing.T) {
	file := "meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status\n" +
		"M-101,2026-09-01 00:00:00,23.5,221.9,101.28,0.954,OK\n" +
		"M-101,2026-09-01 01:00,22.1,221.9,101.28,0.954,OK\n"

	readings, err := ReadReadings(strings.NewReader(file))
	if err != nil {
		t.Fatalf("ReadReadings: %v", err)
	}
	if readings[0].Timestamp.Hour() != 0 || readings[1].Timestamp.Hour() != 1 {
		t.Errorf("hours = %d, %d; want 0, 1", readings[0].Timestamp.Hour(), readings[1].Timestamp.Hour())
	}
}

func TestReadEvents(t *testing.T) {
	file := "meter_id,event_timestamp,event_type,description\n" +
		"M-104,2026-09-11 00:00,OPERATIONAL_CHANGE,New production line activated\n"

	events, err := ReadEvents(strings.NewReader(file))
	if err != nil {
		t.Fatalf("ReadEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if events[0].Type != "OPERATIONAL_CHANGE" {
		t.Errorf("type = %q, want OPERATIONAL_CHANGE", events[0].Type)
	}
	if !events[0].Explains() {
		t.Error("an operational change should explain an episode")
	}
}
