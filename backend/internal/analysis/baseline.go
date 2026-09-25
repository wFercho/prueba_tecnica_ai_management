package analysis

import (
	"math"
	"sort"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// ProfilePoint is a meter's expected consumption at one hour of day, with the
// robust scale that says how far a reading may sit from it before it counts as
// anomalous.
type ProfilePoint struct {
	// Expected is the median consumption at this hour of day over the window.
	Expected float64
	// Scale is 1.4826 * MAD, the robust standard deviation of that hour.
	Scale float64
	// SampleCount is how many readings the point was learned from.
	SampleCount int
}

// HourlyProfile is a meter's expected consumption for each hour of day.
//
// It is a Baseline: expected consumption for one meter at one hour of day, over a
// trailing window that excludes the candidate episode. Learned per meter, never
// global — the twelve meters in the delivered dataset share one normalised diurnal
// shape (pairwise r = 0.95-0.999) and differ only in absolute magnitude, so a
// global profile would either miss every meter or fire on all of them.
type HourlyProfile struct {
	points [24]ProfilePoint
	known  [24]bool
}

// Point returns the baseline for one hour of day.
func (p HourlyProfile) Point(hour int) (ProfilePoint, bool) {
	if hour < 0 || hour > 23 || !p.known[hour] {
		return ProfilePoint{}, false
	}
	return p.points[hour], true
}

// Hours returns the hours of day this profile can speak about.
func (p HourlyProfile) Hours() []int {
	var out []int
	for hour := range 24 {
		if p.known[hour] {
			out = append(out, hour)
		}
	}
	sort.Ints(out)
	return out
}

// Deviation returns how far a reading sits from the baseline for its hour, as a
// fraction of the baseline, and the number of robust standard deviations it sits
// away. The two are reported together because either alone is a weak detector: a
// tiny baseline makes a small absolute error a huge relative one, and a large
// scale makes a real jump invisible.
func (p HourlyProfile) Deviation(reading catalog.Reading) (relative float64, sigma float64, ok bool) {
	point, ok := p.Point(reading.Timestamp.Hour())
	if !ok || point.Expected == 0 {
		return 0, 0, false
	}
	scale := point.Scale
	if scale == 0 {
		// A perfectly flat hour has no dispersion to measure against; treat a
		// single percent of the level as the scale so the test stays decidable.
		scale = point.Expected * flatScaleFraction
	}
	return (reading.ConsumptionKWh - point.Expected) / point.Expected, math.Abs(reading.ConsumptionKWh-point.Expected) / scale, true
}

// flatScaleFraction is the dispersion assumed for an hour whose history shows no
// spread at all.
const flatScaleFraction = 0.01

// BuildHourlyProfile learns the expected consumption at each hour of day from the
// readings handed to it. Callers pass the meter's clean history: readings outside
// every candidate episode, so a sustained change cannot redefine normal.
func BuildHourlyProfile(readings []catalog.Reading, cfg DetectorConfig) HourlyProfile {
	byHour := make(map[int][]float64, 24)
	for _, r := range readings {
		byHour[r.Timestamp.Hour()] = append(byHour[r.Timestamp.Hour()], r.ConsumptionKWh)
	}

	var profile HourlyProfile
	for hour, samples := range byHour {
		if len(samples) < cfg.MinProfileHistory {
			continue
		}
		sort.Float64s(samples)
		median := medianOfSorted(samples)
		deviations := make([]float64, len(samples))
		for i, sample := range samples {
			deviations[i] = math.Abs(sample - median)
		}
		sort.Float64s(deviations)
		profile.points[hour] = ProfilePoint{
			Expected:    median,
			Scale:       robustScaleFromSorted(deviations),
			SampleCount: len(samples),
		}
		profile.known[hour] = true
	}
	return profile
}

func medianOfSorted(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// robustScaleFromSorted turns a sorted slice of absolute deviations into the
// scale a robust z-score divides by.
func robustScaleFromSorted(sortedDeviations []float64) float64 {
	return 1.4826 * medianOfSorted(sortedDeviations)
}

// Median is the median of a sample, used by the quality and confidence stages.
func Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return medianOfSorted(sorted)
}

// robustScale is 1.4826 * MAD: the standard deviation a healthy sample has, in
// the units this package reports. Used for whole-history dispersion, where the
// sample is large enough for MAD to be meaningful.
func robustScale(values []float64) float64 {
	med := Median(values)
	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - med)
	}
	return robustScaleFromSorted(deviations)
}

// spread summarises a sample of readings that should be behaving alike.
type spread struct {
	Median float64
	Scale  float64
}

// newSpread measures a sample's dispersion. floorFraction is the dispersion
// assumed when the sample shows none at all, as a fraction of its median: a
// perfectly flat series has no scale to divide by, and reporting every reading as
// infinitely significant would be a worse answer than treating small noise as
// noise. Every floor below is set under the smallest dispersion the delivered
// dataset actually shows, so no floor loosens a real threshold.
func newSpread(values []float64, floorFraction float64) spread {
	median := Median(values)
	scale := robustScale(values)
	if floor := math.Abs(median) * floorFraction; scale < floor {
		scale = floor
	}
	return spread{Median: median, Scale: scale}
}

// sigma returns how many robust standard deviations value sits from the centre.
func (s spread) sigma(value float64) float64 {
	scale := s.Scale
	if scale == 0 {
		return 0
	}
	return math.Abs(value-s.Median) / scale
}

// Profile returns the baseline that would have applied to a window starting at
// start, learned from every reading strictly before it.
func ProfileBefore(readings []catalog.Reading, start time.Time, cfg DetectorConfig) HourlyProfile {
	clean := make([]catalog.Reading, 0, len(readings))
	for _, r := range readings {
		if r.Timestamp.Before(start) {
			clean = append(clean, r)
		}
	}
	return BuildHourlyProfile(clean, cfg)
}
