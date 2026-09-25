package analysis

import "time"

// DetectorConfig holds every tunable the detectors need (ADR-0011). No threshold
// lives anywhere else in the repository.
//
// Each default is justified by a measurement on the delivered 14-day dataset:
// twelve meters, 336 hourly readings each, no gaps, all ingested status OK.
//
// The eight meters with no operational event — M-101, M-102, M-103, M-105,
// M-107, M-108, M-110, M-111 — are the control set. Their largest day-over-day
// swing is 4.63% and none of them carries a single reading outside its own robust
// band. They must produce no anomalies at all; the negative-control test in
// dataset_test.go is the gate on every value in this struct, because the obvious
// way to silence a false positive is to loosen the threshold that fired it.
//
// These values are calibrated to one dataset. If that dataset is replaced, they
// must be re-calibrated against it and this comment updated with the new
// measurements.
type DetectorConfig struct {
	// KConsumptionSigma is how many robust standard deviations from the
	// hour-of-day baseline a reading must sit before it is considered anomalous.
	// 5: the control set's readings sit within 3.1 sigma of their own baseline.
	KConsumptionSigma float64
	// MinConsumptionDeviation is the relative departure that makes a reading
	// materially anomalous, so that a statistical blip is not an anomaly.
	// 0.20: the control set never moves 20% from its own hour-of-day baseline,
	// while the smallest planted departure (M-106's outage) is 78% below it.
	MinConsumptionDeviation float64
	// MinEpisodeReadings is the shortest run of anomalous readings that becomes
	// an episode. 2: a single mis-timed reading is noise, and every planted
	// episode is 12 hours or longer.
	MinEpisodeReadings int
	// SpikeDeviation is the relative departure at which one reading stands alone
	// as an episode. 2.0 (three times the baseline): a single reading more than
	// double its hour's expectation is not a coincidence.
	SpikeDeviation float64
	// MinProfileHistory is how many readings at an hour of day are needed before
	// a baseline can be learned for it. 3: fewer samples and the median is
	// whatever the last two readings happened to be.
	MinProfileHistory int
	// MaxEpisodeGap is the largest gap between two anomalous readings that still
	// keeps them in one episode. 3h: corrupt sampling and short interruptions
	// produce periodic rather than contiguous gaps — M-112's 16 corrupt readings
	// are spaced exactly three hours apart — while a wider gap would merge two
	// unrelated episodes.
	MaxEpisodeGap time.Duration
	// MinDataQualityReadings is the shortest run of data-quality findings that
	// becomes an episode. 3: a single implausible reading is a transient, and
	// one corrupt hour is not an operational finding.
	MinDataQualityReadings int

	// KVoltageSigma is the robust deviation at which reported voltage is judged
	// corrupt. 5: supply voltage is a property of the grid, not of the load, and
	// no meter's voltage in the control set leaves its own 5-sigma band, while
	// M-112's corrupt readings sit 10.6 to 14.1 sigma out.
	KVoltageSigma float64
	// KCurrentSigma is the robust deviation at which reported current is judged
	// corrupt, judged only on readings whose consumption is itself normal. 5.
	KCurrentSigma float64
	// KPowerFactorSigma is the robust deviation at which reported power factor is
	// judged corrupt, judged only on readings whose consumption is itself
	// normal. 5: M-112's readings sit at 14.8 to 23.8 sigma out.
	KPowerFactorSigma float64
	// KEnergyBalanceSigma is the robust deviation at which the reported
	// consumption stops agreeing with the reported voltage, current and power
	// factor. 6: the widest balance ratio any healthy meter reaches is M-109's
	// 1.475, which is 6.4 sigma from its own median but sits inside the noise
	// envelope of a load that genuinely doubled; M-112's corrupt readings sit
	// 8.6 to 58.6 sigma out, with ratios from 0.58 to 4.22.
	KEnergyBalanceSigma float64
	// MaxFlatConsumptionDeviation is the consumption departure below which an
	// episode carrying data-quality findings is a data-quality problem rather
	// than a real change. 0.15: M-112's consumption moves 0.32% across its
	// corrupt window, while M-109's moves 110%.
	MaxFlatConsumptionDeviation float64
	// MinPlausiblePowerFactor is the lowest power factor a healthy installation
	// reports. 0.80: below it the meter is reporting a load no distribution
	// feeder delivers, and no amount of statistical unusualness is needed to
	// call it a fault. The floor is what keeps ordinary wander — every control
	// meter wanders between 0.88 and 0.99, which against a fourteen-day profile
	// is five to nine sigma — out of the findings list.
	MinPlausiblePowerFactor float64
	// MaxEnergyBalanceError is the furthest the reported consumption may sit from
	// what the reported voltage, current and power factor imply. 0.5: current
	// transformers and phase imbalance routinely leave tens of percent on the
	// table, and the widest gap any healthy meter shows is 47%, while M-112's
	// corrupt readings are off by up to 222%.
	MaxEnergyBalanceError float64

	// HighDeviation is the relative departure at which a real anomaly is high
	// severity. 0.80: M-109 moves 110%; no control meter moves 5%.
	HighDeviation float64
	// MediumDeviation is the relative departure at which a real anomaly is medium
	// severity. 0.25.
	MediumDeviation float64
	// HighCorroboration lets a 40% to 80% departure reach high severity when
	// other independent measurements moved with it. 0.5.
	HighCorroboration float64
	// HighDeviationWithCorroboration is the lower departure paired with
	// HighCorroboration. 0.40.
	HighDeviationWithCorroboration float64
	// DataQualityHighReadings is how many corrupt readings make a data-quality
	// anomaly high severity. 8: M-112 has 16.
	DataQualityHighReadings int
	// DataQualityHighShare is the share of a meter's readings that make a
	// data-quality anomaly high severity. 0.05: 16 of 336 is 4.8%, and the
	// control set has none at all.
	DataQualityHighShare float64
	// CorroborationSigma is the robust deviation at which a variable counts as
	// having moved alongside consumption. 2.
	CorroborationSigma float64
	// CurrentTracksTolerance is how closely the change in current must track the
	// change in consumption for current to corroborate it. 0.25: M-104's current
	// moves +47.4% against a +46.6% consumption change.
	CurrentTracksTolerance float64
	// CorroboratingVariables is how many independent measurements are counted,
	// which is the denominator of the corroboration term.
	CorroboratingVariables int

	// DeviationSaturation is the departure, in percentage points, at which the
	// deviation evidence term reaches 1-exp(-1), about 63% of its range. 25:
	// M-104's 46.6% already reads 0.85, so the term does not simply restate the
	// severity thresholds.
	DeviationSaturation float64
	// PersistenceSaturation is how many readings make the persistence term
	// reach its maximum. 12: the shortest planted episode is M-106's 12 hours.
	PersistenceSaturation int
	// EventLookback is how far before an episode an event may fall and still be
	// correlated with it. 24h: an event announced hours before a change is still
	// its cause.
	EventLookback time.Duration

	// Weights are the exponents of the weighted geometric mean that produces the
	// confidence score (ADR-0005).
	Weights EvidenceWeights
}

// EvidenceWeights are the weights of the four evidence terms. They sum to 1, so
// the weighted geometric mean of terms in [0,1] is itself in [0,1].
//
// Deviation carries the most weight because it is the measurement; event match and
// corroboration are the two ways a measurement can be independently supported;
// persistence is the weakest because length is the easiest thing to have by
// accident.
type EvidenceWeights struct {
	Deviation     float64
	EventMatch    float64
	Corroboration float64
	Persistence   float64
}

// DefaultDetectorConfig returns the calibration documented on DetectorConfig.
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		KConsumptionSigma:       5.0,
		MinConsumptionDeviation: 0.20,
		MinEpisodeReadings:      2,
		SpikeDeviation:          2.0,
		MinProfileHistory:       3,
		MaxEpisodeGap:           3 * time.Hour,
		MinDataQualityReadings:  3,

		KVoltageSigma:               5.0,
		KCurrentSigma:               5.0,
		KPowerFactorSigma:           5.0,
		KEnergyBalanceSigma:         6.0,
		MinPlausiblePowerFactor:     0.80,
		MaxEnergyBalanceError:       0.5,
		MaxFlatConsumptionDeviation: 0.15,

		HighDeviation:                  0.80,
		MediumDeviation:                0.25,
		HighCorroboration:              0.5,
		HighDeviationWithCorroboration: 0.40,
		DataQualityHighReadings:        8,
		DataQualityHighShare:           0.05,
		CorroborationSigma:             2.0,
		CurrentTracksTolerance:         0.25,
		CorroboratingVariables:         4,

		DeviationSaturation:   25.0,
		PersistenceSaturation: 12,
		EventLookback:         24 * time.Hour,

		Weights: EvidenceWeights{
			Deviation:     0.40,
			EventMatch:    0.20,
			Corroboration: 0.25,
			Persistence:   0.15,
		},
	}
}
