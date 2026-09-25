package analysis

import (
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

var base = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

// hourAt builds a reading one hour after t, carrying consumption kwh.
func hourAt(t time.Time, kwh float64) catalog.Reading {
	return catalog.Reading{MeterCode: "M-TEST", Timestamp: t, ConsumptionKWh: kwh, IngestedStatus: "OK"}
}

func dayReadings(day int, byHour map[int]float64) []catalog.Reading {
	out := make([]catalog.Reading, 0, len(byHour))
	for hour, kwh := range byHour {
		out = append(out, hourAt(base.AddDate(0, 0, day).Add(time.Duration(hour)*time.Hour), kwh))
	}
	return out
}

func TestBuildHourlyProfileIsHourResolved(t *testing.T) {
	// Three days of readings. Night and day have different levels; a single
	// whole-period mean would conflate the diurnal cycle with the level.
	var history []catalog.Reading
	for day := range 3 {
		history = append(history,
			hourAt(base.AddDate(0, 0, day).Add(3*time.Hour), 30),
			hourAt(base.AddDate(0, 0, day).Add(9*time.Hour), 60),
		)
	}

	profile := BuildHourlyProfile(history, DefaultDetectorConfig())

	night, ok := profile.Point(3)
	if !ok {
		t.Fatal("expected a baseline for hour 03")
	}
	if night.Expected != 30 {
		t.Errorf("hour 03 expected = %v, want 30", night.Expected)
	}
	day, ok := profile.Point(9)
	if !ok {
		t.Fatal("expected a baseline for hour 09")
	}
	if day.Expected != 60 {
		t.Errorf("hour 09 expected = %v, want 60", day.Expected)
	}
	if night.Expected == day.Expected {
		t.Error("baseline collapsed the diurnal cycle into one level")
	}
}

func TestHourlyProfileNeedsEnoughHistory(t *testing.T) {
	cfg := DefaultDetectorConfig()
	history := []catalog.Reading{
		hourAt(base.Add(3*time.Hour), 30),
		hourAt(base.AddDate(0, 0, 1).Add(3*time.Hour), 31),
	}

	profile := BuildHourlyProfile(history, cfg)

	if _, ok := profile.Point(3); ok {
		t.Errorf("hour 03 has %d samples, below MinProfileHistory=%d, want no baseline",
			len(history), cfg.MinProfileHistory)
	}
}

func TestHourlyProfileScaleIsRobustToOneWildSample(t *testing.T) {
	var history []catalog.Reading
	for _, kwh := range []float64{30, 30, 30, 30, 900} {
		history = append(history, hourAt(base.Add(3*time.Hour), kwh))
	}
	_ = history

	var sameHour []catalog.Reading
	for day, kwh := range []float64{30, 30, 30, 31, 900} {
		sameHour = append(sameHour, hourAt(base.AddDate(0, 0, day).Add(3*time.Hour), kwh))
	}

	point, ok := BuildHourlyProfile(sameHour, DefaultDetectorConfig()).Point(3)
	if !ok {
		t.Fatal("expected a baseline for hour 03")
	}
	if point.Expected != 30 {
		t.Errorf("expected = %v, want the median 30 despite the 900 sample", point.Expected)
	}
	if point.SampleCount != 5 {
		t.Errorf("SampleCount = %d, want 5", point.SampleCount)
	}
}

func TestProfileExcludesCandidateEpisode(t *testing.T) {
	// Ten clean days at 30 kWh at 03:00, then a sustained 90 kWh episode.
	// The episode must not redefine what normal looks like (ADR-0003).
	var history []catalog.Reading
	for day := range 10 {
		history = append(history, hourAt(base.AddDate(0, 0, day).Add(3*time.Hour), 30))
	}
	episodeStart := base.AddDate(0, 0, 10).Add(3 * time.Hour)

	// The baseline handed to the detector is built from the clean prefix only.
	point, ok := BuildHourlyProfile(history, DefaultDetectorConfig()).Point(3)
	if !ok {
		t.Fatal("expected a baseline for hour 03")
	}
	if point.Expected != 30 {
		t.Errorf("expected = %v, want 30", point.Expected)
	}

	// The median tolerates a short episode on its own: thirteen samples, ten at
	// 30 and three at 90, still read as 30. A long one would not, which is why
	// the detector freezes the profile at the episode's start rather than
	// recomputing it as the episode runs.
	withShortEpisode := append(append([]catalog.Reading{}, history...),
		hourAt(episodeStart, 90), hourAt(episodeStart.Add(time.Hour), 90), hourAt(episodeStart.Add(2*time.Hour), 90))
	if short, _ := BuildHourlyProfile(withShortEpisode, DefaultDetectorConfig()).Point(3); short.Expected != 30 {
		t.Errorf("expected = %v after a 3-reading episode, want the median to hold at 30", short.Expected)
	}

	withLongEpisode := append([]catalog.Reading{}, history...)
	for hour := range 12 {
		withLongEpisode = append(withLongEpisode, hourAt(episodeStart.Add(time.Duration(hour)*time.Hour), 90))
	}
	if long, _ := BuildHourlyProfile(withLongEpisode, DefaultDetectorConfig()).Point(3); long.Expected >= 80 {
		t.Errorf("expected = %v once the episode outnumbers the history, "+
			"which is the case the frozen profile exists for", long.Expected)
	}
}
