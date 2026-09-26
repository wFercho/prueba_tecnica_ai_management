// Package service is the application's use cases: run the detector, explain what
// it found, and answer the questions the dashboard asks.
//
// It is the layer where the run lifecycle lives, and its ordering is the point.
// Detection and its deterministic explanations are persisted before anything
// reaches a network, the run is then COMPLETED, and only then does narration
// start. A narrator that is slow, rate-limited or absent can degrade the wording
// of findings that are already durable, and can do nothing else (ADR-0006).
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/narrate"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

// narrationTimeout bounds one background narration pass, so a narrator that hangs
// cannot hold a goroutine for the life of the process.
const narrationTimeout = 2 * time.Minute

// Service answers the API's questions and runs the analysis pipeline.
type Service struct {
	store    store.Store
	narrator narrate.Narrator
	cfg      analysis.DetectorConfig
	log      *slog.Logger
	// narration tracks the background narration so it can be waited on at shutdown,
	// and waited on by tests. The API never waits on it.
	narration sync.WaitGroup
}

// New returns a service. A nil narrator becomes the rules narrator, so a
// deployment with no model key behaves like one where every call failed: the same
// findings, with plainer words.
func New(store store.Store, narrator narrate.Narrator, cfg analysis.DetectorConfig) *Service {
	if narrator == nil {
		narrator = narrate.Rules{}
	}
	return &Service{store: store, narrator: narrator, cfg: cfg, log: slog.Default()}
}

// SetLogger replaces the logger.
func (s *Service) SetLogger(log *slog.Logger) { s.log = log }

// WaitForNarration blocks until every in-flight narration has finished. It is for
// shutdown and tests, not for request handling.
func (s *Service) WaitForNarration() { s.narration.Wait() }

// Analyze runs the detector over the stored readings, persists what it found with
// its deterministic explanations, completes the run, and starts narration in the
// background.
//
// The run is COMPLETED when this returns. Per ADR-0008 that state means detection
// is done and narration may still be arriving, which is why the run carries
// separate progress counts rather than a second state.
func (s *Service) Analyze(ctx context.Context) (store.Run, error) {
	from, to, err := s.store.ReadingWindow(ctx)
	if err != nil {
		return store.Run{}, fmt.Errorf("nothing to analyse: %w", err)
	}

	runID, err := s.store.StartRun(ctx, from, to)
	if err != nil {
		return store.Run{}, err
	}

	anomalies, err := s.detect(ctx)
	if err != nil {
		return s.abandon(ctx, runID, err)
	}
	_, rulesOnly := s.narrator.(narrate.Rules)
	if rulesOnly {
		for i := range anomalies {
			anomalies[i].ExplanationStatus = catalog.ExplanationReady
		}
	}

	// The rules explanation is written here, before the caller is told the run
	// exists. Everything after this point is an improvement, never a prerequisite.
	if err := s.store.SaveAnomalies(ctx, runID, anomalies); err != nil {
		return s.abandon(ctx, runID, err)
	}
	if err := s.store.FinishRun(ctx, runID, store.RunCompleted, len(anomalies), ""); err != nil {
		return store.Run{}, err
	}

	run, err := s.store.Run(ctx, runID)
	if err != nil {
		return store.Run{}, err
	}

	// Narration outlives the request that triggered it. A client that closed the
	// connection mid-run must not silently cancel the explanations it asked for.
	if !rulesOnly {
		s.startNarration(runID)
	}
	return run, nil
}

// detect runs the deterministic detector over everything stored.
func (s *Service) detect(ctx context.Context) ([]analysis.Anomaly, error) {
	readings, err := s.store.AllReadings(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the readings: %w", err)
	}
	events, err := s.store.Events(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the events: %w", err)
	}
	return analysis.Analyze(readings, events, s.cfg), nil
}

// abandon records that a run failed, so a run is never left RUNNING forever, and
// returns the original error rather than the bookkeeping one.
func (s *Service) abandon(ctx context.Context, runID int64, cause error) (store.Run, error) {
	if err := s.store.FinishRun(ctx, runID, store.RunFailed, 0, cause.Error()); err != nil {
		s.log.Error("could not record the failed run", "run", runID, "error", err)
	}
	return store.Run{}, cause
}

func (s *Service) startNarration(runID int64) {
	s.narration.Add(1)
	go func() {
		defer s.narration.Done()
		// Detached from any request context on purpose; see Analyze.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), narrationTimeout)
		defer cancel()
		s.narrateRun(ctx, runID)
	}()
}

func (s *Service) narrateRun(ctx context.Context, runID int64) {
	anomalies, err := s.store.RunAnomalies(ctx, runID)
	if err != nil {
		s.log.Warn("could not read the anomalies to narrate", "run", runID, "error", err)
		return
	}
	for _, anomaly := range anomalies {
		s.narrateOne(ctx, anomaly)
	}
}

func (s *Service) narrateOne(ctx context.Context, anomaly analysis.Anomaly) {
	evidence, err := s.evidenceFor(ctx, anomaly)
	if err != nil {
		s.log.Warn("could not gather evidence", "anomaly", anomaly.ID, "error", err)
		return
	}

	narrative, err := s.narrator.Narrate(ctx, evidence)
	if err != nil {
		// Recorded and nothing more: the rules prose is already stored, so the row
		// is still explained.
		s.log.Warn("narration failed, keeping the deterministic explanation",
			"anomaly", anomaly.ID, "error", err)
		s.markFailed(ctx, anomaly.ID, err)
		return
	}
	if narrative.Reason == "" || narrative.Action == "" {
		s.log.Warn("the narrator returned nothing usable, keeping the deterministic explanation",
			"anomaly", anomaly.ID)
		s.markFailed(ctx, anomaly.ID, errors.New("the narrator returned no explanation"))
		return
	}

	source := narrative.Source
	if source == "" {
		// Anything other than the template is the model's prose, so an unlabelled
		// narrator is credited as the model rather than silently filed as rules.
		source = catalog.SourceLLM
	}
	if source != catalog.SourceRules && (!narrate.IsSpanishProse(narrative.Reason) || !narrate.IsSpanishProse(narrative.Action)) {
		s.markFailed(ctx, anomaly.ID, errors.New("narration is not in Spanish"))
		return
	}
	if err := s.store.Narrate(ctx, anomaly.ID, source, narrative.Reason, narrative.Action); err != nil {
		s.log.Error("could not store the narration", "anomaly", anomaly.ID, "error", err)
	}
}

func (s *Service) markFailed(ctx context.Context, anomalyID int64, cause error) {
	if err := s.store.MarkNarrationFailed(ctx, anomalyID); err != nil {
		s.log.Error("could not record the narration failure", "anomaly", anomalyID, "error", err)
	}
}

// evidenceFor assembles what the narrator is allowed to see: the episode, its
// baseline, the event it was read against, and how much data the meter has. The
// meter's history is deliberately not included (ADR-0007).
func (s *Service) evidenceFor(ctx context.Context, anomaly analysis.Anomaly) (narrate.Evidence, error) {
	meter, err := s.store.Meter(ctx, anomaly.MeterCode)
	if err != nil {
		return narrate.Evidence{}, err
	}
	detail, err := s.store.Anomaly(ctx, anomaly.ID)
	if err != nil {
		return narrate.Evidence{}, err
	}
	readings, err := s.store.Readings(ctx, anomaly.MeterCode, time.Time{}, time.Time{})
	if err != nil {
		return narrate.Evidence{}, err
	}
	return narrate.Evidence{
		Meter:    meter,
		Anomaly:  detail,
		Series:   detail.DeviationSeries,
		Event:    detail.CorrelatedEvent,
		Readings: len(readings),
	}, nil
}

// Anomalies returns the latest run's findings, in the order an operator reads them.
func (s *Service) Anomalies(ctx context.Context) ([]analysis.Anomaly, error) {
	return s.store.Anomalies(ctx)
}

// AnomalyView is a finding together with the meter's name and location, which the
// investigation view needs and the anomaly row does not carry.
type AnomalyView struct {
	analysis.Anomaly
	MeterName     string `json:"meter_name"`
	MeterLocation string `json:"meter_location"`
}

// Anomaly returns one finding with its evidence and its meter's identity.
func (s *Service) Anomaly(ctx context.Context, id int64) (AnomalyView, error) {
	anomaly, err := s.store.Anomaly(ctx, id)
	if err != nil {
		return AnomalyView{}, err
	}
	current, err := s.store.LatestCompletedRun(ctx)
	if err != nil || anomaly.RunID != current.ID {
		return AnomalyView{}, store.ErrNotFound
	}
	meter, err := s.store.Meter(ctx, anomaly.MeterCode)
	if err != nil {
		return AnomalyView{}, err
	}
	return AnomalyView{Anomaly: anomaly, MeterName: meter.Name, MeterLocation: meter.Location}, nil
}

// Run returns one run's state and progress: the run itself, and how many of its
// findings have their final explanation yet.
func (s *Service) Run(ctx context.Context, id int64) (RunProgress, error) {
	run, err := s.store.Run(ctx, id)
	if err != nil {
		return RunProgress{}, err
	}
	anomalies, err := s.store.RunAnomalies(ctx, id)
	if err != nil {
		return RunProgress{}, err
	}
	return progressOf(run, anomalies), nil
}

// progressOf derives the progress counts from the requested run's own findings.
func progressOf(run store.Run, anomalies []analysis.Anomaly) RunProgress {
	progress := RunProgress{
		ID:           run.ID,
		State:        run.State,
		StartedAt:    run.StartedAt,
		FinishedAt:   run.FinishedAt,
		AnomalyCount: run.AnomalyCount,
		WindowStart:  run.WindowStart,
		WindowEnd:    run.WindowEnd,
		Error:        run.Error,
	}
	for _, anomaly := range anomalies {
		switch anomaly.ExplanationStatus {
		case catalog.ExplanationPending:
			progress.Narrating++
		case catalog.ExplanationReady:
			progress.Explained++
		case catalog.ExplanationFailed:
			progress.Failed++
		}
	}
	return progress
}

// Health is a meter's condition, derived on read from its open anomalies and
// never stored, so the importer, the detector and the UI cannot each hold a
// different opinion about the same meter (ADR-0009).
type Health string

const (
	HealthHealthy      Health = "HEALTHY"
	HealthAlert        Health = "ALERT"
	HealthCritical     Health = "CRITICAL"
	HealthUnassessed   Health = "UNASSESSED"
	HealthInsufficient Health = "INSUFFICIENT_DATA"
)

// MeterView is a meter's row on the dashboard: its identity, its totals, and what
// is currently open against it.
type MeterView struct {
	MeterID  string `json:"meter_id"`
	Name     string `json:"name"`
	Location string `json:"location"`
	Health   Health `json:"health"`
	// TotalKWh is consumption over the analysed window. The dashboard leads with
	// it, and it is a sum of what was read, not a projection.
	TotalKWh         float64  `json:"total_kwh"`
	VariationPercent *float64 `json:"variation_percent"`
	Readings         int      `json:"readings"`
	// OpenAnomalies counts findings still to be dealt with, which is what makes
	// health actionable rather than a label.
	OpenAnomalies int `json:"open_anomalies"`
	// WorstSeverity is the highest severity among the open findings, empty when
	// there are none. It is what the health is derived from, shown so the
	// derivation is visible.
	WorstSeverity analysis.Severity `json:"worst_severity,omitempty"`
}

// RunProgress is a run together with how far its explanations have got.
//
// It carries the two facts the API documentation says must be read together: the
// run is COMPLETED, and some explanations may still be arriving. There is no second
// state for "still narrating", because a completed run with pending explanations is
// a normal thing to poll (ADR-0008).
type RunProgress struct {
	ID         int64          `json:"id"`
	State      store.RunState `json:"state"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at"`
	// AnomalyCount is what the detector found.
	AnomalyCount int `json:"anomaly_count"`
	// Narrating is how many of those are still PENDING an explanation, Explained is
	// how many have the narrator's, and Failed is how many kept the template because
	// the narrator did not answer. A client has to be able to tell those three
	// apart: "still thinking" and "the model is unreachable" call for different
	// things on screen.
	Narrating   int       `json:"narrating"`
	Explained   int       `json:"explained"`
	Failed      int       `json:"failed"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	Error       string    `json:"error,omitempty"`
}

// Dashboard is everything the landing page shows.
type Dashboard struct {
	LastRun           *RunProgress `json:"last_run"`
	LastSuccessfulRun *RunProgress `json:"last_successful_run"`
	Meters            []MeterView  `json:"meters"`
	// AnomalyCounts is how many findings of each type the last run produced,
	// including the false positive. It is the count of what was examined, which is
	// not the count of what needs attention (ADR-0002).
	AnomalyCounts map[analysis.AnomalyType]int `json:"anomaly_counts"`
	// NeedsAttention is how many findings are HIGH or MEDIUM and still open. It
	// is what the dashboard leads with, and it is deliberately not the total.
	NeedsAttention     int                             `json:"needs_attention"`
	HighPriority       int                             `json:"high_priority"`
	ConfidenceBands    map[analysis.ConfidenceBand]int `json:"confidence_bands"`
	PriorityConfidence analysis.ConfidenceBand         `json:"priority_confidence,omitempty"`
	TotalKWh           float64                         `json:"total_kwh"`
}

// Dashboard assembles the landing page from stored state.
//
// It reads the anomalies once and derives health, the counts and the totals from
// that single read, so the page cannot show a meter's health disagreeing with its
// open count.
func (s *Service) Dashboard(ctx context.Context) (Dashboard, error) {
	meters, err := s.store.Meters(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	totals, err := s.store.ConsumptionTotals(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	readings, err := s.store.AllReadings(ctx)
	if err != nil {
		return Dashboard{}, err
	}
	anomalies, err := s.store.Anomalies(ctx)
	if err != nil {
		return Dashboard{}, err
	}

	counts := map[analysis.AnomalyType]int{}
	openByMeter := map[string]int{}
	worstByMeter := map[string]analysis.Anomaly{}
	hasRealHigh := map[string]bool{}
	hasAlert := map[string]bool{}
	needsAttention := 0
	highPriority := 0
	bands := map[analysis.ConfidenceBand]int{}
	for _, anomaly := range anomalies {
		counts[anomaly.Type]++
		bands[anomaly.ConfidenceBand]++
		if anomaly.Status != catalog.StatusOpen {
			continue
		}
		openByMeter[anomaly.MeterCode]++
		current := worstByMeter[anomaly.MeterCode]
		if severityRank(anomaly.Severity) < severityRank(current.Severity) ||
			(severityRank(anomaly.Severity) == severityRank(current.Severity) &&
				typeRank(anomaly.Type) < typeRank(current.Type)) {
			worstByMeter[anomaly.MeterCode] = anomaly
		}
		if anomaly.Type == analysis.AnomalyReal && anomaly.Severity == analysis.SeverityHigh {
			hasRealHigh[anomaly.MeterCode] = true
		} else if anomaly.Severity == analysis.SeverityHigh || anomaly.Severity == analysis.SeverityMedium {
			hasAlert[anomaly.MeterCode] = true
		}
		if anomaly.Severity == analysis.SeverityHigh {
			highPriority++
		}
		if anomaly.Severity != analysis.SeverityLow {
			needsAttention++
		}
	}

	readingsByMeter := map[string]int{}
	seriesByMeter := map[string][]catalog.Reading{}
	for _, reading := range readings {
		readingsByMeter[string(reading.MeterCode)]++
		seriesByMeter[string(reading.MeterCode)] = append(seriesByMeter[string(reading.MeterCode)], reading)
	}
	completed, completedErr := s.store.LatestCompletedRun(ctx)
	if completedErr != nil && !errors.Is(completedErr, store.ErrNotFound) {
		return Dashboard{}, completedErr
	}

	view := Dashboard{
		AnomalyCounts:   counts,
		Meters:          make([]MeterView, 0, len(meters)),
		NeedsAttention:  needsAttention,
		HighPriority:    highPriority,
		ConfidenceBands: bands,
	}
	if len(anomalies) > 0 {
		view.PriorityConfidence = anomalies[0].ConfidenceBand
	}
	affectedByMeter := map[string]map[time.Time]bool{}
	for _, anomaly := range anomalies {
		if affectedByMeter[anomaly.MeterCode] == nil {
			affectedByMeter[anomaly.MeterCode] = map[time.Time]bool{}
		}
		for _, point := range anomaly.DeviationSeries {
			affectedByMeter[anomaly.MeterCode][point.Timestamp] = true
		}
	}
	for _, meter := range meters {
		worst := worstByMeter[string(meter.Code)]
		meterHealth := healthFor(hasRealHigh[string(meter.Code)], hasAlert[string(meter.Code)])
		if completedErr != nil {
			meterHealth = HealthUnassessed
		} else if readingsByMeter[string(meter.Code)] < 24*s.cfg.MinProfileHistory {
			meterHealth = HealthInsufficient
		}
		row := MeterView{
			MeterID:       string(meter.Code),
			Name:          meter.Name,
			Location:      meter.Location,
			Health:        meterHealth,
			TotalKWh:      round(totals[string(meter.Code)], 2),
			Readings:      readingsByMeter[string(meter.Code)],
			OpenAnomalies: openByMeter[string(meter.Code)],
			WorstSeverity: worst.Severity,
		}
		if len(seriesByMeter[row.MeterID]) > 0 {
			clean := make([]catalog.Reading, 0, len(seriesByMeter[row.MeterID]))
			for _, reading := range seriesByMeter[row.MeterID] {
				if !affectedByMeter[row.MeterID][reading.Timestamp] {
					clean = append(clean, reading)
				}
			}
			profile := analysis.BuildHourlyProfile(clean, s.cfg)
			if completedErr == nil && len(profile.Hours()) < 24 {
				row.Health = HealthInsufficient
			}
			baselineTotal := 0.0
			complete := true
			for _, reading := range seriesByMeter[row.MeterID] {
				point, available := profile.Point(reading.Timestamp.Hour())
				if !available {
					complete = false
					break
				}
				baselineTotal += point.Expected
			}
			if complete && baselineTotal > 0 {
				variation := round((row.TotalKWh-baselineTotal)/baselineTotal*100, 1)
				row.VariationPercent = &variation
			}
		}
		view.Meters = append(view.Meters, row)
		view.TotalKWh = round(view.TotalKWh+row.TotalKWh, 2)
	}

	if completedErr == nil {
		last := progressOf(completed, anomalies)
		view.LastSuccessfulRun = &last
	}
	run, err := s.store.LatestRun(ctx)
	switch {
	case err == nil:
		progress, err := s.store.RunAnomalies(ctx, run.ID)
		if err != nil {
			return Dashboard{}, err
		}
		last := progressOf(run, progress)
		view.LastRun = &last
	case errors.Is(err, store.ErrNotFound):
		// No run yet is a state the dashboard can show, not an error: the catalogue
		// is still worth listing before anything has been analysed.
	default:
		return Dashboard{}, err
	}
	return view, nil
}

// healthFor derives a meter's standing from its open findings: a high-severity
// real anomaly is critical, any other high or medium finding is an alert, and
// anything else is healthy. Health is recomputed on read and never persisted.
func healthFor(hasRealHigh, hasAlert bool) Health {
	switch {
	case hasRealHigh:
		return HealthCritical
	case hasAlert:
		return HealthAlert
	default:
		return HealthHealthy
	}
}

func health(worst analysis.Severity) Health {
	switch worst {
	case analysis.SeverityHigh:
		return HealthCritical
	case analysis.SeverityMedium:
		return HealthAlert
	default:
		return HealthHealthy
	}
}

// typeRank orders anomaly types for display when severities tie: a real
// consumption finding outranks a quality finding at the same severity, so a
// meter with both reads as the consumption problem it is.
func typeRank(kind analysis.AnomalyType) int {
	switch kind {
	case analysis.AnomalyReal:
		return 0
	case analysis.AnomalyDataQuality:
		return 1
	case analysis.AnomalyExplainable:
		return 2
	case analysis.AnomalyFalsePositive:
		return 3
	default:
		return 4
	}
}

// severityRank orders severities worst first, with an empty severity last. That
// ordering is what makes the first open finding for a meter its worst one.
func severityRank(severity analysis.Severity) int {
	switch severity {
	case analysis.SeverityHigh:
		return 0
	case analysis.SeverityMedium:
		return 1
	case analysis.SeverityLow:
		return 2
	default:
		return 3
	}
}

// MeterPoint is one hour of a meter's history.
type MeterPoint struct {
	Timestamp         time.Time `json:"timestamp"`
	Consumption       float64   `json:"consumption_kwh"`
	VoltageV          float64   `json:"voltage_v"`
	CurrentA          float64   `json:"current_a"`
	PowerFactor       float64   `json:"power_factor"`
	IngestedStatus    string    `json:"ingested_status"`
	BaselineAvailable bool      `json:"baseline_available"`
	// BaselineKWh is what this hour of the day normally consumes. Zero means the
	// meter has no history for that hour yet, which the chart shows as unknown
	// rather than as a fall to zero.
	BaselineKWh float64 `json:"baseline_kwh"`
	// InAnomaly marks a reading inside a detected episode, so the chart can shade
	// the windows instead of re-deriving them from the anomaly list.
	InAnomaly bool  `json:"in_anomaly"`
	AnomalyID int64 `json:"anomaly_id,omitempty"`
}

// BaselinePoint is one hour of the day in a meter's profile.
type BaselinePoint struct {
	Hour int `json:"hour"`
	// Expected is the median consumption at that hour. It is a median because one
	// long episode must not be able to redefine what normal looks like.
	Expected float64 `json:"expected_kwh"`
	// Samples is how many readings taught the profile that hour. Too few and the
	// profile is shown as not yet known.
	Samples int `json:"samples"`
}

// MeterDetail is the meter page: the history, the baseline it is read against, and
// the findings on this meter.
type MeterDetail struct {
	Meter    catalog.Meter `json:"meter"`
	Health   Health        `json:"health"`
	TotalKWh float64       `json:"total_kwh"`
	Points   []MeterPoint  `json:"points"`
	// Baseline is the meter's hour-of-day profile, sent once rather than repeated
	// per point: it is 24 numbers, not 336.
	Baseline  []BaselinePoint            `json:"baseline"`
	Anomalies []analysis.Anomaly         `json:"anomalies"`
	Events    []catalog.OperationalEvent `json:"events"`
}

// meterSeries is a meter's readings, the baseline they are read against, and the
// episodes among them. The meter page and the readings endpoint are this same
// computation packaged differently, so it is written once here.
type meterSeries struct {
	meter     catalog.Meter
	points    []MeterPoint
	baseline  []BaselinePoint
	total     float64
	anomalies []analysis.Anomaly
}

func (s *Service) meterSeries(ctx context.Context, meterID string) (meterSeries, error) {
	meter, err := s.store.Meter(ctx, meterID)
	if err != nil {
		return meterSeries{}, err
	}
	all, err := s.store.Readings(ctx, meterID, time.Time{}, time.Time{})
	if err != nil {
		return meterSeries{}, err
	}
	anomalies, err := s.store.Anomalies(ctx)
	if err != nil {
		return meterSeries{}, err
	}

	mine := make([]analysis.Anomaly, 0, len(anomalies))
	affected := make(map[time.Time]int64)
	for _, anomaly := range anomalies {
		if anomaly.MeterCode != meterID {
			continue
		}
		mine = append(mine, anomaly)
		for _, point := range anomaly.DeviationSeries {
			affected[point.Timestamp] = anomaly.ID
		}
	}

	// The profile learns from the clean history only, so a sustained change cannot
	// end up explaining itself away. This is the same construction the detector used,
	// which is why the line under the readings is the line they were judged against.
	clean := make([]catalog.Reading, 0, len(all))
	for _, reading := range all {
		if _, detected := affected[reading.Timestamp]; !detected {
			clean = append(clean, reading)
		}
	}
	profile := analysis.BuildHourlyProfile(clean, s.cfg)

	baseline := make([]BaselinePoint, 0, 24)
	expectedAt := map[int]float64{}
	for _, hour := range profile.Hours() {
		point, _ := profile.Point(hour)
		baseline = append(baseline, BaselinePoint{
			Hour:     hour,
			Expected: round(point.Expected, 3),
			Samples:  point.SampleCount,
		})
		expectedAt[hour] = point.Expected
	}

	points := make([]MeterPoint, 0, len(all))
	total := 0.0
	for _, reading := range all {
		point := MeterPoint{
			Timestamp:      reading.Timestamp,
			Consumption:    round(reading.ConsumptionKWh, 3),
			VoltageV:       round(reading.VoltageV, 2),
			CurrentA:       round(reading.CurrentA, 2),
			PowerFactor:    round(reading.PowerFactor, 3),
			BaselineKWh:    round(expectedAt[reading.Timestamp.Hour()], 3),
			IngestedStatus: reading.IngestedStatus,
		}
		_, point.BaselineAvailable = expectedAt[reading.Timestamp.Hour()]
		if anomalyID, ok := affected[reading.Timestamp]; ok {
			point.InAnomaly, point.AnomalyID = true, anomalyID
		}
		points = append(points, point)
		total += reading.ConsumptionKWh
	}
	return meterSeries{
		meter:     meter,
		points:    points,
		baseline:  baseline,
		total:     round(total, 2),
		anomalies: mine,
	}, nil
}

// MeterReadings is the series the chart asks for on its own: the readings, the
// baseline each was read against, and the episodes they fall inside.
type MeterReadings struct {
	MeterID string       `json:"meter_id"`
	Points  []MeterPoint `json:"points"`
}

// MeterReadings serves the chart without also sending the baseline table, the
// findings and the events.
func (s *Service) MeterReadings(ctx context.Context, meterID string) (MeterReadings, error) {
	series, err := s.meterSeries(ctx, meterID)
	if err != nil {
		return MeterReadings{}, err
	}
	return MeterReadings{MeterID: string(series.meter.Code), Points: series.points}, nil
}

// MeterDetail assembles the meter page: the history, the baseline, the findings on
// this meter, and the events that could account for them.
func (s *Service) MeterDetail(ctx context.Context, meterID string) (MeterDetail, error) {
	series, err := s.meterSeries(ctx, meterID)
	if err != nil {
		return MeterDetail{}, err
	}
	events, err := s.store.Events(ctx)
	if err != nil {
		return MeterDetail{}, err
	}

	worst := analysis.Anomaly{}
	hasRealHigh := false
	hasAlert := false
	for _, anomaly := range series.anomalies {
		current := worst
		if severityRank(anomaly.Severity) < severityRank(current.Severity) ||
			(severityRank(anomaly.Severity) == severityRank(current.Severity) &&
				typeRank(anomaly.Type) < typeRank(current.Type)) {
			worst = anomaly
		}
		if anomaly.Type == analysis.AnomalyReal && anomaly.Severity == analysis.SeverityHigh {
			hasRealHigh = true
		} else if anomaly.Severity == analysis.SeverityHigh || anomaly.Severity == analysis.SeverityMedium {
			hasAlert = true
		}
	}
	meterHealth := healthFor(hasRealHigh, hasAlert)
	if _, err := s.store.LatestCompletedRun(ctx); errors.Is(err, store.ErrNotFound) {
		meterHealth = HealthUnassessed
	} else if err != nil {
		return MeterDetail{}, err
	} else if len(series.points) < 24*s.cfg.MinProfileHistory {
		meterHealth = HealthInsufficient
	}

	meterEvents := make([]catalog.OperationalEvent, 0, len(events))
	for _, event := range events {
		if event.MeterCode == series.meter.Code {
			meterEvents = append(meterEvents, event)
		}
	}

	return MeterDetail{
		Meter:     series.meter,
		Health:    meterHealth,
		TotalKWh:  series.total,
		Points:    series.points,
		Baseline:  series.baseline,
		Anomalies: series.anomalies,
		Events:    meterEvents,
	}, nil
}

func round(v float64, places int) float64 {
	factor := math.Pow(10, float64(places))
	return math.Round(v*factor) / factor
}
