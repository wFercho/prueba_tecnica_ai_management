package analysis

import (
	"math"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
)

// AnomalyType is the fixed four-value vocabulary an episode is classified into.
// It is not an open label.
type AnomalyType string

const (
	// AnomalyReal is a change in consumption the system cannot account for.
	AnomalyReal AnomalyType = "REAL_ANOMALY"
	// AnomalyDataQuality is a fault of the meter or its reporting: the reported
	// electrical values are not physically consistent with the reported
	// consumption.
	AnomalyDataQuality AnomalyType = "DATA_QUALITY"
	// AnomalyExplainable is a real change that a known operational event
	// accounts for.
	AnomalyExplainable AnomalyType = "EXPLAINABLE"
	// AnomalyFalsePositive is a change a known operational event fully accounts
	// for, so nothing needs to be done. It is stored rather than suppressed: the
	// system examined it and decided (ADR-0002).
	AnomalyFalsePositive AnomalyType = "FALSE_POSITIVE"
)

// Severity is how much attention an anomaly warrants, judged independently of how
// confident the detector is. A high-severity finding can carry low confidence and
// must still be shown.
type Severity string

const (
	SeverityHigh   Severity = "HIGH"
	SeverityMedium Severity = "MEDIUM"
	SeverityLow    Severity = "LOW"
)

// ConfidenceBand is the categorical reading of a confidence score, derived from
// it rather than judged separately, so there is one number and one formula.
type ConfidenceBand string

const (
	BandHigh ConfidenceBand = "HIGH"
	BandMid  ConfidenceBand = "MEDIUM"
	BandLow  ConfidenceBand = "LOW"
)

// bandCutoffs are the score at which a confidence reads as high and as medium.
const (
	bandHighCutoff = 0.75
	bandMidCutoff  = 0.50
)

// Band is the categorical reading of a confidence score.
func Band(confidence float64) ConfidenceBand {
	switch {
	case confidence >= bandHighCutoff:
		return BandHigh
	case confidence >= bandMidCutoff:
		return BandMid
	default:
		return BandLow
	}
}

// Evidence is the four terms a confidence score is built from. They are always
// exposed alongside the score, so the number can be traced to the readings that
// produced it (ADR-0005).
type Evidence struct {
	// Deviation is how far the episode departed from the meter's own baseline.
	Deviation float64 `json:"deviation"`
	// EventMatch is how well the operational event evidence supports the
	// classification. An event that explains the episode scores full marks; the
	// documented absence of any event also scores full marks, because it is
	// positive evidence for a real anomaly; a meter nobody reported anything for
	// scores least.
	EventMatch float64 `json:"event_match"`
	// Corroboration is the share of independent measurements that moved with the
	// episode.
	Corroboration float64 `json:"corroboration"`
	// Persistence is how long the episode lasted, saturating at
	// DetectorConfig.PersistenceSaturation readings.
	Persistence float64 `json:"persistence"`
}

// Anomaly is one episode, classified and stored as a single row carrying a window,
// a type, a severity, a confidence and a recommended action. A meter under two
// separate episodes has two anomalies.
type Anomaly struct {
	// Anomaly means this candidate episode was examined, including an explained
	// false positive; it is not a claim that the operator must escalate it.
	Anomaly     bool      `json:"anomaly"`
	ID          int64     `json:"id,omitempty"`
	MeterCode   string    `json:"meter_id"`
	RunID       int64     `json:"run_id,omitempty"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	// AffectedReadings is how many readings the episode covers.
	AffectedReadings int         `json:"affected_readings"`
	Type             AnomalyType `json:"type"`
	Severity         Severity    `json:"severity"`
	// Confidence is an uncalibrated evidence score in [0,1]. It is not a
	// probability and must never be presented as one.
	Confidence float64 `json:"confidence"`
	// ConfidenceBand is Confidence read categorically.
	ConfidenceBand ConfidenceBand `json:"confidence_band"`
	// ConfidenceBasis is the four terms Confidence was built from.
	ConfidenceBasis Evidence `json:"confidence_basis"`
	// DeviationPercent is the episode's departure from baseline, for display.
	DeviationPercent float64 `json:"deviation_percent"`
	// ActualKWh and BaselineKWh are the two sides of that comparison.
	ActualKWh   float64 `json:"actual_kwh"`
	BaselineKWh float64 `json:"baseline_kwh"`
	// Corroborating lists the independent measurements that moved.
	Corroborating []string `json:"corroborating"`
	// DeviationSeries is the episode's own hourly readings against the baseline
	// that judged them, and nothing outside the window. It is persisted with the
	// anomaly because the baseline is derived state: recomputing it at read time
	// would show a different series than the one the detector acted on (ADR-0007).
	DeviationSeries []Point `json:"deviation_series,omitempty"`
	// Findings are the data-quality findings behind a DATA_QUALITY anomaly.
	Findings []DataQualityFinding `json:"findings,omitempty"`
	// CorrelatedEvent is the operational event that accounts for the episode, or
	// the one that reports there was none.
	CorrelatedEvent *EventSummary `json:"correlated_event,omitempty"`
	// Reason and RecommendedAction are prose. Who wrote them is ExplanationSource.
	Reason            string `json:"reason"`
	RecommendedAction string `json:"recommended_action"`
	// DetectedBy names the rule that fired, so a finding can be traced to code.
	DetectedBy string `json:"detected_by"`
	// ExplanationSource is who wrote Reason and RecommendedAction, and
	// ExplanationStatus is whether the narrator has answered yet. Both are set by
	// the store, not by the detector: the detector only ever writes rules prose,
	// and saying so is what makes a surviving template identifiable.
	ExplanationSource catalog.ExplanationSource `json:"explanation_source"`
	ExplanationStatus catalog.ExplanationStatus `json:"explanation_status"`
	// Status is the operator's decision about this anomaly, never the detector's.
	Status catalog.AnomalyStatus `json:"status"`
}

// Point is one hour inside an episode, with what the meter reported and what its
// own baseline expected. This is the evidence the narrator receives: the change
// as a series, so it can say when it happened rather than only that it did.
type Point struct {
	Timestamp   time.Time `json:"timestamp"`
	ActualKWh   float64   `json:"actual_kwh"`
	BaselineKWh float64   `json:"baseline_kwh"`
	Deviation   float64   `json:"deviation"`
	VoltageV    float64   `json:"voltage_v"`
	CurrentA    float64   `json:"current_a"`
	PowerFactor float64   `json:"power_factor"`
}

// EventSummary is the operational event an anomaly is read against.
type EventSummary struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	// Explains is whether the event accounts for the anomaly. An event of type
	// UNKNOWN never does.
	Explains bool `json:"explains"`
}

// Classify turns an episode into an anomaly: what kind of thing it is, how much
// attention it warrants, and how reproducible the evidence for that is.
//
// The rules are ordered and each one is a claim about the world, not a score:
//
//  1. Readings that disagree with their own physics, on a meter whose consumption
//     never moved, are a metering fault however the meter is otherwise behaving.
//  2. A drop a scheduled outage accounts for needs nothing done: it is stored as a
//     false positive rather than suppressed, because "we looked and dismissed it"
//     and "we never saw it" are different facts.
//  3. A change a known operational event accounts for is real but explained.
//  4. Everything else is real, and its severity follows how far it went and
//     whether other independent measurements moved with it.
func Classify(episode Episode, cfg DetectorConfig) Anomaly {
	anomaly := Anomaly{
		Anomaly:           true,
		MeterCode:         string(episode.MeterCode),
		WindowStart:       episode.Start.UTC(),
		WindowEnd:         episode.End.UTC(),
		AffectedReadings:  episode.ReadingCount(),
		ActualKWh:         round(episode.ActualKWh, 2),
		BaselineKWh:       round(episode.BaselineKWh, 2),
		DeviationPercent:  round(episode.Deviation*100, 1),
		Corroborating:     episode.Corroborating,
		DeviationSeries:   episode.DeviationSeries(),
		ExplanationSource: catalog.SourceRules,
		ExplanationStatus: catalog.ExplanationPending,
		Status:            catalog.StatusOpen,
		Findings:          episode.Findings,
		DetectedBy:        string(episode.Kind) + "/hourly-profile+mad",
	}
	if event, ok := episode.ExplainingEvent(); ok {
		anomaly.CorrelatedEvent = &EventSummary{
			Timestamp:   event.Timestamp.UTC(),
			Type:        string(event.Type),
			Description: event.Description,
			Explains:    true,
		}
	} else if len(episode.CorrelatedEvents) > 0 {
		// Reported and explicitly empty: the absence of a cause is the evidence.
		event := episode.CorrelatedEvents[0]
		anomaly.CorrelatedEvent = &EventSummary{
			Timestamp:   event.Timestamp.UTC(),
			Type:        string(event.Type),
			Description: event.Description,
			Explains:    false,
		}
	}

	anomaly.Type = classifyType(episode)
	anomaly.Severity = classifySeverity(episode, anomaly.Type, cfg)
	anomaly.ConfidenceBasis = evidenceTerms(episode, anomaly.Type, cfg)
	anomaly.Confidence = round(scoreConfidence(anomaly.ConfidenceBasis, cfg), 3)
	anomaly.ConfidenceBand = Band(anomaly.Confidence)
	anomaly.Reason, anomaly.RecommendedAction = explain(anomaly)
	return anomaly
}

func classifyType(episode Episode) AnomalyType {
	if episode.Kind == EpisodeDataQuality &&
		abs(episode.Deviation) < DefaultDetectorConfig().MaxFlatConsumptionDeviation {
		return AnomalyDataQuality
	}
	if event, ok := episode.ExplainingEvent(); ok {
		if event.Type == catalog.EventTypeScheduledOutage || event.Type == catalog.EventTypeMaintenance {
			return AnomalyFalsePositive
		}
		return AnomalyExplainable
	}
	return AnomalyReal
}

func classifySeverity(episode Episode, kind AnomalyType, cfg DetectorConfig) Severity {
	switch kind {
	case AnomalyDataQuality:
		if episode.HighByDataQualityVolume {
			return SeverityHigh
		}
		return SeverityMedium
	case AnomalyFalsePositive:
		// Something known explained it, so there is nothing to attend to. Severity
		// is about attention, not about how large the change was.
		return SeverityLow
	case AnomalyExplainable:
		return SeverityMedium
	default:
		magnitude := abs(episode.Deviation)
		switch {
		case magnitude >= cfg.HighDeviation:
			return SeverityHigh
		case magnitude >= cfg.HighDeviationWithCorroboration && episode.Corroboration >= cfg.HighCorroboration:
			return SeverityHigh
		case magnitude >= cfg.MediumDeviation:
			return SeverityMedium
		default:
			return SeverityLow
		}
	}
}

// evidenceTerms measures the four inputs to confidence.
func evidenceTerms(episode Episode, kind AnomalyType, cfg DetectorConfig) Evidence {
	return Evidence{
		Deviation:     deviationTerm(episode, kind, cfg),
		EventMatch:    eventMatchTerm(episode, kind),
		Corroboration: episode.Corroboration,
		Persistence:   persistenceTerm(episode, cfg),
	}
}

// deviationTerm saturates so that it measures how far the episode went without
// simply restating the severity thresholds.
//
// A data-quality episode is scored on its findings' physical departures rather
// than on its consumption, because its consumption barely moved — that flatness
// is the finding, not a weak deviation — and because scoring it on sigma would
// put a meter with a merely tight history above a real 110% load change, which
// is the opposite of the ranking the dashboard needs.
//
// The departures are averaged, not maximised: an episode is a story about many
// readings, and letting its single most absurd reading represent all of them
// would rank a meter with one wild number above a genuine 110% load change.
func deviationTerm(episode Episode, kind AnomalyType, cfg DetectorConfig) float64 {
	if kind == AnomalyDataQuality {
		if len(episode.Findings) == 0 {
			return 0
		}
		total := 0.0
		for _, finding := range episode.Findings {
			total += finding.RelativeDeviation
		}
		return 1 - math.Exp(-(total/float64(len(episode.Findings)))*100/cfg.DeviationSaturation)
	}
	return 1 - math.Exp(-abs(episode.Deviation)*100/cfg.DeviationSaturation)
}

// eventMatchTerm scores the operational-event evidence for the classification the
// detector actually made.
func eventMatchTerm(episode Episode, kind AnomalyType) float64 {
	explaining := false
	reportedNone := false
	for _, event := range episode.CorrelatedEvents {
		switch {
		case event.Explains():
			explaining = true
		case event.Type == catalog.EventTypeUnknown:
			reportedNone = true
		}
	}
	if kind == AnomalyDataQuality {
		// A metering fault is evidenced by the readings, not by anyone's report,
		// so a report only confirms it and silence costs a little.
		if explaining {
			return 1
		}
		return 0.5
	}
	if explaining {
		return 1
	}
	if reportedNone {
		// Somebody looked and reported that nothing happened. That is stronger
		// evidence for a real anomaly than an empty event table.
		return 1
	}
	return 0.3
}

func persistenceTerm(episode Episode, cfg DetectorConfig) float64 {
	if cfg.PersistenceSaturation <= 0 {
		return 0
	}
	return min(1, float64(episode.ReadingCount())/float64(cfg.PersistenceSaturation))
}

// scoreConfidence combines the four terms into one score in [0,1] as a weighted
// geometric mean.
//
// A geometric mean rather than an arithmetic one, because a term near zero should
// pull the whole score down — an episode with no corroboration and no persistence
// is weakly evidenced however large it is — and because a weighted arithmetic mean
// of four terms would let three good terms hide one missing one.
func scoreConfidence(terms Evidence, cfg DetectorConfig) float64 {
	weights := cfg.Weights
	score := pow(terms.Deviation, weights.Deviation) *
		pow(terms.EventMatch, weights.EventMatch) *
		pow(terms.Corroboration, weights.Corroboration) *
		pow(terms.Persistence, weights.Persistence)
	return min(1, max(0, score))
}

func pow(base, exponent float64) float64 { return math.Pow(base, exponent) }

func round(v float64, places int) float64 {
	factor := math.Pow(10, float64(places))
	return math.Round(v*factor) / factor
}
