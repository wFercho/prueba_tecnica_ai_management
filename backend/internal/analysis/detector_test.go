package analysis

import (
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// synthetic builds a meter's readings over days of hourly data. level receives the
// day offset and the hour of day so a test can describe a profile, a step or a dip.
func synthetic(meter catalog.MeterCode, days int, level func(day, hour int) float64) []catalog.Reading {
	out := make([]catalog.Reading, 0, days*24)
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for day := range days {
		for hour := range 24 {
			ts := start.AddDate(0, 0, day).Add(time.Duration(hour) * time.Hour)
			out = append(out, catalog.Reading{
				MeterCode:      meter,
				Timestamp:      ts,
				ConsumptionKWh: level(day, hour),
				VoltageV:       220,
				CurrentA:       level(day, hour) * 1000 / (220 * 0.93),
				PowerFactor:    0.93,
				IngestedStatus: "OK",
			})
		}
	}
	return out
}

// flatThen steps up after the given day index.
func flatThen(days, stepDay int, before, after float64) func(int, int) float64 {
	return func(day, _ int) float64 {
		if day >= stepDay {
			return after
		}
		return before
	}
}

func TestConsumptionEpisodeFindsSustainedStep(t *testing.T) {
	readings := synthetic("M-STEP", 14, flatThen(14, 10, 30, 44))

	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())

	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	episode := episodes[0]
	if got, want := episode.Start, time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("Start = %v, want %v", got, want)
	}
	if got, want := episode.ReadingCount(), 4*24; got != want {
		t.Errorf("ReadingCount = %d, want %d (four days at 44 kWh)", got, want)
	}
	if episode.Deviation < 0.40 || episode.Deviation > 0.50 {
		t.Errorf("Deviation = %v, want roughly +46%%", episode.Deviation)
	}
}

func TestConsumptionEpisodeIgnoresStableMeter(t *testing.T) {
	readings := synthetic("M-FLAT", 14, flatThen(14, 10, 30, 30))

	if episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig()); len(episodes) != 0 {
		t.Fatalf("got %d episodes for a stable meter, want 0: %+v", len(episodes), episodes[0])
	}
}

func TestConsumptionEpisodeIgnoresDiurnalRamp(t *testing.T) {
	// A meter that is only busy in the afternoon must not read as anomalous every
	// time it ramps up. This is what hour-of-day resolution buys (ADR-0003).
	readings := synthetic("M-DIURNAL", 14, func(_, hour int) float64 {
		if hour >= 8 && hour < 18 {
			return 60
		}
		return 25
	})

	if episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig()); len(episodes) != 0 {
		t.Fatalf("got %d episodes for a stable diurnal meter, want 0", len(episodes))
	}
}

func TestConsumptionEpisodeFindsOutage(t *testing.T) {
	// Day 7 reads near zero for twelve hours: a scheduled outage.
	readings := synthetic("M-OUTAGE", 14, func(day, hour int) float64 {
		if day == 7 && hour < 12 {
			return 8
		}
		return 30
	})

	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())

	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	episode := episodes[0]
	if got, want := episode.ReadingCount(), 12; got != want {
		t.Errorf("ReadingCount = %d, want %d", got, want)
	}
	if episode.Deviation > -0.5 {
		t.Errorf("Deviation = %v, want a drop well past -50%%", episode.Deviation)
	}
}

func TestConsumptionEpisodeRejectsSingleOddReading(t *testing.T) {
	// One reading 15% off, statistically far but materially small: noise.
	readings := synthetic("M-NOISE", 14, flatThen(14, 10, 30, 30))
	readings[5*24+3].ConsumptionKWh = 34.5

	if episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig()); len(episodes) != 0 {
		t.Fatalf("got %d episodes, want 0 for a single 15%% excursion", len(episodes))
	}
}

func TestConsumptionEpisodeAcceptsSingleSpike(t *testing.T) {
	// One reading at four times its baseline: a spike, on its own.
	readings := synthetic("M-SPIKE", 14, flatThen(14, 10, 30, 30))
	spikeAt := readings[6*24+3].Timestamp
	readings[6*24+3].ConsumptionKWh = 120

	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())

	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	if got := episodes[0].ReadingCount(); got != 1 {
		t.Errorf("ReadingCount = %d, want 1", got)
	}
	if !episodes[0].Start.Equal(spikeAt) || !episodes[0].End.Equal(spikeAt) {
		t.Errorf("window = %v..%v, want the single reading at %v", episodes[0].Start, episodes[0].End, spikeAt)
	}
}

func TestConsumptionEpisodeRejectsRunBelowMinimum(t *testing.T) {
	readings := synthetic("M-BRIEF", 14, flatThen(14, 10, 30, 30))
	// One hour of doubled consumption, then back to normal: too brief to be an
	// episode, and not a big enough jump to be a spike.
	readings[8*24+3].ConsumptionKWh = 60
	readings[8*24+4].ConsumptionKWh = 60

	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())
	// Two readings at +100% do clear the spike threshold, so they form an episode.
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	if got := episodes[0].ReadingCount(); got != 2 {
		t.Errorf("ReadingCount = %d, want 2", got)
	}
}

func TestConsumptionEpisodeKeepsBaselineFrozenAcrossLongChange(t *testing.T) {
	// A change that runs to the end of the data must not go silent by redefining
	// what normal is. If the baseline were recomputed per reading, the second half
	// of this episode would read as normal and the episode would be cut in two.
	readings := synthetic("M-LONG", 14, flatThen(14, 4, 30, 60))

	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())

	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want the change to stay one episode", len(episodes))
	}
	if got, want := episodes[0].ReadingCount(), 10*24; got != want {
		t.Errorf("ReadingCount = %d, want %d", got, want)
	}
}

func TestConsumptionEpisodeJoinsFindingsWithinMaxGap(t *testing.T) {
	// M-112's corruption lands every third hour. Readings three hours apart are
	// still the same episode, or the finding would be shredded into singletons.
	readings := synthetic("M-GAP", 14, flatThen(14, 10, 30, 30))
	mark := map[int]bool{0: true, 3: true, 6: true}
	for hour := range 6 {
		idx := 6*24 + hour
		readings[idx].ConsumptionKWh = 9
		_ = mark
	}

	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())

	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	if got, want := episodes[0].ReadingCount(), 6; got != want {
		t.Errorf("ReadingCount = %d, want %d", got, want)
	}
}

func TestConsumptionEpisodeSeparatesDistantEpisodes(t *testing.T) {
	readings := synthetic("M-TWO", 14, func(day, hour int) float64 {
		switch {
		case day == 3 && hour < 6:
			return 70
		case day == 9 && hour < 6:
			return 70
		default:
			return 30
		}
	})

	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())

	if len(episodes) != 2 {
		t.Fatalf("got %d episodes, want 2", len(episodes))
	}
	if !episodes[0].Start.Before(episodes[1].Start) {
		t.Errorf("episodes are out of order: %v then %v", episodes[0].Start, episodes[1].Start)
	}
}
