package analysis

import (
	"slices"
	"testing"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

func TestAnalyzeReportsOneAnomalyPerEpisode(t *testing.T) {
	quiet := synthetic("M-B", 14, func(day, hour int) float64 {
		if day == 7 && hour < 12 {
			return 8
		}
		return 30
	})
	readings := slices.Concat(synthetic("M-A", 14, flatThen(14, 10, 30, 44)), quiet)
	events := []catalog.OperationalEvent{
		eventAt("M-A", readings[len(readings)-1].Timestamp, catalog.EventTypeOperationalChange, "New production line"),
	}

	anomalies := Analyze(readings, events, DefaultDetectorConfig())

	if len(anomalies) != 2 {
		var got []string
		for _, anomaly := range anomalies {
			got = append(got, anomaly.MeterCode+":"+string(anomaly.Type))
		}
		t.Fatalf("got %d anomalies (%v), want one per affected meter", len(anomalies), got)
	}
	if anomalies[0].MeterCode != "M-A" || anomalies[1].MeterCode != "M-B" {
		t.Errorf("got %s then %s, want meters in order", anomalies[0].MeterCode, anomalies[1].MeterCode)
	}
}

func TestAnalyzeLeavesQuietMetersAlone(t *testing.T) {
	readings := slices.Concat(
		synthetic("M-QUIET", 14, flatThen(14, 10, 30, 30)),
		synthetic("M-LOUD", 14, flatThen(14, 10, 30, 44)),
	)

	anomalies := Analyze(readings, nil, DefaultDetectorConfig())

	if len(anomalies) != 1 {
		t.Fatalf("got %d anomalies, want 1", len(anomalies))
	}
	if anomalies[0].MeterCode != "M-LOUD" {
		t.Errorf("anomaly is on %s, want M-LOUD", anomalies[0].MeterCode)
	}
}

func TestAnalyzeDoesNotReportAConsumptionEpisodeTwice(t *testing.T) {
	// A load that doubles moves consumption, current and power factor together.
	// That is one episode seen from three angles, and it must be one anomaly: the
	// consumption episode already carries the readings the load-variable findings
	// would have pointed at.
	readings := synthetic("M-DOUBLE", 14, flatThen(14, 10, 30, 60))
	for i := 10 * 24; i < len(readings); i++ {
		readings[i].PowerFactor = 0.74
		readings[i].CurrentA = readings[i].ConsumptionKWh * 1000 / (220 * 0.93)
	}

	anomalies := Analyze(readings, nil, DefaultDetectorConfig())

	if len(anomalies) != 1 {
		var got []string
		for _, anomaly := range anomalies {
			got = append(got, string(anomaly.Type))
		}
		t.Fatalf("got %d anomalies (%v), want the one episode", len(anomalies), got)
	}
	if anomalies[0].Type != AnomalyReal {
		t.Errorf("Type = %q, want %q", anomalies[0].Type, AnomalyReal)
	}
}

func TestAnalyzeIsStableInItsOrder(t *testing.T) {
	readings := slices.Concat(
		synthetic("M-2", 14, flatThen(14, 10, 30, 44)),
		synthetic("M-1", 14, flatThen(14, 10, 30, 44)),
		synthetic("M-3", 14, flatThen(14, 10, 30, 44)),
	)

	first := Analyze(readings, nil, DefaultDetectorConfig())
	second := Analyze(readings, nil, DefaultDetectorConfig())

	for i := range first {
		if first[i].MeterCode != second[i].MeterCode {
			t.Fatalf("run %d gave %s, run %d gave %s: the order is not stable",
				i, first[i].MeterCode, i, second[i].MeterCode)
		}
	}
	if first[0].MeterCode != "M-1" {
		t.Errorf("first anomaly is %s, want M-1", first[0].MeterCode)
	}
}

func TestDataQualityEpisodesSplitWhenTheGapExceedsThreeHours(t *testing.T) {
	readings := synthetic("M-SPLIT", 14, flatThen(14, 10, 30, 30))
	for _, hour := range []int{0, 1, 2, 10, 11, 12} {
		readings[10*24+hour].VoltageV = 241.2
	}
	episodes := FindDataQualityEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) != 2 {
		t.Fatalf("got %d episodes, want 2 separated by an 8-hour gap", len(episodes))
	}
	for _, episode := range episodes {
		if episode.ReadingCount() != 3 {
			t.Errorf("episode has %d readings, want 3 and no merging", episode.ReadingCount())
		}
	}
}

func TestDataQualityGroupsIntermittentFindingsWithinThreeHours(t *testing.T) {
	readings := synthetic("M-GROUP", 14, flatThen(14, 10, 30, 30))
	for _, hour := range []int{0, 3, 6, 9, 12, 15} {
		readings[10*24+hour].VoltageV = 241.2
	}
	episodes := FindDataQualityEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1 grouped across 3-hour gaps", len(episodes))
	}
	if got := episodes[0].ReadingCount(); got != 6 {
		t.Errorf("grouped %d affected readings, want 6 without the normal hours between them", got)
	}
}

func TestOneMeterKeepsConsumptionAndQualityEpisodesDistinct(t *testing.T) {
	readings := synthetic("M-BOTH", 14, flatThen(14, 10, 30, 60))
	for _, hour := range []int{0, 1, 2, 3} {
		readings[5*24+hour].VoltageV = 241.2
	}
	anomalies := Analyze(readings, nil, DefaultDetectorConfig())
	types := map[AnomalyType]bool{}
	for _, anomaly := range anomalies {
		if anomaly.MeterCode == "M-BOTH" {
			types[anomaly.Type] = true
		}
	}
	if !types[AnomalyReal] || !types[AnomalyDataQuality] {
		t.Fatalf("got types %v, want both REAL and DATA_QUALITY on one meter", types)
	}
}
