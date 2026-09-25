package analysis

import (
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// EpisodeKind is which detector found an episode.
type EpisodeKind string

const (
	// EpisodeConsumption is a run of readings that departed from the meter's
	// own hour-of-day baseline.
	EpisodeConsumption EpisodeKind = "CONSUMPTION"
	// EpisodeDataQuality is a run of readings whose reported electrical values
	// are not physically consistent with the reported consumption.
	EpisodeDataQuality EpisodeKind = "DATA_QUALITY"
)

// Episode is a contiguous run of anomalous readings on a single meter, bounded by
// a first and last reading. It is the unit the detector reasons about and the unit
// an Anomaly is stored as (ADR-0002).
type Episode struct {
	MeterCode catalog.MeterCode
	Kind      EpisodeKind
	Start     time.Time
	End       time.Time
	// Readings holds the anomalous readings themselves, in order.
	Readings []catalog.Reading
	// Profile is the baseline the episode was measured against, frozen at the
	// episode's first reading.
	Profile HourlyProfile
	// Deviation is the relative departure of the episode's consumption from its
	// baseline over the same hours: (actual - baseline) / baseline.
	Deviation float64
	// ActualKWh and BaselineKWh are the two sides of that comparison, so the
	// explanation can quote the numbers rather than only the ratio.
	ActualKWh   float64
	BaselineKWh float64
	// Findings are the data-quality findings, one per affected reading, set only
	// on data-quality episodes.
	Findings []DataQualityFinding
	// CorroboratingVariables is how many independent measurements moved with the
	// episode, out of DetectorConfig.CorroboratingVariables.
	CorroboratingVariables int
	// Corroborating names them, for the explanation to cite.
	Corroborating []string
	// Corroboration is the corroboration evidence term in [0,1].
	Corroboration float64
	// CorroboratingCount is how many readings carry data-quality findings, set on
	// data-quality episodes.
	CorroboratingCount int
	// AffectedShare is the share of the meter's readings the episode covers, set
	// on data-quality episodes.
	AffectedShare float64
	// HighByDataQualityVolume records whether enough readings are corrupt to make
	// the finding high severity.
	HighByDataQualityVolume bool
	// CorrelatedEvents are the operational events that fall in the episode's
	// window, extended backwards by DetectorConfig.EventLookback.
	CorrelatedEvents []catalog.OperationalEvent
}

// ReadingCount is how many readings the episode covers.
func (e Episode) ReadingCount() int { return len(e.Readings) }

// ExplainingEvent returns the correlated event that accounts for the episode, if
// any. An event of type UNKNOWN never accounts for anything.
func (e Episode) ExplainingEvent() (catalog.OperationalEvent, bool) {
	for _, event := range e.CorrelatedEvents {
		if event.Explains() {
			return event, true
		}
	}
	return catalog.OperationalEvent{}, false
}

// FindConsumptionEpisodes walks a meter's readings once and returns the episodes in
// which its consumption departed from its own hour-of-day baseline.
//
// The baseline is frozen at the first reading of an episode and released when the
// episode closes, so a sustained change is measured against the behaviour that
// preceded it for its whole length and cannot go silent by redefining normal
// (ADR-0003). Readings are only admitted to the profile once they are known to be
// clean, so the episode never appears in its own baseline.
func FindConsumptionEpisodes(readings []catalog.Reading, cfg DetectorConfig) []Episode {
	ordered := sortedReadings(readings)
	clean := make([]catalog.Reading, 0, len(ordered))
	var episodes []Episode
	var run []catalog.Reading
	var runProfile HourlyProfile
	runIsSpike := false

	// closeRun emits a finished run as an episode, or returns its readings to the
	// clean history when the run is too short to be one and did not spike.
	closeRun := func() {
		if len(run) == 0 {
			return
		}
		if len(run) >= cfg.MinEpisodeReadings || runIsSpike {
			episodes = append(episodes, buildEpisode(run[0].MeterCode, EpisodeConsumption, run, runProfile, cfg))
		} else {
			clean = append(clean, run...)
		}
		run = nil
		runIsSpike = false
	}

	for _, reading := range ordered {
		if len(run) == 0 {
			// Learned from the clean history only, and frozen for as long as the
			// run lasts, so the run never appears in its own baseline.
			runProfile = BuildHourlyProfile(clean, cfg)
		}
		relative, sigma, ok := runProfile.Deviation(reading)
		if !ok {
			// No baseline for this hour yet: the reading is the only evidence
			// about it, so it is history, not an anomaly.
			clean = append(clean, reading)
			continue
		}

		anomalous := sigma > cfg.KConsumptionSigma && abs(relative) >= cfg.MinConsumptionDeviation
		if anomalous {
			if len(run) == 0 || reading.Timestamp.Sub(run[len(run)-1].Timestamp) > cfg.MaxEpisodeGap {
				closeRun()
				runProfile = BuildHourlyProfile(clean, cfg)
				run = []catalog.Reading{reading}
				// A lone reading more than triple its hour's expectation stands as
				// an episode in its own right; anything shorter is noise.
				runIsSpike = abs(relative) >= cfg.SpikeDeviation
				continue
			}
			run = append(run, reading)
			continue
		}

		if len(run) > 0 && reading.Timestamp.Sub(run[len(run)-1].Timestamp) > cfg.MaxEpisodeGap {
			closeRun()
		}
		if len(run) == 0 {
			clean = append(clean, reading)
		}
		// Otherwise the reading is quiet but inside the gap that keeps the run
		// open. It is clean history; it is not a member of the episode.
	}
	closeRun()

	for i := range episodes {
		corroborate(ordered, &episodes[i], cfg)
	}
	return episodes
}

func buildEpisode(meter catalog.MeterCode, kind EpisodeKind, run []catalog.Reading, profile HourlyProfile, cfg DetectorConfig) Episode {
	episode := Episode{
		MeterCode: meter,
		Kind:      kind,
		Start:     run[0].Timestamp,
		End:       run[len(run)-1].Timestamp,
		Readings:  run,
		Profile:   profile,
	}
	episode.ActualKWh, episode.BaselineKWh = compareToProfile(run, profile)
	if episode.BaselineKWh > 0 {
		episode.Deviation = (episode.ActualKWh - episode.BaselineKWh) / episode.BaselineKWh
	}
	return episode
}

// compareToProfile sums the episode's consumption and the baseline for the same
// hours, so a multi-day episode is compared hour by hour against the profile
// rather than against one level.
func compareToProfile(readings []catalog.Reading, profile HourlyProfile) (actual, baseline float64) {
	for _, reading := range readings {
		actual += reading.ConsumptionKWh
		if point, ok := profile.Point(reading.Timestamp.Hour()); ok {
			baseline += point.Expected
		}
	}
	return actual, baseline
}

func sortedReadings(readings []catalog.Reading) []catalog.Reading {
	ordered := append([]catalog.Reading(nil), readings...)
	sortReadingsByTime(ordered)
	return ordered
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// Correlate attaches the operational events that fall inside the episode's window,
// extended backwards by the configured lookback so an event announced before a
// change is still its cause.
func Correlate(all []catalog.OperationalEvent, episode *Episode, cfg DetectorConfig) {
	from := episode.Start.Add(-cfg.EventLookback)
	for _, event := range all {
		if event.MeterCode != episode.MeterCode {
			continue
		}
		if event.Timestamp.Before(from) || event.Timestamp.After(episode.End) {
			continue
		}
		episode.CorrelatedEvents = append(episode.CorrelatedEvents, event)
	}
}

// DeviationSeries is the episode's own hourly readings against the baseline that
// judged them.
//
// It is the series rather than the summary, because a reader looking at an
// investigation needs to see *when* the change happened — a step at 03:00 and a
// ramp are different problems — and because an aggregate alone is derivable from
// inputs the narrator already has, which would leave the narrator nothing to say
// that the rules have not already said.
func (e Episode) DeviationSeries() []Point {
	points := make([]Point, 0, len(e.Readings))
	for _, reading := range e.Readings {
		point := Point{
			Timestamp:   reading.Timestamp.UTC(),
			ActualKWh:   round(reading.ConsumptionKWh, 3),
			VoltageV:    round(reading.VoltageV, 2),
			CurrentA:    round(reading.CurrentA, 2),
			PowerFactor: round(reading.PowerFactor, 3),
		}
		if expected, ok := e.Profile.Point(reading.Timestamp.Hour()); ok {
			point.BaselineKWh = round(expected.Expected, 3)
			if expected.Expected != 0 {
				point.Deviation = round((reading.ConsumptionKWh-expected.Expected)/expected.Expected, 4)
			}
		}
		points = append(points, point)
	}
	return points
}
