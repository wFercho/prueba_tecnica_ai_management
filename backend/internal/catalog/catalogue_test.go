package catalog

import (
	"strings"
	"testing"
	"time"
)

// The delivered dataset has readings and events but no catalogue: a meter exists
// because something reported a reading for it. These tests fix what the API shows for
// such a meter, because a synthesised name that looks like real data is worse than
// one that admits where it came from.

func TestTheCatalogueIsDerivedFromTheMetersThatReported(t *testing.T) {
	readings := []Reading{
		{MeterCode: "M-112", Timestamp: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)},
		{MeterCode: "M-101", Timestamp: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)},
		{MeterCode: "M-101", Timestamp: time.Date(2026, 9, 1, 1, 0, 0, 0, time.UTC)},
	}

	meters := FromReadings(readings)

	if len(meters) != 2 {
		t.Fatalf("got %d meters, want one per meter that reported", len(meters))
	}
	// Ordered by code, so the catalogue does not depend on which reading arrived first.
	if meters[0].Code != "M-101" || meters[1].Code != "M-112" {
		t.Errorf("meters = %q, %q, want them ordered by code", meters[0].Code, meters[1].Code)
	}
}

func TestADerivedMeterIsNamedAfterItsCodeAndSaysWhereItCameFrom(t *testing.T) {
	meters := FromReadings([]Reading{{MeterCode: "M-109"}})

	if meters[0].Name != "Medidor M-109" {
		t.Errorf("name = %q, want the code it was derived from", meters[0].Name)
	}
	// The delivered data carries no locations, and inventing a plant name would put a
	// claim on screen that no data supports.
	if meters[0].Location != "" {
		t.Errorf("location = %q, want empty rather than invented", meters[0].Location)
	}
}

func TestDerivingFromNothingProducesNoMeters(t *testing.T) {
	if meters := FromReadings(nil); len(meters) != 0 {
		t.Errorf("got %d meters from no readings", len(meters))
	}
}

func TestTheMeterCatalogueCoversTheDeliveredDataset(t *testing.T) {
	// The delivered file is the one thing here that cannot drift, and the dashboard's
	// meter count is read off it.
	readings := []Reading{}
	for _, code := range []MeterCode{"M-101", "M-102", "M-103", "M-104", "M-105", "M-106",
		"M-107", "M-108", "M-109", "M-110", "M-111", "M-112"} {
		readings = append(readings, Reading{MeterCode: code})
	}

	meters := FromReadings(readings)
	if len(meters) != 12 {
		t.Errorf("got %d meters, want the delivered 12", len(meters))
	}
}

func TestAReadingWithNoMeterIsNotSilentlyDropped(t *testing.T) {
	// A row with a blank meter code cannot be stored — the column references the
	// catalogue — so the importer is told rather than the row vanishing.
	if err := Validate([]Reading{{MeterCode: "", Timestamp: time.Now()}}); err == nil {
		t.Error("a reading with no meter code was accepted")
	}
}

func TestValidatingReadingsCatchesTheMistakesACSVCanCarry(t *testing.T) {
	good := Reading{
		MeterCode: "M-101", Timestamp: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		ConsumptionKWh: 23.5, VoltageV: 221.9, CurrentA: 101.28, PowerFactor: 0.954,
		IngestedStatus: "OK",
	}
	if err := Validate([]Reading{good}); err != nil {
		t.Errorf("a well-formed reading was refused: %v", err)
	}

	// Each case drops or corrupts one field, so a failure names the field to fix.
	cases := map[string]Reading{
		"meter_id":     {Timestamp: good.Timestamp, IngestedStatus: "OK"},
		"timestamp":    {MeterCode: "M-101", IngestedStatus: "OK"},
		"status":       {MeterCode: "M-101", Timestamp: good.Timestamp},
		"power_factor": {MeterCode: "M-101", Timestamp: good.Timestamp, IngestedStatus: "OK", PowerFactor: 1.4},
		"consumption":  {MeterCode: "M-101", Timestamp: good.Timestamp, IngestedStatus: "OK", ConsumptionKWh: -5},
	}
	for field, reading := range cases {
		if err := Validate([]Reading{reading}); err == nil {
			t.Errorf("accepted a reading with an unusable %s", field)
		} else if !strings.Contains(err.Error(), field) {
			t.Errorf("the error for a bad %s does not name the field: %v", field, err)
		}
	}
}

func TestValidatingSaysWhichRowAndWhichField(t *testing.T) {
	// A CSV with a mistake in row 2,000 needs an answer an operator can act on.
	err := Validate([]Reading{
		{MeterCode: "M-101", Timestamp: time.Now().UTC(), IngestedStatus: "OK"},
		{MeterCode: "M-101", Timestamp: time.Now().UTC(), IngestedStatus: "OK", PowerFactor: 3},
	})
	if err == nil {
		t.Fatal("accepted a reading with an impossible power factor")
	}
	for _, want := range []string{"2", "power_factor"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %s: %s", want, err)
		}
	}
}
