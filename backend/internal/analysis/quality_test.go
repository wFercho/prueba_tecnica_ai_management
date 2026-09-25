package analysis

import (
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// balanced fills a reading's electrical values so that they agree with its
// consumption, which is what a working meter reports.
func balanced(reading catalog.Reading) catalog.Reading {
	reading.VoltageV = 220
	reading.PowerFactor = 0.93
	reading.CurrentA = reading.ConsumptionKWh * 1000 / (220 * 0.93)
	return reading
}

func TestQualityFlagsVoltageOutsideItsOwnBand(t *testing.T) {
	readings := synthetic("M-V", 14, flatThen(14, 10, 30, 30))
	readings[8*24+3].VoltageV = 241.2

	findings := DataQualityFindings(readings)

	voltage := findVariable(findings, VariableVoltage)
	if voltage == nil {
		t.Fatalf("got %+v, want a voltage finding", findings)
	}
	if voltage.Value != 241.2 {
		t.Errorf("Value = %v, want 241.2", voltage.Value)
	}
	if voltage.Sigma <= DefaultDetectorConfig().KVoltageSigma {
		t.Errorf("Sigma = %v, want past the threshold of %v", voltage.Sigma, DefaultDetectorConfig().KVoltageSigma)
	}
}

// findVariable returns the first finding about one variable, or nil.
func findVariable(findings []DataQualityFinding, variable Variable) *DataQualityFinding {
	for i := range findings {
		if findings[i].Variable == variable {
			return &findings[i]
		}
	}
	return nil
}

// distinctTimestamps counts the readings a set of findings covers, as opposed to
// the findings themselves: one corrupt reading often produces two.
func distinctTimestamps(findings []DataQualityFinding) int {
	seen := map[time.Time]bool{}
	for _, finding := range findings {
		seen[finding.Timestamp] = true
	}
	return len(seen)
}

func TestQualityFlagsCurrentThatDisagreesWithConsumption(t *testing.T) {
	readings := synthetic("M-B", 14, flatThen(14, 10, 30, 30))
	// Current doubles while the reported consumption stays where it was: the two
	// cannot both be true, and the consumption is not itself unusual, so the
	// meter, not the load, is at fault. The current is what betrays it.
	readings[9*24+3].CurrentA *= 2

	if findVariable(DataQualityFindings(readings), VariableCurrent) == nil {
		t.Error("got no current finding for a current double its reported consumption implies")
	}
}

func TestQualityDoesNotJudgeLoadVariablesWhenConsumptionChanged(t *testing.T) {
	// M-109's case: consumption doubled, current doubled, power factor fell to
	// 0.74. That is a heavier load drawing badly, not a broken meter, and the
	// power factor must not be reported as corrupt.
	readings := synthetic("M-109", 14, flatThen(14, 10, 30, 60))
	for i := 10 * 24; i < len(readings); i++ {
		readings[i].PowerFactor = 0.74
		readings[i].CurrentA = readings[i].ConsumptionKWh * 1000 / (220 * 0.74)
	}

	findings := DataQualityFindings(readings)

	for _, finding := range findings {
		if finding.Variable == VariablePowerFactor || finding.Variable == VariableCurrent {
			t.Errorf("unexpected %s finding at %v: a load change is not a metering fault",
				finding.Variable, finding.Timestamp)
		}
	}
}

func TestQualityEpisodeJoinsFindingsThreeHoursApart(t *testing.T) {
	readings := synthetic("M-112", 14, flatThen(14, 10, 30, 30))
	for _, hour := range []int{0, 3, 6, 9, 12} {
		readings[11*24+hour].VoltageV = 203
	}

	episodes := FindDataQualityEpisodes(readings, DefaultDetectorConfig())

	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	if got, want := episodes[0].ReadingCount(), 5; got != want {
		t.Errorf("ReadingCount = %d, want %d", got, want)
	}
	if episodes[0].Kind != EpisodeDataQuality {
		t.Errorf("Kind = %q, want %q", episodes[0].Kind, EpisodeDataQuality)
	}
}

func TestQualityEpisodeRejectsIsolatedFinding(t *testing.T) {
	readings := synthetic("M-ONE", 14, flatThen(14, 10, 30, 30))
	readings[8*24+3].VoltageV = 203

	if episodes := FindDataQualityEpisodes(readings, DefaultDetectorConfig()); len(episodes) != 0 {
		t.Fatalf("got %d episodes, want 0: one implausible reading is not a finding", len(episodes))
	}
}

func TestQualityIsSilentOnAConsistentMeter(t *testing.T) {
	readings := synthetic("M-OK", 14, flatThen(14, 10, 30, 30))
	for i := range readings {
		readings[i] = balanced(readings[i])
	}

	if findings := DataQualityFindings(readings); len(findings) != 0 {
		t.Fatalf("got %d findings on a consistent meter, want 0: %+v", len(findings), findings[0])
	}
}

func TestDataQualityEpisodeReportsConsumptionStayedFlat(t *testing.T) {
	readings := synthetic("M-FLATQ", 14, flatThen(14, 10, 30, 30))
	for _, hour := range []int{0, 3, 6, 9} {
		readings[11*24+hour].VoltageV = 203
	}

	episodes := FindDataQualityEpisodes(readings, DefaultDetectorConfig())

	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	if abs(episodes[0].Deviation) > 0.05 {
		t.Errorf("Deviation = %v, want the consumption to read as flat", episodes[0].Deviation)
	}
	if got := distinctTimestamps(episodes[0].Findings); got != 4 {
		t.Errorf("got findings on %d readings, want 4", got)
	}
}

func TestCorroborationCountsIndependentMeasurementsThatMoved(t *testing.T) {
	readings := synthetic("M-CORR", 14, flatThen(14, 10, 30, 30))
	for i := 10 * 24; i < len(readings); i++ {
		readings[i].ConsumptionKWh = 60
		readings[i].CurrentA = 60 * 1000 / (220 * 0.93)
		readings[i].PowerFactor = 0.74
	}

	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	corroborated := episodes[0].Corroborating
	if len(corroborated) != 3 {
		t.Fatalf("corroborating = %v, want consumption, current and power factor", corroborated)
	}
	if episodes[0].Corroboration < 0.75 {
		t.Errorf("Corroboration = %v, want at least 0.75 of four independent variables", episodes[0].Corroboration)
	}
}

func TestCorroborationIsPartialWhenOnlyCurrentMoves(t *testing.T) {
	// M-104's case: current tracks the new consumption, but the power factor and
	// the voltage stay inside their own noise, so only one measurement moved.
	readings := synthetic("M-PART", 14, flatThen(14, 10, 30, 30))
	for i := 10 * 24; i < len(readings); i++ {
		readings[i].ConsumptionKWh = 44
		readings[i].CurrentA = 44 * 1000 / (220 * 0.93)
	}

	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	if episodes[0].Corroboration > 0.5 {
		t.Errorf("Corroboration = %v (%v), want at most half the variables",
			episodes[0].Corroboration, episodes[0].Corroborating)
	}
}

func TestCorrelateIgnoresEventsForOtherMeters(t *testing.T) {
	readings := synthetic("M-OWN", 14, flatThen(14, 10, 30, 44))
	episodes := FindConsumptionEpisodes(readings, DefaultDetectorConfig())
	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	events := []catalog.OperationalEvent{{
		MeterCode:   "M-OTHER",
		Timestamp:   episodes[0].Start,
		Type:        catalog.EventTypeOperationalChange,
		Description: "somewhere else",
	}}

	corroborate(readings, &episodes[0], DefaultDetectorConfig())
	Correlate(events, &episodes[0], DefaultDetectorConfig())

	if len(episodes[0].CorrelatedEvents) != 0 {
		t.Fatalf("got %d correlated events, want 0: the event belongs to another meter",
			len(episodes[0].CorrelatedEvents))
	}
}

func TestUnknownEventDoesNotExplainAnEpisode(t *testing.T) {
	// M-109's event row says "No operational event reported". It must be kept as
	// evidence and refused as an explanation.
	episodes := []Episode{{CorrelatedEvents: []catalog.OperationalEvent{{
		MeterCode:   "M-109",
		Timestamp:   time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC),
		Type:        catalog.EventTypeUnknown,
		Description: "No operational event reported",
	}}}}

	if _, ok := episodes[0].ExplainingEvent(); ok {
		t.Fatal("an UNKNOWN event explained the episode; it reports the absence of a cause")
	}
}

func TestQualityIgnoresPowerFactorThatIsNoisyButPlausible(t *testing.T) {
	// A meter's power factor wanders with the load. 0.88 is a real measurement of
	// a real load, so however many sigma it sits from its own history it is not a
	// fault, and calling it one would flood the list with noise.
	readings := synthetic("M-PFN", 14, flatThen(14, 10, 30, 30))
	readings[9*24+3].PowerFactor = 0.88

	if findings := DataQualityFindings(readings); findVariable(findings, VariablePowerFactor) != nil {
		t.Errorf("got a power-factor finding for a plausible %v", 0.88)
	}
}

func TestQualityFlagsPowerFactorNoMeterWouldReport(t *testing.T) {
	// Below MinPlausiblePowerFactor the meter is reporting something no healthy
	// installation produces, however quiet its own history is.
	readings := synthetic("M-PFB", 14, flatThen(14, 10, 30, 30))
	readings[9*24+3].PowerFactor = 0.62

	if findVariable(DataQualityFindings(readings), VariablePowerFactor) == nil {
		t.Error("got no power-factor finding for an implausible 0.62")
	}
}

func TestQualityIgnoresEnergyBalanceWithinMeteringError(t *testing.T) {
	// Current transformers and phase imbalance routinely put the electrical
	// estimate tens of percent away from the billed consumption. Only an
	// imbalance past MaxEnergyBalanceError is evidence of a fault.
	readings := synthetic("M-EB", 14, flatThen(14, 10, 30, 30))
	readings[9*24+3].CurrentA *= 1.4

	if findVariable(DataQualityFindings(readings), VariableEnergyBalance) != nil {
		t.Error("got an energy-balance finding for a 40% current difference, which is ordinary metering error")
	}
}

func TestQualityFlagsEnergyBalancePastTolerance(t *testing.T) {
	readings := synthetic("M-EBF", 14, flatThen(14, 10, 30, 30))
	readings[9*24+3].CurrentA *= 2.5

	if findVariable(DataQualityFindings(readings), VariableEnergyBalance) == nil {
		t.Errorf("got no energy-balance finding for a current %vx the reported consumption implies", 2.5)
	}
}
