package analysis

import (
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

func eventAt(meter catalog.MeterCode, at time.Time, kind catalog.EventType, description string) catalog.OperationalEvent {
	return catalog.OperationalEvent{MeterCode: meter, Timestamp: at, Type: kind, Description: description}
}

func TestClassifyStepWithOperationalChangeIsExplainable(t *testing.T) {
	readings := synthetic("M-104", 14, flatThen(14, 10, 30, 44))
	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	Correlate([]catalog.OperationalEvent{
		eventAt("M-104", episodes[0].Start, catalog.EventTypeOperationalChange, "New production line activated"),
	}, &episodes[0], DefaultDetectorConfig())

	anomaly := Classify(episodes[0], DefaultDetectorConfig())

	if anomaly.Type != AnomalyExplainable {
		t.Errorf("Type = %q, want %q", anomaly.Type, AnomalyExplainable)
	}
	if anomaly.Severity != SeverityMedium {
		t.Errorf("Severity = %q, want %q", anomaly.Severity, SeverityMedium)
	}
}

func TestClassifyOutageWithScheduledOutageIsFalsePositive(t *testing.T) {
	readings := synthetic("M-106", 14, func(day, hour int) float64 {
		if day == 7 && hour < 12 {
			return 8
		}
		return 30
	})
	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	Correlate([]catalog.OperationalEvent{
		eventAt("M-106", episodes[0].Start, catalog.EventTypeScheduledOutage, "Scheduled maintenance outage for 12 hours"),
	}, &episodes[0], DefaultDetectorConfig())

	anomaly := Classify(episodes[0], DefaultDetectorConfig())

	if anomaly.Type != AnomalyFalsePositive {
		t.Errorf("Type = %q, want %q: the outage is known, so it is not a real anomaly",
			anomaly.Type, AnomalyFalsePositive)
	}
	if anomaly.Severity != SeverityLow {
		t.Errorf("Severity = %q, want %q", anomaly.Severity, SeverityLow)
	}
}

func TestClassifyLargeUnexplainedChangeIsRealAndHigh(t *testing.T) {
	readings := synthetic("M-109", 14, flatThen(14, 10, 30, 60))
	for i := 10 * 24; i < len(readings); i++ {
		readings[i].PowerFactor = 0.74
		readings[i].CurrentA = readings[i].ConsumptionKWh * 1000 / (220 * 0.93)
	}
	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	// The event row says no event was reported. It is evidence of absence, not a
	// cause, so it must not excuse the change.
	Correlate([]catalog.OperationalEvent{
		eventAt("M-109", episodes[0].Start, catalog.EventTypeUnknown, "No operational event reported"),
	}, &episodes[0], DefaultDetectorConfig())

	anomaly := Classify(episodes[0], DefaultDetectorConfig())

	if anomaly.Type != AnomalyReal {
		t.Errorf("Type = %q, want %q", anomaly.Type, AnomalyReal)
	}
	if anomaly.Severity != SeverityHigh {
		t.Errorf("Severity = %q, want %q", anomaly.Severity, SeverityHigh)
	}
	if anomaly.Confidence < 0.75 {
		t.Errorf("Confidence = %v, want at least 0.75", anomaly.Confidence)
	}
}

func TestClassifyFlatConsumptionWithCorruptReadingsIsDataQuality(t *testing.T) {
	readings := synthetic("M-112", 14, flatThen(14, 10, 30, 30))
	for _, hour := range []int{0, 3, 6, 9, 12, 15} {
		readings[11*24+hour].VoltageV = 203
	}
	episodes := FindDataQualityEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	Correlate([]catalog.OperationalEvent{
		eventAt("M-112", episodes[0].Start, catalog.EventTypeDataQuality, "Intermittent readings and abnormal electrical jumps"),
	}, &episodes[0], DefaultDetectorConfig())

	anomaly := Classify(episodes[0], DefaultDetectorConfig())

	if anomaly.Type != AnomalyDataQuality {
		t.Errorf("Type = %q, want %q: consumption never moved, so the meter is what is wrong",
			anomaly.Type, AnomalyDataQuality)
	}
	if anomaly.Severity != SeverityMedium {
		t.Errorf("Severity = %q, want %q for six corrupt readings", anomaly.Severity, SeverityMedium)
	}
}

func TestClassifyLargeDataQualityFindingIsHigh(t *testing.T) {
	readings := synthetic("M-BIG", 14, flatThen(14, 10, 30, 30))
	for hour := 0; hour < 24; hour += 3 {
		readings[11*24+hour].VoltageV = 203
	}
	episodes := FindDataQualityEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}

	anomaly := Classify(episodes[0], DefaultDetectorConfig())

	if anomaly.Severity != SeverityHigh {
		t.Errorf("Severity = %q, want %q for eight corrupt readings", anomaly.Severity, SeverityHigh)
	}
}

func TestClassifyDataQualityFindingWithMovingConsumptionStaysReal(t *testing.T) {
	// A load change that happens to leave some readings inconsistent is still a
	// real change: the data-quality reading only applies when consumption stayed
	// where it was.
	readings := synthetic("M-BOTH", 14, flatThen(14, 10, 30, 90))
	for i := 10 * 24; i < len(readings); i++ {
		readings[i].VoltageV = 203
	}
	episodes := FindDataQualityEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) == 0 {
		t.Fatal("got no data-quality episodes, want the corrupt voltage to be found")
	}

	anomaly := Classify(episodes[0], DefaultDetectorConfig())

	if anomaly.Type == AnomalyDataQuality {
		t.Errorf("Type = %q, want a real change: consumption moved %v", anomaly.Type, episodes[0].Deviation)
	}
}

func TestConfidenceIsAWeightedGeometricMeanOfItsTerms(t *testing.T) {
	terms := Evidence{
		Deviation:     0.85,
		EventMatch:    1.0,
		Corroboration: 0.5,
		Persistence:   1.0,
	}
	cfg := DefaultDetectorConfig()

	got := scoreConfidence(terms, cfg)

	want := 0.85
	want = pow(want, cfg.Weights.Deviation) * 1
	_ = want
	expected := pow(0.85, 0.4) * pow(1.0, 0.2) * pow(0.5, 0.25) * pow(1.0, 0.15)
	if diff := got - expected; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("Confidence = %v, want %v", got, expected)
	}
}

func TestConfidenceStaysInUnitRange(t *testing.T) {
	cfg := DefaultDetectorConfig()
	cases := []Evidence{
		{Deviation: 0, EventMatch: 0, Corroboration: 0, Persistence: 0},
		{Deviation: 1, EventMatch: 1, Corroboration: 1, Persistence: 1},
		{Deviation: 0.5, EventMatch: 0.3, Corroboration: 0.25, Persistence: 0.08},
	}
	for _, terms := range cases {
		got := scoreConfidence(terms, cfg)
		if got < 0 || got > 1 {
			t.Errorf("Confidence(%+v) = %v, want within [0,1]", terms, got)
		}
	}
}

func TestConfidenceNeverRisesWhenEvidenceFalls(t *testing.T) {
	cfg := DefaultDetectorConfig()
	strong := Evidence{Deviation: 0.9, EventMatch: 1, Corroboration: 1, Persistence: 1}
	weak := strong
	weak.Corroboration = 0.25

	if scoreConfidence(weak, cfg) >= scoreConfidence(strong, cfg) {
		t.Error("dropping corroboration did not lower the score")
	}
}

func TestEvidenceTermsAreExposed(t *testing.T) {
	readings := synthetic("M-E", 14, flatThen(14, 10, 30, 44))
	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	Correlate([]catalog.OperationalEvent{
		eventAt("M-E", episodes[0].Start, catalog.EventTypeOperationalChange, "New production line activated"),
	}, &episodes[0], DefaultDetectorConfig())

	anomaly := Classify(episodes[0], DefaultDetectorConfig())

	if anomaly.ConfidenceBasis.Deviation <= 0 {
		t.Error("the deviation term is not exposed, so the score cannot be traced")
	}
	if anomaly.ConfidenceBasis.EventMatch != 1 {
		t.Errorf("EventMatch = %v, want 1: a known event fully accounts for the episode",
			anomaly.ConfidenceBasis.EventMatch)
	}
	if anomaly.ConfidenceBasis.Persistence != 1 {
		t.Errorf("Persistence = %v, want 1 for a four-day episode", anomaly.ConfidenceBasis.Persistence)
	}
}
