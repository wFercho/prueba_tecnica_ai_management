package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/ingest"
)

// These tests read the prose of the four anomalies the delivered dataset
// produces, because that prose is part of what the product promises. A template
// that reads well against a synthetic fixture and cites nothing real is still a
// template that reads well against nothing.

// delivered runs the whole detector over data/ and returns its anomalies.
func delivered(t *testing.T) []Anomaly {
	t.Helper()
	readingsFile, err := os.Open(filepath.Join("../../../data", "readings.csv"))
	if err != nil {
		t.Fatalf("open readings.csv: %v", err)
	}
	defer readingsFile.Close()
	readings, err := ingest.ReadReadings(readingsFile)
	if err != nil {
		t.Fatalf("read readings: %v", err)
	}

	eventsFile, err := os.Open(filepath.Join("../../../data", "events.csv"))
	if err != nil {
		t.Fatalf("open events.csv: %v", err)
	}
	defer eventsFile.Close()
	events, err := ingest.ReadEvents(eventsFile)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	return Analyze(readings, events, DefaultDetectorConfig())
}

func theAnomaly(t *testing.T, meter string) Anomaly {
	t.Helper()
	for _, anomaly := range delivered(t) {
		if anomaly.MeterCode == meter {
			return anomaly
		}
	}
	t.Fatalf("%s produced no anomaly", meter)
	return Anomaly{}
}

func TestRealAnomalyCitesItsNumbersAndSaysItIsUnexplained(t *testing.T) {
	anomaly := theAnomaly(t, "M-109")

	for _, want := range []string{"110.0%", "58", "M-109", "2026-09-12 14:00"} {
		if !strings.Contains(anomaly.Reason, want) {
			t.Errorf("Reason = %q, want it to cite %q", anomaly.Reason, want)
		}
	}
	// M-109's event row says no event was reported. That is evidence of absence,
	// so the prose must say nothing on record accounts for the change rather than
	// reach for a likel-sounding cause.
	if !strings.Contains(anomaly.Reason, "reported no operational event") {
		t.Errorf("Reason = %q, want it to say the reported absence of an event", anomaly.Reason)
	}
	if strings.Contains(anomaly.Reason, "no reported operational event") {
		t.Errorf("Reason = %q, want it to distinguish nobody reporting from nothing being reported", anomaly.Reason)
	}
	if !strings.Contains(strings.ToLower(anomaly.RecommendedAction), "investigate") {
		t.Errorf("RecommendedAction = %q, want it to ask for an investigation", anomaly.RecommendedAction)
	}
}

func TestExplainableAnomalyNamesTheEventAndAsksForConfirmation(t *testing.T) {
	anomaly := theAnomaly(t, "M-104")

	for _, want := range []string{"New production line activated", "OPERATIONAL_CHANGE", "46.6%", "96"} {
		if !strings.Contains(anomaly.Reason, want) {
			t.Errorf("Reason = %q, want it to cite %q", anomaly.Reason, want)
		}
	}
	if !strings.Contains(strings.ToLower(anomaly.RecommendedAction), "confirm") {
		t.Errorf("RecommendedAction = %q, want confirmation rather than escalation", anomaly.RecommendedAction)
	}
}

func TestFalsePositiveSaysThereIsNothingToDo(t *testing.T) {
	anomaly := theAnomaly(t, "M-106")

	if !strings.Contains(anomaly.Reason, "Scheduled maintenance outage for 12 hours") {
		t.Errorf("Reason = %q, want it to name the outage", anomaly.Reason)
	}
	if !strings.Contains(anomaly.Reason, "79.8%") {
		t.Errorf("Reason = %q, want it to quantify the drop it dismissed", anomaly.Reason)
	}
	// §11 shows the action for this row as "No escalar", and §18 awards marks for
	// not treating it as real.
	if !strings.Contains(anomaly.RecommendedAction, "No escalar") {
		t.Errorf("RecommendedAction = %q, want the spec's 'No escalar'", anomaly.RecommendedAction)
	}
}

func TestDataQualityAnomalyCitesTheOffendingReadings(t *testing.T) {
	anomaly := theAnomaly(t, "M-112")

	for _, want := range []string{"voltage", "241", "16", "0.3%"} {
		if !strings.Contains(anomaly.Reason, want) {
			t.Errorf("Reason = %q, want it to cite %q", anomaly.Reason, want)
		}
	}
	if !strings.Contains(strings.ToLower(anomaly.RecommendedAction), "meter") {
		t.Errorf("RecommendedAction = %q, want it to point at the meter rather than the load", anomaly.RecommendedAction)
	}
}

func TestProseIsSentencesRatherThanRunOnText(t *testing.T) {
	// A missing space after a full stop is the kind of defect that survives every
	// assertion about content and is still the first thing a reader sees.
	for _, anomaly := range delivered(t) {
		for _, text := range []string{anomaly.Reason, anomaly.RecommendedAction} {
			if strings.Contains(text, ".N") || strings.Contains(text, `." `) {
				t.Errorf("%s prose has a run-on: %q", anomaly.MeterCode, text)
			}
			if strings.Contains(text, "power_factor") || strings.Contains(text, "energy_balance") {
				t.Errorf("%s prose leaks a variable name instead of a word: %q", anomaly.MeterCode, text)
			}
			if strings.Contains(text, "  ") {
				t.Errorf("%s prose has a double space: %q", anomaly.MeterCode, text)
			}
			if !strings.HasSuffix(text, ".") {
				t.Errorf("%s prose does not end in a full stop: %q", anomaly.MeterCode, text)
			}
		}
		if strings.HasPrefix(anomaly.Reason, "Between from") {
			t.Errorf("%s prose says 'Between from': %q", anomaly.MeterCode, anomaly.Reason)
		}
	}
}

func TestEveryAnomalyCarriesProseBeforeNarration(t *testing.T) {
	// A row is persisted with its deterministic explanation before anything
	// touches the network, so this must hold for every classification and not
	// just the four the dataset happens to contain (ADR-0006).
	cfg := DefaultDetectorConfig()
	classified := []Anomaly{
		Classify(Episode{MeterCode: "M", Kind: EpisodeConsumption, Start: someTime(), End: someTime(),
			Deviation: 1.1, ActualKWh: 300, BaselineKWh: 140, Corroboration: 0.5,
			Readings: []catalog.Reading{{ConsumptionKWh: 300}}}, cfg),
		Classify(Episode{MeterCode: "M", Kind: EpisodeDataQuality, Start: someTime(), End: someTime(),
			Deviation: 0.003, Corroboration: 1,
			Findings: []DataQualityFinding{{Variable: VariableVoltage, Value: 203, Expected: 220, RelativeDeviation: 0.08}},
			Readings: []catalog.Reading{{ConsumptionKWh: 30}}}, cfg),
		Classify(Episode{MeterCode: "M", Kind: EpisodeConsumption, Start: someTime(), End: someTime(),
			Deviation: -0.8, ActualKWh: 60, BaselineKWh: 300, Corroboration: 0.5,
			CorrelatedEvents: []catalog.OperationalEvent{{Type: catalog.EventTypeScheduledOutage, Description: "Outage"}},
			Readings:         []catalog.Reading{{ConsumptionKWh: 60}}}, cfg),
		Classify(Episode{MeterCode: "M", Kind: EpisodeConsumption, Start: someTime(), End: someTime(),
			Deviation: 0.46, ActualKWh: 420, BaselineKWh: 288, Corroboration: 0.5,
			CorrelatedEvents: []catalog.OperationalEvent{{Type: catalog.EventTypeOperationalChange, Description: "New line"}},
			Readings:         []catalog.Reading{{ConsumptionKWh: 420}}}, cfg),
	}

	for _, anomaly := range classified {
		if strings.TrimSpace(anomaly.Reason) == "" {
			t.Errorf("a %s anomaly has no reason", anomaly.Type)
		}
		if strings.TrimSpace(anomaly.RecommendedAction) == "" {
			t.Errorf("a %s anomaly has no recommended action", anomaly.Type)
		}
		if anomaly.ExplanationSource != catalog.SourceRules {
			t.Errorf("ExplanationSource = %q, want %q: the detector only ever writes rules prose",
				anomaly.ExplanationSource, catalog.SourceRules)
		}
		if anomaly.ExplanationStatus != catalog.ExplanationPending {
			t.Errorf("ExplanationStatus = %q, want %q", anomaly.ExplanationStatus, catalog.ExplanationPending)
		}
	}
}

func someTime() time.Time { return time.Date(2026, 9, 13, 3, 0, 0, 0, time.UTC) }
