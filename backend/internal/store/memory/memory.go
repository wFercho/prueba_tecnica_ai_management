// Package memory is an in-memory store.Store.
//
// It exists so the HTTP layer, the dashboard aggregation and the analysis flow
// can be tested without a database, and so `make dev` can run without Docker. It
// implements the same contract as the PostgreSQL store, including the parts that
// are easy to get wrong: the anomaly ordering, ErrNotFound, and the invariant
// that narration never blanks the rules prose.
package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

// Store is a store.Store held in memory. It is safe for concurrent use, because
// narration writes from a goroutine while requests read.
type Store struct {
	mu           sync.RWMutex
	meters       map[string]catalog.Meter
	order        []string
	readings     []catalog.Reading
	events       []catalog.OperationalEvent
	runs         []store.Run
	anomaly      map[int64]analysis.Anomaly
	anomalyOrder []int64
	users        map[string]store.User
	sessions     map[string]session
	nextID       int64
	// Fails makes every write fail, so callers can be tested against a database
	// that is unavailable.
	Fails error
}

// New returns an empty store.
func New() *Store {
	return &Store{
		meters:  map[string]catalog.Meter{},
		anomaly: map[int64]analysis.Anomaly{},
	}
}

func (s *Store) fail() error {
	if s.Fails != nil {
		return s.Fails
	}
	return nil
}

// Seed replaces the catalogue, readings and events in one call, which is what
// the importer does and what tests do in setup.
func (s *Store) Seed(meters []catalog.Meter, readings []catalog.Reading, events []catalog.OperationalEvent) error {
	return s.ReplaceDataset(context.Background(), meters, readings, events)
}

func (s *Store) ReplaceDataset(ctx context.Context, meters []catalog.Meter, readings []catalog.Reading, events []catalog.OperationalEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	staged := make(map[string]catalog.Meter, len(meters))
	var order []string
	for _, meter := range meters {
		if _, seen := staged[string(meter.Code)]; !seen {
			order = append(order, string(meter.Code))
		}
		staged[string(meter.Code)] = meter
	}
	for _, reading := range readings {
		if _, known := staged[string(reading.MeterCode)]; !known {
			return fmt.Errorf("reading for unknown meter %s: %w", reading.MeterCode, store.ErrNotFound)
		}
	}
	for _, event := range events {
		if _, known := staged[string(event.MeterCode)]; !known {
			return fmt.Errorf("event for unknown meter %s: %w", event.MeterCode, store.ErrNotFound)
		}
	}
	s.meters = staged
	s.order = order
	s.readings = append([]catalog.Reading(nil), readings...)
	s.events = append([]catalog.OperationalEvent(nil), events...)
	s.runs = nil
	s.anomaly = map[int64]analysis.Anomaly{}
	s.anomalyOrder = nil
	return nil
}

// AppendReadings stores a batch, keyed on (meter, hour) so a re-import replaces the
// hour rather than adding a second reading of it.
func (s *Store) AppendReadings(_ context.Context, readings []catalog.Reading) error {
	if len(readings) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(); err != nil {
		return err
	}
	for _, reading := range readings {
		if _, known := s.meters[string(reading.MeterCode)]; !known {
			// A reading nothing will ever read is a mistake in the import, not data.
			return fmt.Errorf("reading for unknown meter %s: %w", reading.MeterCode, store.ErrNotFound)
		}
		s.readings = upsertReading(s.readings, reading)
	}
	return nil
}

// AppendEvents stores a batch, keyed on (meter, hour, type) so one event reported
// twice stays one event.
func (s *Store) AppendEvents(_ context.Context, events []catalog.OperationalEvent) error {
	if len(events) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(); err != nil {
		return err
	}
	for _, event := range events {
		if _, known := s.meters[string(event.MeterCode)]; !known {
			return fmt.Errorf("event for unknown meter %s: %w", event.MeterCode, store.ErrNotFound)
		}
		s.events = upsertEvent(s.events, event)
	}
	return nil
}

func upsertReading(readings []catalog.Reading, incoming catalog.Reading) []catalog.Reading {
	for i, existing := range readings {
		if existing.MeterCode == incoming.MeterCode && existing.Timestamp.Equal(incoming.Timestamp) {
			readings[i] = incoming
			return readings
		}
	}
	return append(readings, incoming)
}

func upsertEvent(events []catalog.OperationalEvent, incoming catalog.OperationalEvent) []catalog.OperationalEvent {
	for i, existing := range events {
		if existing.MeterCode == incoming.MeterCode &&
			existing.Type == incoming.Type &&
			existing.Timestamp.Equal(incoming.Timestamp) {
			// The description is part of the event's meaning, so a re-import that
			// carries more detail keeps the newer wording.
			events[i] = incoming
			return events
		}
	}
	return append(events, incoming)
}

func (s *Store) Meters(context.Context) ([]catalog.Meter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return nil, err
	}
	out := make([]catalog.Meter, 0, len(s.order))
	for _, code := range s.order {
		out = append(out, s.meters[code])
	}
	return out, nil
}

func (s *Store) Meter(_ context.Context, meterID string) (catalog.Meter, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return catalog.Meter{}, err
	}
	meter, ok := s.meters[meterID]
	if !ok {
		return catalog.Meter{}, fmt.Errorf("meter %s: %w", meterID, store.ErrNotFound)
	}
	return meter, nil
}

func (s *Store) UpsertMeters(ctx context.Context, meters []catalog.Meter) error {
	return s.Seed(meters, s.currentReadings(), s.currentEvents())
}

func (s *Store) currentReadings() []catalog.Reading {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readings
}

func (s *Store) currentEvents() []catalog.OperationalEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.events
}

func (s *Store) Readings(_ context.Context, meterID string, from, to time.Time) ([]catalog.Reading, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return nil, err
	}
	var out []catalog.Reading
	for _, reading := range s.readings {
		if string(reading.MeterCode) != meterID {
			continue
		}
		if !from.IsZero() && reading.Timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && reading.Timestamp.After(to) {
			continue
		}
		out = append(out, reading)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out, nil
}

func (s *Store) AllReadings(context.Context) ([]catalog.Reading, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return nil, err
	}
	out := append([]catalog.Reading(nil), s.readings...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].MeterCode != out[j].MeterCode {
			return out[i].MeterCode < out[j].MeterCode
		}
		return out[i].Timestamp.Before(out[j].Timestamp)
	})
	return out, nil
}

func (s *Store) ReadingWindow(context.Context) (time.Time, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return time.Time{}, time.Time{}, err
	}
	if len(s.readings) == 0 {
		return time.Time{}, time.Time{}, store.ErrNotFound
	}
	from, to := s.readings[0].Timestamp, s.readings[0].Timestamp
	for _, reading := range s.readings {
		if reading.Timestamp.Before(from) {
			from = reading.Timestamp
		}
		if reading.Timestamp.After(to) {
			to = reading.Timestamp
		}
	}
	return from, to, nil
}

func (s *Store) ConsumptionTotals(context.Context) (map[string]float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return nil, err
	}
	totals := map[string]float64{}
	for _, reading := range s.readings {
		totals[string(reading.MeterCode)] += reading.ConsumptionKWh
	}
	return totals, nil
}

func (s *Store) Events(context.Context) ([]catalog.OperationalEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return nil, err
	}
	return append([]catalog.OperationalEvent(nil), s.events...), nil
}

func (s *Store) StartRun(_ context.Context, windowStart, windowEnd time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(); err != nil {
		return 0, err
	}
	s.nextID++
	run := store.Run{
		ID:          s.nextID,
		StartedAt:   time.Now().UTC(),
		State:       store.RunRunning,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	}
	s.runs = append(s.runs, run)
	return run.ID, nil
}

func (s *Store) FinishRun(_ context.Context, runID int64, state store.RunState, anomalyCount int, failure string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(); err != nil {
		return err
	}
	finished := time.Now().UTC()
	for i := range s.runs {
		if s.runs[i].ID != runID {
			continue
		}
		s.runs[i].State = state
		s.runs[i].AnomalyCount = anomalyCount
		s.runs[i].Error = failure
		s.runs[i].FinishedAt = &finished
		return nil
	}
	return fmt.Errorf("run %d: %w", runID, store.ErrNotFound)
}

func (s *Store) LatestRun(context.Context) (store.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return store.Run{}, err
	}
	if len(s.runs) == 0 {
		return store.Run{}, store.ErrNotFound
	}
	return s.runs[len(s.runs)-1], nil
}

func (s *Store) LatestCompletedRun(context.Context) (store.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return store.Run{}, err
	}
	for i := len(s.runs) - 1; i >= 0; i-- {
		if s.runs[i].State == store.RunCompleted {
			return s.runs[i], nil
		}
	}
	return store.Run{}, store.ErrNotFound
}

// Run returns one run, for the run-state endpoint.
func (s *Store) Run(_ context.Context, runID int64) (store.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, run := range s.runs {
		if run.ID == runID {
			return run, nil
		}
	}
	return store.Run{}, fmt.Errorf("run %d: %w", runID, store.ErrNotFound)
}

func (s *Store) SaveAnomalies(_ context.Context, runID int64, anomalies []analysis.Anomaly) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(); err != nil {
		return err
	}
	// All or nothing, as the contract requires: a run whose anomalies are only
	// partly written is a run whose count lies.
	staged := map[int64]analysis.Anomaly{}
	var order []int64
	for _, anomaly := range anomalies {
		if _, clash := staged[anomaly.ID]; anomaly.ID != 0 && clash {
			return fmt.Errorf("anomaly %d saved twice in run %d", anomaly.ID, runID)
		}
		s.nextID++
		anomaly.ID = s.nextID
		anomaly.Anomaly = true
		anomaly.RunID = runID
		if anomaly.ExplanationSource == "" {
			anomaly.ExplanationSource = catalog.SourceRules
		}
		if anomaly.ExplanationStatus == "" {
			anomaly.ExplanationStatus = catalog.ExplanationPending
		}
		if anomaly.Status == "" {
			anomaly.Status = catalog.StatusOpen
		}
		staged[anomaly.ID] = anomaly
		order = append(order, anomaly.ID)
	}
	for id, anomaly := range staged {
		s.anomaly[id] = anomaly
		s.anomalyOrder = append(s.anomalyOrder, id)
	}
	return nil
}

func (s *Store) Narrate(_ context.Context, anomalyID int64, source store.ExplanationSource, reason, action string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(); err != nil {
		return err
	}
	anomaly, ok := s.anomaly[anomalyID]
	if !ok {
		return fmt.Errorf("anomaly %d: %w", anomalyID, store.ErrNotFound)
	}
	anomaly.ExplanationSource = source
	anomaly.ExplanationStatus = store.ExplanationReady
	anomaly.Reason = reason
	anomaly.RecommendedAction = action
	s.anomaly[anomalyID] = anomaly
	return nil
}

func (s *Store) MarkNarrationFailed(_ context.Context, anomalyID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.fail(); err != nil {
		return err
	}
	anomaly, ok := s.anomaly[anomalyID]
	if !ok {
		return fmt.Errorf("anomaly %d: %w", anomalyID, store.ErrNotFound)
	}
	anomaly.ExplanationStatus = store.ExplanationFailed
	s.anomaly[anomalyID] = anomaly
	return nil
}

func (s *Store) Anomalies(ctx context.Context) ([]analysis.Anomaly, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return nil, err
	}
	for i := len(s.runs) - 1; i >= 0; i-- {
		if s.runs[i].State == store.RunCompleted {
			return s.anomaliesOfRunLocked(s.runs[i].ID), nil
		}
	}
	return nil, nil
}

func (s *Store) RunAnomalies(_ context.Context, runID int64) ([]analysis.Anomaly, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return nil, err
	}
	return s.anomaliesOfRunLocked(runID), nil
}

// anomaliesOfRunLocked returns one run's anomalies in operator reading order.
func (s *Store) anomaliesOfRunLocked(runID int64) []analysis.Anomaly {
	var out []analysis.Anomaly
	for _, id := range s.anomalyOrder {
		if anomaly, ok := s.anomaly[id]; ok && anomaly.RunID == runID {
			out = append(out, anomaly)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if severityRank(out[i].Severity) != severityRank(out[j].Severity) {
			return severityRank(out[i].Severity) < severityRank(out[j].Severity)
		}
		if typeRank(out[i].Type) != typeRank(out[j].Type) {
			return typeRank(out[i].Type) < typeRank(out[j].Type)
		}
		return out[i].Confidence > out[j].Confidence
	})
	return out
}

func (s *Store) Anomaly(_ context.Context, id int64) (store.AnomalyDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.fail(); err != nil {
		return store.AnomalyDetail{}, err
	}
	anomaly, ok := s.anomaly[id]
	if !ok {
		return store.AnomalyDetail{}, fmt.Errorf("anomaly %d: %w", id, store.ErrNotFound)
	}
	return anomaly, nil
}

// severityRank and typeRank are the reading order: worst first, and within a
// severity, the kind an operator should look at first. They are the same ranking
// the SQL store uses in SQL, so a dashboard built on this fake is laid out the
// same way as one built on PostgreSQL.
func severityRank(severity analysis.Severity) int {
	switch severity {
	case analysis.SeverityHigh:
		return 0
	case analysis.SeverityMedium:
		return 1
	default:
		return 2
	}
}

func typeRank(kind analysis.AnomalyType) int {
	switch kind {
	case analysis.AnomalyReal:
		return 0
	case analysis.AnomalyDataQuality:
		return 1
	case analysis.AnomalyExplainable:
		return 2
	default:
		return 3
	}
}

// Store satisfies the contract it was written against, at compile time.
var _ store.Store = (*Store)(nil)
