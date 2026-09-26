package analysis_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/ingest"
)

// These tests run the detector over the delivered 14-day dataset and assert the
// outcomes the assignment states. They are the gate on every constant in
// DetectorConfig: the obvious way to silence a false positive is to loosen the
// threshold that fired it, and the control set is what catches that.

const (
	// datasetPath is the delivered data, the only dataset the assignment fixes.
	datasetPath = "../../../data"
)

// controlMeters are the eight meters with no operational event. Their largest
// day-over-day swing is 4.63%, which is what makes them a usable control: any
// anomaly on one of them is a false positive, whatever its type.
var controlMeters = []catalog.MeterCode{
	"M-101", "M-102", "M-103", "M-105", "M-107", "M-108", "M-110", "M-111",
}

func loadDataset(t *testing.T) ([]catalog.Reading, []catalog.OperationalEvent) {
	t.Helper()
	readingsFile, err := os.Open(filepath.Join(datasetPath, "readings.csv"))
	if err != nil {
		t.Fatalf("open readings.csv: %v", err)
	}
	defer readingsFile.Close()
	readings, err := ingest.ReadReadings(readingsFile)
	if err != nil {
		t.Fatalf("read readings: %v", err)
	}

	eventsFile, err := os.Open(filepath.Join(datasetPath, "events.csv"))
	if err != nil {
		t.Fatalf("open events.csv: %v", err)
	}
	defer eventsFile.Close()
	events, err := ingest.ReadEvents(eventsFile)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	return readings, events
}

// analyse runs the whole detector over the delivered dataset, through the same
// entry point the API uses.
func analyse(t *testing.T) []analysis.Anomaly {
	t.Helper()
	readings, events := loadDataset(t)
	return analysis.Analyze(readings, events, analysis.DefaultDetectorConfig())
}

func TestDetectorProducesExactlyTheFourExpectedAnomalies(t *testing.T) {
	anomalies := analyse(t)

	type expectation struct {
		meter    string
		kind     analysis.AnomalyType
		severity analysis.Severity
		readings int
	}
	want := map[string]expectation{
		"M-104": {"M-104", analysis.AnomalyExplainable, analysis.SeverityMedium, 96},
		"M-106": {"M-106", analysis.AnomalyFalsePositive, analysis.SeverityLow, 12},
		"M-109": {"M-109", analysis.AnomalyReal, analysis.SeverityHigh, 58},
		"M-112": {"M-112", analysis.AnomalyDataQuality, analysis.SeverityHigh, 16},
	}

	if len(anomalies) != len(want) {
		var got []string
		for _, anomaly := range anomalies {
			got = append(got, anomaly.MeterCode+":"+string(anomaly.Type))
		}
		t.Fatalf("got %d anomalies (%v), want %d", len(anomalies), got, len(want))
	}

	for _, anomaly := range anomalies {
		expected, ok := want[anomaly.MeterCode]
		if !ok {
			t.Errorf("unexpected anomaly on %s: %s %s", anomaly.MeterCode, anomaly.Type, anomaly.Severity)
			continue
		}
		if anomaly.Type != expected.kind {
			t.Errorf("%s type = %s, want %s", anomaly.MeterCode, anomaly.Type, expected.kind)
		}
		if anomaly.Severity != expected.severity {
			t.Errorf("%s severity = %s, want %s", anomaly.MeterCode, anomaly.Severity, expected.severity)
		}
		if anomaly.AffectedReadings != expected.readings {
			t.Errorf("%s affected readings = %d, want %d", anomaly.MeterCode, anomaly.AffectedReadings, expected.readings)
		}
	}
}

func TestControlSetProducesNoAnomalies(t *testing.T) {
	anomalies := analyse(t)

	for _, anomaly := range anomalies {
		for _, control := range controlMeters {
			if catalog.MeterCode(anomaly.MeterCode) == control {
				t.Errorf("control meter %s produced a %s anomaly over %d readings (%s..%s, deviation %v%%)",
					control, anomaly.Type, anomaly.AffectedReadings,
					anomaly.WindowStart, anomaly.WindowEnd, anomaly.DeviationPercent)
			}
		}
	}
}

func TestMostUrgentAnomalyIsM109(t *testing.T) {
	anomalies := analyse(t)

	var ranked []analysis.Anomaly
	for _, anomaly := range anomalies {
		if anomaly.Severity == analysis.SeverityHigh {
			ranked = append(ranked, anomaly)
		}
	}
	if len(ranked) != 2 {
		t.Fatalf("got %d high-severity anomalies, want 2", len(ranked))
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Confidence > ranked[j].Confidence
	})

	if ranked[0].MeterCode != "M-109" {
		t.Errorf("most urgent anomaly is %s, want M-109", ranked[0].MeterCode)
	}
	if ranked[1].MeterCode != "M-112" {
		t.Errorf("second most urgent anomaly is %s, want M-112", ranked[1].MeterCode)
	}
}

func TestM106IsNotTreatedAsReal(t *testing.T) {
	anomalies := analyse(t)

	for _, anomaly := range anomalies {
		if anomaly.MeterCode != "M-106" {
			continue
		}
		if anomaly.Type == analysis.AnomalyReal {
			t.Error("M-106 was classified as a real anomaly; a scheduled outage explains it")
		}
		if anomaly.CorrelatedEvent == nil || !anomaly.CorrelatedEvent.Explains {
			t.Error("M-106 is not linked to the scheduled outage that explains it")
		}
		return
	}
	t.Fatal("M-106 produced no anomaly at all; the outage was examined and dismissed, not missed")
}

func TestM109CarriesEvidenceForItsExplanation(t *testing.T) {
	anomalies := analyse(t)

	for _, anomaly := range anomalies {
		if anomaly.MeterCode != "M-109" {
			continue
		}
		if anomaly.DeviationPercent < 100 {
			t.Errorf("M-109 deviation = %v%%, want more than 100%% above baseline", anomaly.DeviationPercent)
		}
		if len(anomaly.Corroborating) < 2 {
			t.Errorf("M-109 corroborating = %v, want the current and the power factor to have moved too",
				anomaly.Corroborating)
		}
		if anomaly.ConfidenceBasis.Deviation <= 0 || anomaly.ConfidenceBasis.Corroboration <= 0 {
			t.Errorf("M-109 evidence terms = %+v, want them populated", anomaly.ConfidenceBasis)
		}
		return
	}
	t.Fatal("M-109 produced no anomaly")
}

func TestM112IsReportedAsDataQualityWithItsCorruptReadings(t *testing.T) {
	anomalies := analyse(t)

	for _, anomaly := range anomalies {
		if anomaly.MeterCode != "M-112" {
			continue
		}
		if anomaly.Type != analysis.AnomalyDataQuality {
			t.Fatalf("M-112 type = %s, want DATA_QUALITY", anomaly.Type)
		}
		if abs(anomaly.DeviationPercent) > 15 {
			t.Errorf("M-112 deviation = %v%%, want consumption to have stayed flat", anomaly.DeviationPercent)
		}
		if len(anomaly.Findings) == 0 {
			t.Error("M-112 carries no data-quality findings, so the explanation has nothing to cite")
		}
		voltages := 0
		for _, finding := range anomaly.Findings {
			if finding.Variable == analysis.VariableVoltage {
				voltages++
			}
		}
		if voltages != anomaly.AffectedReadings {
			t.Errorf("M-112 has %d voltage findings across %d affected readings", voltages, anomaly.AffectedReadings)
		}
		return
	}
	t.Fatal("M-112 produced no anomaly")
}

func TestDetectorGeneralizesToRenamedMetersAndShiftedDates(t *testing.T) {
	readings, events := loadDataset(t)
	for i := range readings {
		readings[i].MeterCode = catalog.MeterCode("ALT-" + string(readings[i].MeterCode))
		readings[i].Timestamp = readings[i].Timestamp.Add(30 * 24 * time.Hour)
	}
	for i := range events {
		events[i].MeterCode = catalog.MeterCode("ALT-" + string(events[i].MeterCode))
		events[i].Timestamp = events[i].Timestamp.Add(30 * 24 * time.Hour)
	}
	want := map[string]analysis.AnomalyType{
		"ALT-M-104": analysis.AnomalyExplainable,
		"ALT-M-106": analysis.AnomalyFalsePositive,
		"ALT-M-109": analysis.AnomalyReal,
		"ALT-M-112": analysis.AnomalyDataQuality,
	}
	got := analysis.Analyze(readings, events, analysis.DefaultDetectorConfig())
	if len(got) != 4 {
		t.Fatalf("shifted dataset produced %d findings, want 4", len(got))
	}
	for _, finding := range got {
		if finding.Type != want[finding.MeterCode] || !finding.Anomaly {
			t.Errorf("unexpected finding %+v", finding)
		}
	}
}

func TestDataQualityNeedsElectricalEvidenceNotTheContextLabel(t *testing.T) {
	readings, events := loadDataset(t)
	filtered := events[:0]
	for _, event := range events {
		if event.MeterCode != "M-112" {
			filtered = append(filtered, event)
		}
	}
	for _, finding := range analysis.Analyze(readings, filtered, analysis.DefaultDetectorConfig()) {
		if finding.MeterCode == "M-112" {
			if finding.Type != analysis.AnomalyDataQuality || finding.AffectedReadings != 16 {
				t.Fatalf("without the report: %+v", finding)
			}
			return
		}
	}
	t.Fatal("physical inconsistencies went unnoticed without the quality report")
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
