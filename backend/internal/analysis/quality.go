package analysis

import (
	"sort"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// Variable names one measured quantity a data-quality finding is about.
type Variable string

const (
	VariableVoltage       Variable = "voltage"
	VariableCurrent       Variable = "current"
	VariablePowerFactor   Variable = "power_factor"
	VariableEnergyBalance Variable = "energy_balance"
)

// DataQualityFinding is our per-reading verdict that the reported voltage,
// current and power factor are not physically consistent with the reported
// consumption. It is distinct from Ingested status, which is the source's claim.
type DataQualityFinding struct {
	Timestamp time.Time `json:"timestamp"`
	Variable  Variable  `json:"variable"`
	// Value is what the reading reported for that variable.
	Value float64 `json:"value"`
	// Expected is what this meter's own history says for it.
	Expected float64 `json:"expected"`
	// Sigma is how many robust standard deviations away Value sits. It is
	// evidence that the value is unusual, not a measure of how wrong it is: a
	// fourteen-day profile can be tight enough to call ordinary wander many
	// sigma out.
	Sigma float64 `json:"sigma"`
	// RelativeDeviation is how far Value sits from what a healthy installation
	// could report, as a fraction of the reference. It is the measure of how
	// wrong the reading is, and it is comparable across meters and variables in
	// a way Sigma is not.
	RelativeDeviation float64 `json:"relative_deviation"`
}

// Floors on dispersion, as a fraction of the median, for meters whose history is
// implausibly flat. Supply voltage is 0.5% (1.1V at 220V; the flattest meter in
// the delivered dataset shows 1.119V), current 1%, power factor 0.5% and the
// energy balance ratio 1% (the flattest shows 5.2%).
const (
	voltageScaleFloor     = 0.005
	currentScaleFloor     = 0.01
	powerFactorScaleFloor = 0.005
	balanceScaleFloor     = 0.01
)

// meterElectricals is a meter's own history for the quantities that should not
// depend on what it is currently doing.
type meterElectricals struct {
	voltage     spread
	current     spread
	powerFactor spread
	balance     spread
}

// balanceRatio is the energy balance residual: reported consumption against what
// the reported voltage, current and power factor imply. For an hour of metering,
// consumption in kWh is voltage * current * power factor / 1000, so the ratio
// should sit near the meter's own historical value.
func balanceRatio(reading catalog.Reading) float64 {
	implied := reading.VoltageV * reading.CurrentA * reading.PowerFactor / 1000
	if implied == 0 {
		return 0
	}
	return reading.ConsumptionKWh / implied
}

func measureMeter(readings []catalog.Reading) meterElectricals {
	voltages := make([]float64, 0, len(readings))
	currents := make([]float64, 0, len(readings))
	powerFactors := make([]float64, 0, len(readings))
	balances := make([]float64, 0, len(readings))
	for _, reading := range readings {
		voltages = append(voltages, reading.VoltageV)
		currents = append(currents, reading.CurrentA)
		powerFactors = append(powerFactors, reading.PowerFactor)
		balances = append(balances, balanceRatio(reading))
	}
	return meterElectricals{
		voltage:     newSpread(voltages, voltageScaleFloor),
		current:     newSpread(currents, currentScaleFloor),
		powerFactor: newSpread(powerFactors, powerFactorScaleFloor),
		balance:     newSpread(balances, balanceScaleFloor),
	}
}

// consumptionAnomaly is the per-reading verdict that a reading's consumption has
// departed from the meter's own hour-of-day baseline. The data-quality stage needs
// it because a load that genuinely changed is expected to move current and power
// factor with it: judging those variables on a reading whose consumption has
// already departed would report every real load change as a broken meter.
func consumptionAnomaly(readings []catalog.Reading, cfg DetectorConfig) map[time.Time]bool {
	out := make(map[time.Time]bool, len(readings))
	ordered := sortedReadings(readings)
	clean := make([]catalog.Reading, 0, len(ordered))
	var frozen HourlyProfile
	var episodeStart time.Time
	for _, reading := range ordered {
		if len(clean) == 0 || reading.Timestamp.Sub(episodeStart) > cfg.MaxEpisodeGap {
			frozen = BuildHourlyProfile(clean, cfg)
		}
		relative, sigma, ok := frozen.Deviation(reading)
		if !ok {
			out[reading.Timestamp] = false
			clean = append(clean, reading)
			continue
		}
		anomalous := sigma > cfg.KConsumptionSigma && abs(relative) >= cfg.MinConsumptionDeviation
		out[reading.Timestamp] = anomalous
		if anomalous {
			if episodeStart.IsZero() {
				episodeStart = reading.Timestamp
			}
			continue
		}
		episodeStart = reading.Timestamp
		clean = append(clean, reading)
	}
	return out
}

// DataQualityFindings returns one finding per reading whose reported electrical
// values are not physically consistent, in time order.
//
// Voltage is judged on its own: supply voltage is a property of the grid, not of
// the load, so a reading whose voltage has left the meter's own band is wrong
// whatever it is doing. Current, power factor and the energy balance are judged
// only on readings whose consumption is itself normal, because a changed load is
// expected to move them.
func DataQualityFindings(readings []catalog.Reading) []DataQualityFinding {
	cfg := DefaultDetectorConfig()
	return dataQualityFindings(readings, cfg)
}

func dataQualityFindings(readings []catalog.Reading, cfg DetectorConfig) []DataQualityFinding {
	electricals := measureMeter(readings)
	consumptionMoved := consumptionAnomaly(readings, cfg)

	var findings []DataQualityFinding
	for _, reading := range sortedReadings(readings) {
		if sigma := electricals.voltage.sigma(reading.VoltageV); sigma > cfg.KVoltageSigma {
			findings = append(findings, DataQualityFinding{
				Timestamp:         reading.Timestamp,
				Variable:          VariableVoltage,
				Value:             reading.VoltageV,
				Expected:          electricals.voltage.Median,
				Sigma:             sigma,
				RelativeDeviation: relativeTo(reading.VoltageV, electricals.voltage.Median),
			})
		}
		if consumptionMoved[reading.Timestamp] {
			continue
		}
		for _, candidate := range []struct {
			variable Variable
			value    float64
			centre   spread
			sigma    float64
			// reference is what a healthy installation could report, and the
			// denominator of the relative deviation.
			reference float64
			plausible func(float64) bool
		}{
			{VariableCurrent, reading.CurrentA, electricals.current, cfg.KCurrentSigma, electricals.current.Median, nil},
			{VariablePowerFactor, reading.PowerFactor, electricals.powerFactor, cfg.KPowerFactorSigma,
				cfg.MinPlausiblePowerFactor,
				func(v float64) bool { return v >= cfg.MinPlausiblePowerFactor }},
			{VariableEnergyBalance, balanceRatio(reading), electricals.balance, cfg.KEnergyBalanceSigma, 1,
				func(v float64) bool { return abs(v-1) <= cfg.MaxEnergyBalanceError }},
		} {
			// A sigma alone is not enough to call a reading faulty. Fourteen days
			// of history give an hour-of-day profile whose spread is a few
			// hundredths, so ordinary jitter reads as many sigma and every
			// reading becomes a finding. Where the variable has a physical floor,
			// a finding needs both: unusual against this meter's own history, and
			// outside what a healthy installation can produce. Neither test alone
			// is trustworthy — the floor would flag a meter stuck at an odd but
			// legal reading, and the sigma would flag ordinary wander — and
			// together they leave the control meters silent.
			sigma := candidate.centre.sigma(candidate.value)
			if sigma <= candidate.sigma {
				continue
			}
			if candidate.plausible != nil && candidate.plausible(candidate.value) {
				continue
			}
			findings = append(findings, DataQualityFinding{
				Timestamp:         reading.Timestamp,
				Variable:          candidate.variable,
				Value:             candidate.value,
				Expected:          candidate.centre.Median,
				Sigma:             sigma,
				RelativeDeviation: relativeTo(candidate.value, candidate.reference),
			})
		}
	}
	return findings
}

// FindDataQualityEpisodes groups a meter's data-quality findings into episodes,
// the same way the consumption detector groups departures from the baseline.
func FindDataQualityEpisodes(readings []catalog.Reading, cfg DetectorConfig) []Episode {
	ordered := sortedReadings(readings)
	byTime := make(map[time.Time][]DataQualityFinding, len(ordered))
	for _, finding := range dataQualityFindings(readings, cfg) {
		byTime[finding.Timestamp] = append(byTime[finding.Timestamp], finding)
	}

	var episodes []Episode
	var run []catalog.Reading
	appendFinding := func(reading catalog.Reading) {
		run = append(run, reading)
	}
	flush := func() {
		if len(run) == 0 {
			return
		}
		if len(run) >= cfg.MinDataQualityReadings {
			episode := buildEpisode(run[0].MeterCode, EpisodeDataQuality, run, ProfileBefore(ordered, run[0].Timestamp, cfg), cfg)
			for _, reading := range run {
				episode.Findings = append(episode.Findings, byTime[reading.Timestamp]...)
			}
			episodes = append(episodes, episode)
		}
		run = nil
	}

	for _, reading := range ordered {
		if len(byTime[reading.Timestamp]) == 0 {
			if len(run) > 0 && reading.Timestamp.Sub(run[len(run)-1].Timestamp) > cfg.MaxEpisodeGap {
				flush()
			}
			continue
		}
		if len(run) > 0 && reading.Timestamp.Sub(run[len(run)-1].Timestamp) > cfg.MaxEpisodeGap {
			flush()
		}
		appendFinding(reading)
	}
	flush()

	for i := range episodes {
		scoreDataQualitySeverity(&episodes[i], len(ordered), cfg)
	}
	return episodes
}

// corroborate counts how many independent measurements moved with the episode.
// Consumption is one of them: the departure is a measurement, and the other three
// are what turn a departure into evidence.
func corroborate(all []catalog.Reading, episode *Episode, cfg DetectorConfig) {
	before := make([]catalog.Reading, 0, len(all))
	for _, reading := range all {
		if reading.Timestamp.Before(episode.Start) {
			before = append(before, reading)
		}
	}
	if len(before) == 0 {
		return
	}

	episode.Corroboration = 1.0 / float64(cfg.CorroboratingVariables)
	episode.Corroborating = append(episode.Corroborating, "consumption")

	consumptionRatio := ratio(episode.ActualKWh/float64(episode.ReadingCount()), meanOf(before, func(r catalog.Reading) float64 { return r.ConsumptionKWh }))
	meanCurrentBefore := meanOf(before, func(r catalog.Reading) float64 { return r.CurrentA })
	currentRatio := ratio(meanOf(episode.Readings, func(r catalog.Reading) float64 { return r.CurrentA }), meanCurrentBefore)
	if abs(currentRatio-consumptionRatio) <= cfg.CurrentTracksTolerance*abs(consumptionRatio) {
		episode.Corroborating = append(episode.Corroborating, "current")
		episode.Corroboration += 1.0 / float64(cfg.CorroboratingVariables)
	}

	// Power factor and voltage corroborate when they moved further than their own
	// dispersion: a shift inside the noise is not a second measurement.
	powerFactorBefore := newSpread(valuesOf(before, func(r catalog.Reading) float64 { return r.PowerFactor }), powerFactorScaleFloor)
	powerFactorAfter := Median(valuesOf(episode.Readings, func(r catalog.Reading) float64 { return r.PowerFactor }))
	if powerFactorBefore.sigma(powerFactorAfter) > cfg.CorroborationSigma {
		episode.Corroborating = append(episode.Corroborating, "power_factor")
		episode.Corroboration += 1.0 / float64(cfg.CorroboratingVariables)
	}

	voltageBefore := newSpread(valuesOf(before, func(r catalog.Reading) float64 { return r.VoltageV }), voltageScaleFloor)
	voltageAfter := Median(valuesOf(episode.Readings, func(r catalog.Reading) float64 { return r.VoltageV }))
	if voltageBefore.sigma(voltageAfter) > cfg.CorroborationSigma {
		episode.Corroborating = append(episode.Corroborating, "voltage")
		episode.Corroboration += 1.0 / float64(cfg.CorroboratingVariables)
	}
}

// scoreDataQualitySeverity records how much of a meter's history the corrupt
// readings cover, which is what makes a data-quality finding urgent.
func scoreDataQualitySeverity(episode *Episode, meterReadingCount int, cfg DetectorConfig) {
	affected := episode.ReadingCount()
	share := 0.0
	if meterReadingCount > 0 {
		share = float64(affected) / float64(meterReadingCount)
	}
	episode.Corroboration = min(1, float64(distinctVariables(episode.Findings))/3)
	episode.Corroborating = namesOfVariables(episode.Findings)
	episode.CorroboratingCount = affected
	episode.AffectedShare = share
	episode.HighByDataQualityVolume = affected >= cfg.DataQualityHighReadings || share >= cfg.DataQualityHighShare
}

// namesOfVariables lists the distinct variables a set of findings covers, so the
// explanation can say which measurements disagreed.
func namesOfVariables(findings []DataQualityFinding) []string {
	seen := map[Variable]bool{}
	var out []string
	for _, finding := range findings {
		if !seen[finding.Variable] {
			seen[finding.Variable] = true
			out = append(out, string(finding.Variable))
		}
	}
	sort.Strings(out)
	return out
}

func distinctVariables(findings []DataQualityFinding) int {
	seen := map[Variable]bool{}
	for _, finding := range findings {
		seen[finding.Variable] = true
	}
	return len(seen)
}

func valuesOf(readings []catalog.Reading, pick func(catalog.Reading) float64) []float64 {
	out := make([]float64, 0, len(readings))
	for _, reading := range readings {
		out = append(out, pick(reading))
	}
	return out
}

func meanOf(readings []catalog.Reading, pick func(catalog.Reading) float64) float64 {
	if len(readings) == 0 {
		return 0
	}
	total := 0.0
	for _, reading := range readings {
		total += pick(reading)
	}
	return total / float64(len(readings))
}

func ratio(after, before float64) float64 {
	if before == 0 {
		return 0
	}
	return after/before - 1
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// relativeTo is how far value sits from reference, as a fraction of reference.
func relativeTo(value, reference float64) float64 {
	if reference == 0 {
		return 0
	}
	return abs(value-reference) / abs(reference)
}
