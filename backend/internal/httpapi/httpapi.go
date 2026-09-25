// Package httpapi is the HTTP surface: the routes the assignment lists, and nothing
// that is not asked for.
//
// It holds no business logic. Every handler translates a request into one service
// call and translates what comes back into JSON, so the rules about what a finding
// means live in one place and this one only decides how to say it. A failing
// narrator, an unreachable database and a meter that does not exist all arrive here
// as the same thing: a status code and a message a person can read.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/catalog"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/service"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store"
)

// Handler serves the API.
type Handler struct {
	service *service.Service
	log     *slog.Logger
	mux     *http.ServeMux
	// dashboard answers the paths the API does not route, and is nil until a build
	// is found.
	dashboard http.Handler
}

// New returns the API. The routes are the ones the assignment lists, at the root:
//
//	GET   /meters
//	GET   /meters/{meterId}
//	GET   /meters/{meterId}/readings
//	GET   /anomalies
//	GET   /anomalies/{id}
//	PATCH /anomalies/{id}
//	POST  /ai/analyze
//	GET   /ai/analysis/{id}
//	GET   /dashboard/summary
//
// PATCH is the one addition: the investigation view has to record what an operator
// decided, and the data model has a status column for it.
func New(svc *service.Service, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	handler := &Handler{service: svc, log: log, mux: http.NewServeMux()}

	handler.mux.HandleFunc("GET /meters", handler.listMeters)
	handler.mux.HandleFunc("GET /meters/{meterId}", handler.getMeter)
	handler.mux.HandleFunc("GET /meters/{meterId}/readings", handler.getMeterReadings)
	handler.mux.HandleFunc("GET /anomalies", handler.listAnomalies)
	handler.mux.HandleFunc("GET /anomalies/{id}", handler.getAnomaly)
	handler.mux.HandleFunc("PATCH /anomalies/{id}", handler.patchAnomaly)
	handler.mux.HandleFunc("POST /ai/analyze", handler.postAnalyze)
	handler.mux.HandleFunc("GET /ai/analysis/{id}", handler.getAnalysis)
	handler.mux.HandleFunc("GET /dashboard/summary", handler.getDashboardSummary)

	return handler
}

// ServeHTTP applies the cross-cutting concerns and hands the request to a route.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	// A closure, because a deferred call's arguments are evaluated immediately — and
	// time.Since evaluated there would log the time taken so far: always zero.
	defer func() {
		h.log.Info("request", "method", r.Method, "path", r.URL.Path, "took", time.Since(started))
	}()
	h.recoverPanics(w, r)

	// The API's own routes are consulted first. A request that matched no route is
	// the dashboard's, which is what lets the page own /dashboard and /anomalies/12
	// while /meters/M-109 and /anomalies/12-as-an-id still reach the data — the two
	// shapes are told apart by which routes are registered, not by guessing here.
	if h.dashboard != nil {
		if _, pattern := h.mux.Handler(r); pattern == "" {
			h.dashboard.ServeHTTP(w, r)
			return
		}
	}
	h.mux.ServeHTTP(w, r)
}

// recoverPanics answers a panic as a 500 rather than dropping the connection, so a
// bug in one handler does not look like a network failure to the client. The
// original error is logged with the request that caused it.
func (h *Handler) recoverPanics(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if recovered := recover(); recovered != nil {
			h.log.Error("the handler panicked", "method", r.Method, "path", r.URL.Path, "panic", recovered)
			writeError(w, http.StatusInternalServerError, "INTERNAL", "the request could not be completed")
		}
	}()
}

func (h *Handler) listMeters(w http.ResponseWriter, r *http.Request) {
	view, err := h.service.Dashboard(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"meters": view.Meters})
}

func (h *Handler) getMeter(w http.ResponseWriter, r *http.Request) {
	detail, err := h.service.MeterDetail(r.Context(), r.PathValue("meterId"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (h *Handler) getMeterReadings(w http.ResponseWriter, r *http.Request) {
	// The chart's own endpoint: the series with the baseline it is read against and
	// the episodes it falls inside, so the chart never has to re-derive either, and
	// a long window costs neither the findings nor the events.
	readings, err := h.service.MeterReadings(r.Context(), r.PathValue("meterId"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, readings)
}

func (h *Handler) listAnomalies(w http.ResponseWriter, r *http.Request) {
	anomalies, err := h.service.Anomalies(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	// An empty list is [] rather than null: the table is rendered either way, and a
	// null would be the one shape it cannot render.
	if anomalies == nil {
		anomalies = []analysis.Anomaly{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"anomalies": anomalies})
}

func (h *Handler) getAnomaly(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(w, r)
	if !ok {
		return
	}
	anomaly, err := h.service.Anomaly(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"anomaly": anomaly})
}

// patchAnomalyStatus is the request body of the one write the API accepts.
type patchAnomalyStatus struct {
	Status catalog.AnomalyStatus `json:"status"`
}

func (h *Handler) patchAnomaly(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(w, r)
	if !ok {
		return
	}
	var body patchAnomalyStatus
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	if err := h.service.SetAnomalyStatus(r.Context(), id, body.Status); err != nil {
		h.fail(w, r, err)
		return
	}
	// The updated row is returned rather than a bare acknowledgement, so the client
	// renders the server's view instead of assuming its own was applied.
	anomaly, err := h.service.Anomaly(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"anomaly": anomaly})
}

func (h *Handler) postAnalyze(w http.ResponseWriter, r *http.Request) {
	// Detection and the deterministic explanations are finished by the time this
	// returns, so the reply is 202 with a run the client can read the findings from
	// straight away and poll while the narrator catches up.
	run, err := h.service.Analyze(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"run": run})
}

func (h *Handler) getAnalysis(w http.ResponseWriter, r *http.Request) {
	id, ok := h.pathID(w, r)
	if !ok {
		return
	}
	progress, err := h.service.Run(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": progress})
}

func (h *Handler) getDashboardSummary(w http.ResponseWriter, r *http.Request) {
	view, err := h.service.Dashboard(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// pathID reads the {id} path value, answering a malformed one itself.
func (h *Handler) pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST",
			fmt.Sprintf("%q is not a valid id", raw))
		return 0, false
	}
	return id, true
}

// fail turns a service error into a status code. A missing row is the only expected
// one; everything else is logged and reported as a failure without its detail,
// because a database error message is not something to hand to a browser.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	case errors.Is(err, service.ErrInvalidRequest):
		// The caller sent something the system understood well enough to refuse.
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	h.log.Error("the request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL", "the request could not be completed")
}

// decodeJSON reads a JSON body, refusing an unknown field rather than ignoring it:
// a client sending {"severity": "HIGH"} to the status endpoint has made a mistake,
// and silently accepting it would leave the operator thinking it worked.
func decodeJSON(r *http.Request, into any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return fmt.Errorf("the request body is not valid: %w", err)
	}
	return nil
}

// errorPayload is the body of every failure. The code is for the client to branch
// on and the message is for a person to read.
type errorPayload struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var payload errorPayload
	payload.Error.Code = code
	payload.Error.Message = message
	writeJSON(w, status, payload)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		// The payload could not be encoded, so there is nothing to send but the
		// fact that it failed.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `{"error":{"code":"INTERNAL","message":"the response could not be encoded"}}`)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(body)
}
