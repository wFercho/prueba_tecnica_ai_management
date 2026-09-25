package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/ingest"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/narrate"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/service"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store/memory"
)

// These tests hold the HTTP surface to the API the assignment asks for, and to the
// one promise that matters underneath it: what the detector decides is durable
// before any HTTP response is written, so nothing a client sees depends on a
// language model answering.

type testServer struct {
	t       *testing.T
	handler http.Handler
	store   *memory.Store
	svc     *service.Service
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	fake := seededStore(t)
	svc := service.New(fake, narrate.Failing{Err: errModelUnreachable}, analysis.DefaultDetectorConfig())
	ts := &testServer{t: t, handler: New(svc, discardLogger()), store: fake, svc: svc}
	t.Cleanup(svc.WaitForNarration)
	return ts
}

func (ts *testServer) service() *service.Service { return ts.svc }

// errModelUnreachable is the failure the narrator hits in these tests, so the
// degraded path is the one the API is exercised against.
var errModelUnreachable = errors.New("the model is unreachable")

// discardLogger keeps the request log out of the test output.
func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

// response is one decoded reply, kept raw as well so a test can assert on the bytes
// a client would really receive.
type response struct {
	recorder *httptest.ResponseRecorder
	body     []byte
}

func (ts *testServer) do(method, path string, body any) response {
	ts.t.Helper()

	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			ts.t.Fatalf("encode the request body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}

	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	ts.handler.ServeHTTP(recorder, request)
	return response{recorder: recorder, body: recorder.Body.Bytes()}
}

// decodeInto unmarshals the reply and fails the test if the status is not the one
// expected, so no test has to repeat the status check.
func (r response) decodeInto(t *testing.T, wantStatus int, into any) {
	t.Helper()
	if r.status() != wantStatus {
		t.Fatalf("status = %d, want %d: %s", r.status(), wantStatus, r.body)
	}
	if got := r.header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want JSON", got)
	}
	if err := json.Unmarshal(r.body, into); err != nil {
		t.Fatalf("decode %s: %v", r.body, err)
	}
}

func (r response) status() int         { return r.recorder.Code }
func (r response) header() http.Header { return r.recorder.Header() }

// errorBody is the shape every failure replies with.
func (r response) errorBody(t *testing.T) (code, message string) {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(r.body, &body); err != nil {
		t.Fatalf("decode the error body %s: %v", r.body, err)
	}
	return body.Error.Code, body.Error.Message
}

func TestMetersListsTheCatalogueWithDerivedHealth(t *testing.T) {
	ts := newTestServer(t)

	var reply struct {
		Meters []service.MeterView `json:"meters"`
	}
	ts.do(http.MethodGet, "/meters", nil).decodeInto(t, http.StatusOK, &reply)

	if len(reply.Meters) != 2 {
		t.Fatalf("got %d meters, want 2", len(reply.Meters))
	}
	// Ordered by meter id, so the table does not reshuffle between requests.
	if reply.Meters[0].MeterID != "M-1" || reply.Meters[1].MeterID != "M-2" {
		t.Errorf("meters = %q, %q, want them ordered by id",
			reply.Meters[0].MeterID, reply.Meters[1].MeterID)
	}
	for _, meter := range reply.Meters {
		if meter.Health == "" {
			t.Errorf("meter %s has no health", meter.MeterID)
		}
	}
}

func TestMeterDetailCarriesTheSeriesBaselineAndFindings(t *testing.T) {
	ts := newTestServer(t)
	ts.runAnalysis()

	var reply service.MeterDetail
	ts.do(http.MethodGet, "/meters/M-1", nil).decodeInto(t, http.StatusOK, &reply)

	if reply.Meter.Code != "M-1" {
		t.Errorf("meter = %q, want M-1", reply.Meter.Code)
	}
	if len(reply.Points) != 14*24 {
		t.Errorf("points = %d, want the meter's whole window", len(reply.Points))
	}
	if len(reply.Baseline) == 0 {
		t.Error("the detail has no baseline to draw the readings against")
	}
	if len(reply.Anomalies) == 0 {
		t.Error("the meter has a finding but the detail reports none")
	}
	if reply.Health == "" {
		t.Error("the detail reports no health")
	}
}

func TestMeterReadingsAreServedOnTheirOwnPath(t *testing.T) {
	// The chart asks for readings on its own so it can load a long window without
	// dragging the baseline and the findings along with it.
	ts := newTestServer(t)

	var reply service.MeterReadings
	ts.do(http.MethodGet, "/meters/M-1/readings", nil).decodeInto(t, http.StatusOK, &reply)

	if reply.MeterID != "M-1" {
		t.Errorf("meter_id = %q, want M-1", reply.MeterID)
	}
	if len(reply.Points) != 14*24 {
		t.Errorf("points = %d, want 336", len(reply.Points))
	}
}

func TestAnUnknownMeterIsNotFoundWithAReadableError(t *testing.T) {
	ts := newTestServer(t)

	reply := ts.do(http.MethodGet, "/meters/M-NOPE", nil)
	if reply.status() != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", reply.status(), reply.body)
	}
	code, message := reply.errorBody(t)
	if code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", code)
	}
	if message == "" {
		t.Error("the 404 carries no message for the client to show")
	}
}

func TestAnomaliesAreAnEmptyListBeforeAnyRunRatherThanAnError(t *testing.T) {
	// A dashboard opened before anyone pressed the button must render, so the
	// empty answer is [] and not null or a failure.
	ts := newTestServer(t)

	var reply struct {
		Anomalies []analysis.Anomaly `json:"anomalies"`
	}
	ts.do(http.MethodGet, "/anomalies", nil).decodeInto(t, http.StatusOK, &reply)

	if reply.Anomalies == nil {
		t.Error("anomalies = null, want an empty list the client can iterate")
	}
	if len(reply.Anomalies) != 0 {
		t.Errorf("got %d anomalies before any run", len(reply.Anomalies))
	}
}

func TestRunningTheAnalysisAnswersAcceptedAndTheFindingsAreAlreadyThere(t *testing.T) {
	ts := newTestServer(t)

	reply := ts.do(http.MethodPost, "/ai/analyze", nil)
	if reply.status() != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", reply.status(), reply.body)
	}

	var run struct {
		Run runReply `json:"run"`
	}
	reply.decodeInto(t, http.StatusAccepted, &run)
	if run.Run.ID == 0 {
		t.Error("the reply carries no run id to poll")
	}
	if run.Run.State != "COMPLETED" {
		t.Errorf("run state = %q, want COMPLETED: detection is done when this returns", run.Run.State)
	}
	if run.Run.AnomalyCount == 0 {
		t.Error("the run reports no anomalies, so this proves nothing")
	}

	// The point of the whole ordering: the findings are readable immediately,
	// narrated or not.
	var found struct {
		Anomalies []analysis.Anomaly `json:"anomalies"`
	}
	ts.do(http.MethodGet, "/anomalies", nil).decodeInto(t, http.StatusOK, &found)
	if len(found.Anomalies) != run.Run.AnomalyCount {
		t.Errorf("the run reports %d anomalies and the list holds %d",
			run.Run.AnomalyCount, len(found.Anomalies))
	}
	for _, anomaly := range found.Anomalies {
		if anomaly.Reason == "" || anomaly.RecommendedAction == "" {
			t.Errorf("anomaly %d is listed with no explanation", anomaly.ID)
		}
	}
}

func TestAnAnalysisCanBePolledForItsProgress(t *testing.T) {
	// The Run AI Analysis button polls this while the model is still writing.
	ts := newTestServer(t)
	started := ts.runAnalysis()

	var progress struct {
		Run struct {
			ID           int64  `json:"id"`
			State        string `json:"state"`
			AnomalyCount int    `json:"anomaly_count"`
			Explained    int    `json:"explained"`
			Narrating    int    `json:"narrating"`
			Failed       int    `json:"failed"`
		} `json:"run"`
	}
	ts.do(http.MethodGet, "/ai/analysis/"+itoa(started), nil).decodeInto(t, http.StatusOK, &progress)

	if progress.Run.ID != started {
		t.Errorf("polled run %d, want %d", progress.Run.ID, started)
	}
	if progress.Run.State != "COMPLETED" {
		t.Errorf("state = %q, want COMPLETED", progress.Run.State)
	}
	// The narrator here always fails, so every finding keeps the template and is
	// marked failed. The counts have to say that rather than reporting nothing in
	// flight and leaving the client to think nothing is happening.
	if progress.Run.Explained != 0 {
		t.Errorf("explained = %d, want 0 while the narrator keeps failing", progress.Run.Explained)
	}
	if progress.Run.Narrating != 0 {
		t.Errorf("narrating = %d, want 0 once every narration has failed", progress.Run.Narrating)
	}
	if progress.Run.Failed != progress.Run.AnomalyCount {
		t.Errorf("failed = %d, want the run's %d findings", progress.Run.Failed, progress.Run.AnomalyCount)
	}
}

func TestPollingAnUnknownRunIsNotFound(t *testing.T) {
	ts := newTestServer(t)

	reply := ts.do(http.MethodGet, "/ai/analysis/999", nil)
	if reply.status() != http.StatusNotFound {
		t.Errorf("status = %d, want 404", reply.status())
	}
}

func TestAnomalyDetailCarriesTheEvidenceAndTheMeterItBelongsTo(t *testing.T) {
	// The investigation screen is specified as: what was found, what changed,
	// the comparison against the baseline, related events, severity, confidence,
	// the action, and the evidence. All of it has to be in this one reply.
	ts := newTestServer(t)
	ts.runAnalysis()
	id := ts.firstAnomalyID()

	var reply struct {
		Anomaly service.AnomalyView `json:"anomaly"`
	}
	ts.do(http.MethodGet, "/anomalies/"+itoa(id), nil).decodeInto(t, http.StatusOK, &reply)
	view := reply.Anomaly

	if view.MeterName == "" {
		t.Error("the reply carries no meter name to show as the title")
	}
	if view.Reason == "" || view.RecommendedAction == "" {
		t.Error("the investigation view has no explanation or no action")
	}
	if view.DeviationPercent == 0 {
		t.Error("the comparison against the baseline is missing")
	}
	if len(view.DeviationSeries) == 0 {
		t.Error("the evidence series is missing, so the chart has nothing to draw")
	}
	if view.ConfidenceBasis == (analysis.Evidence{}) {
		t.Error("the confidence arrives with no terms to trace it to")
	}
	if view.Severity == "" || view.ConfidenceBand == "" {
		t.Error("severity or confidence band is missing")
	}
}

func TestAnInvestigationCanRecordTheOperatorsDecision(t *testing.T) {
	ts := newTestServer(t)
	ts.runAnalysis()
	id := ts.firstAnomalyID()

	var reply struct {
		Anomaly analysis.Anomaly `json:"anomaly"`
	}
	ts.do(http.MethodPatch, "/anomalies/"+itoa(id),
		map[string]string{"status": "ACKNOWLEDGED"}).decodeInto(t, http.StatusOK, &reply)

	if reply.Anomaly.Status != catalog.StatusAcknowledged {
		t.Errorf("status = %q, want ACKNOWLEDGED", reply.Anomaly.Status)
	}

	// And it sticks, rather than being acknowledged in one request and forgotten
	// by the next.
	var listed struct {
		Anomalies []analysis.Anomaly `json:"anomalies"`
	}
	ts.do(http.MethodGet, "/anomalies", nil).decodeInto(t, http.StatusOK, &listed)
	for _, anomaly := range listed.Anomalies {
		if anomaly.ID == id && anomaly.Status != catalog.StatusAcknowledged {
			t.Errorf("anomaly %d reads back as %q", id, anomaly.Status)
		}
	}
}

func TestAnUnknownStatusIsRejectedWithoutTouchingTheFinding(t *testing.T) {
	ts := newTestServer(t)
	ts.runAnalysis()
	id := ts.firstAnomalyID()

	reply := ts.do(http.MethodPatch, "/anomalies/"+itoa(id),
		map[string]string{"status": "MAYBE"})
	if reply.status() != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", reply.status(), reply.body)
	}
	if code, _ := reply.errorBody(t); code != "INVALID_REQUEST" {
		t.Errorf("code = %q, want INVALID_REQUEST", code)
	}

	var listed struct {
		Anomalies []analysis.Anomaly `json:"anomalies"`
	}
	ts.do(http.MethodGet, "/anomalies", nil).decodeInto(t, http.StatusOK, &listed)
	for _, anomaly := range listed.Anomalies {
		if anomaly.ID == id && anomaly.Status != catalog.StatusOpen {
			t.Errorf("a rejected status still changed the finding to %q", anomaly.Status)
		}
	}
}

func TestTheDashboardSummaryLeadsWithWhatNeedsAttention(t *testing.T) {
	ts := newTestServer(t)
	ts.runAnalysis()

	var reply struct {
		LastRun        *service.RunProgress `json:"last_run"`
		Meters         []service.MeterView  `json:"meters"`
		AnomalyCounts  map[string]int       `json:"anomaly_counts"`
		NeedsAttention int                  `json:"needs_attention"`
		TotalKWh       float64              `json:"total_kwh"`
	}
	ts.do(http.MethodGet, "/dashboard/summary", nil).decodeInto(t, http.StatusOK, &reply)

	if reply.LastRun == nil {
		t.Fatal("the summary names no last run")
	}
	if reply.NeedsAttention == 0 {
		t.Error("needs_attention = 0 although the run found something worth acting on")
	}
	if len(reply.AnomalyCounts) == 0 {
		t.Error("the summary breaks the findings down by no type at all")
	}
	if reply.TotalKWh <= 0 {
		t.Error("the summary reports no consumption at all")
	}
	if len(reply.Meters) != 2 {
		t.Errorf("meters = %d, want the whole catalogue", len(reply.Meters))
	}
}

func TestTheDashboardSummaryBeforeAnyRunIsUsable(t *testing.T) {
	ts := newTestServer(t)

	var reply struct {
		LastRun *service.RunProgress `json:"last_run"`
		Meters  []service.MeterView  `json:"meters"`
	}
	ts.do(http.MethodGet, "/dashboard/summary", nil).decodeInto(t, http.StatusOK, &reply)

	if reply.LastRun != nil {
		t.Errorf("last_run = %+v, want null before anything has run", reply.LastRun)
	}
	if len(reply.Meters) == 0 {
		t.Error("the catalogue is empty before the first run, so the page has nothing to show")
	}
}

func TestAnUnknownPathAndAWrongMethodAreBothRefused(t *testing.T) {
	ts := newTestServer(t)

	if reply := ts.do(http.MethodGet, "/nope", nil); reply.status() != http.StatusNotFound {
		t.Errorf("unknown path status = %d, want 404", reply.status())
	}
	// Reading the analysis endpoint must not start an analysis.
	reply := ts.do(http.MethodGet, "/ai/analyze", nil)
	if reply.status() != http.StatusMethodNotAllowed {
		t.Errorf("GET /ai/analyze status = %d, want 405", reply.status())
	}
	var listed struct {
		Anomalies []analysis.Anomaly `json:"anomalies"`
	}
	ts.do(http.MethodGet, "/anomalies", nil).decodeInto(t, http.StatusOK, &listed)
	if len(listed.Anomalies) != 0 {
		t.Error("a refused method still ran the analysis")
	}
}

// TestTheDeliveredDatasetComesBackWithTheFourExpectedCases is the end-to-end gate:
// the delivered CSVs in, the same four findings the assignment specifies out, with
// the unexplained one first.
func TestTheDeliveredDatasetComesBackWithTheFourExpectedCases(t *testing.T) {
	ts := newTestServerWithDeliveredData(t)

	reply := ts.do(http.MethodPost, "/ai/analyze", nil)
	reply.decodeInto(t, http.StatusAccepted, &struct {
		Run runReply `json:"run"`
	}{})

	var found struct {
		Anomalies []analysis.Anomaly `json:"anomalies"`
	}
	ts.do(http.MethodGet, "/anomalies", nil).decodeInto(t, http.StatusOK, &found)

	want := map[string]analysis.AnomalyType{
		"M-104": analysis.AnomalyExplainable,
		"M-106": analysis.AnomalyFalsePositive,
		"M-109": analysis.AnomalyReal,
		"M-112": analysis.AnomalyDataQuality,
	}
	if len(found.Anomalies) != len(want) {
		t.Fatalf("got %d anomalies, want exactly %d", len(found.Anomalies), len(want))
	}
	for _, anomaly := range found.Anomalies {
		kind, expected := want[anomaly.MeterCode]
		if !expected {
			t.Errorf("meter %s was reported, want silence", anomaly.MeterCode)
			continue
		}
		if anomaly.Type != kind {
			t.Errorf("%s read as %s, want %s", anomaly.MeterCode, anomaly.Type, kind)
		}
	}
	// Most urgent first, so M-109 leads the table the way the assignment shows it.
	if found.Anomalies[0].MeterCode != "M-109" {
		t.Errorf("the first anomaly is %s, want M-109", found.Anomalies[0].MeterCode)
	}
	if found.Anomalies[0].Severity != analysis.SeverityHigh {
		t.Errorf("M-109 severity = %s, want HIGH", found.Anomalies[0].Severity)
	}
}

// runReply mirrors the run payload so the tests read exactly the fields a client
// sees, and cannot silently depend on a Go struct the API does not expose.
type runReply struct {
	ID           int64  `json:"id"`
	State        string `json:"state"`
	AnomalyCount int    `json:"anomaly_count"`
	Explained    int    `json:"explained"`
	Narrating    int    `json:"narrating"`
}

// runAnalysis triggers a run and waits for its narration, so a test can assert on
// the settled state.
func (ts *testServer) runAnalysis() int64 {
	ts.t.Helper()
	var reply struct {
		Run runReply `json:"run"`
	}
	ts.do(http.MethodPost, "/ai/analyze", nil).decodeInto(ts.t, http.StatusAccepted, &reply)
	ts.service().WaitForNarration()
	return reply.Run.ID
}

func (ts *testServer) firstAnomalyID() int64 {
	ts.t.Helper()
	var found struct {
		Anomalies []analysis.Anomaly `json:"anomalies"`
	}
	ts.do(http.MethodGet, "/anomalies", nil).decodeInto(ts.t, http.StatusOK, &found)
	if len(found.Anomalies) == 0 {
		ts.t.Fatal("no anomalies to investigate")
	}
	return found.Anomalies[0].ID
}

func newTestServerWithDeliveredData(t *testing.T) *testServer {
	t.Helper()

	readings, events, meters := deliveredData(t)
	fake := memory.New()
	if err := fake.Seed(meters, readings, events); err != nil {
		t.Fatalf("seed: %v", err)
	}
	svc := service.New(fake, narrate.Failing{Err: errModelUnreachable}, analysis.DefaultDetectorConfig())
	ts := &testServer{t: t, handler: New(svc, discardLogger()), store: fake, svc: svc}
	t.Cleanup(svc.WaitForNarration)
	return ts
}

// deliveredData reads the delivered CSVs and derives the meter catalogue from them,
// which is what the seeder does.
func deliveredData(t *testing.T) ([]catalog.Reading, []catalog.OperationalEvent, []catalog.Meter) {
	t.Helper()

	readingsFile, err := os.Open(filepath.Join("..", "..", "..", "data", "readings.csv"))
	if err != nil {
		t.Fatalf("open the delivered readings: %v", err)
	}
	defer readingsFile.Close()
	eventsFile, err := os.Open(filepath.Join("..", "..", "..", "data", "events.csv"))
	if err != nil {
		t.Fatalf("open the delivered events: %v", err)
	}
	defer eventsFile.Close()

	readings, err := ingest.ReadReadings(readingsFile)
	if err != nil {
		t.Fatalf("read the delivered readings: %v", err)
	}
	events, err := ingest.ReadEvents(eventsFile)
	if err != nil {
		t.Fatalf("read the delivered events: %v", err)
	}

	seen := map[catalog.MeterCode]bool{}
	var meters []catalog.Meter
	for _, reading := range readings {
		if seen[reading.MeterCode] {
			continue
		}
		seen[reading.MeterCode] = true
		meters = append(meters, catalog.Meter{
			Code:     reading.MeterCode,
			Name:     "Medidor " + string(reading.MeterCode),
			Location: "Planta 1",
		})
	}
	return readings, events, meters
}

// seededStore builds a small store shaped like the delivered one: two meters, one of
// which doubles its load for the last few days.
func seededStore(t *testing.T) *memory.Store {
	t.Helper()

	var readings []catalog.Reading
	for _, code := range []catalog.MeterCode{"M-1", "M-2"} {
		for day := range 14 {
			for hour := range 24 {
				reading := catalog.Reading{
					MeterCode:      code,
					Timestamp:      time.Date(2026, 9, 1, hour, 0, 0, 0, time.UTC).AddDate(0, 0, day),
					ConsumptionKWh: 30,
					VoltageV:       220,
					CurrentA:       100,
					PowerFactor:    0.95,
					IngestedStatus: "OK",
				}
				if code == "M-1" && day >= 10 {
					reading.ConsumptionKWh = 60
					reading.CurrentA = 200
				}
				readings = append(readings, reading)
			}
		}
	}
	meters := []catalog.Meter{
		{Code: "M-1", Name: "Taller 1", Location: "Planta 1"},
		{Code: "M-2", Name: "Taller 2", Location: "Planta 2"},
	}
	fake := memory.New()
	if err := fake.Seed(meters, readings, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return fake
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }
