package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The dashboard shares the API's origin, so these tests cover the three cases that
// actually bite: no build present, a client-side route with no file behind it, and a
// missing asset that must not be answered with HTML.

func buildDashboard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "<!doctype html><title>Energy</title>")
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatalf("create assets: %v", err)
	}
	writeFile(t, filepath.Join(dir, "assets", "app.js"), "console.log('app')")
	return dir
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// servedDashboard is a real API handler with a build attached, which is the wiring the
// server command uses.
func servedDashboard(t *testing.T, dir string) http.Handler {
	t.Helper()
	handler := New(newTestServer(t).service(), discardLogger())
	if err := handler.ServeDashboard(dir, discardLogger()); err != nil {
		t.Fatalf("ServeDashboard: %v", err)
	}
	return handler
}

func request(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder
}

func TestWithoutABuiltDashboardTheAPIStillAnswers(t *testing.T) {
	// `go run ./cmd/server` with no frontend build has to serve the API and say why
	// there is no page, rather than refusing to start.
	handler := servedDashboard(t, t.TempDir())

	api := request(handler, http.MethodGet, "/meters")
	if api.Code != http.StatusOK {
		t.Errorf("GET /meters = %d, want 200: %s", api.Code, api.Body)
	}
	// And a path neither the API nor a page claims is still a 404 from the API,
	// rather than an empty 200.
	if missing := request(handler, http.MethodGet, "/nowhere"); missing.Code != http.StatusNotFound {
		t.Errorf("GET /nowhere = %d, want 404", missing.Code)
	}
}

func TestTheRootServesTheBuiltPage(t *testing.T) {
	recorder := request(servedDashboard(t, buildDashboard(t)), http.MethodGet, "/")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "<title>Energy</title>") {
		t.Errorf("body = %s, want the built page", recorder.Body)
	}
}

func TestAClientSideRouteWithNoFileBehindItServesThePage(t *testing.T) {
	// The dashboard navigates to paths that are not files, and the server has to hand
	// back the page and let the client router resolve them.
	handler := servedDashboard(t, buildDashboard(t))

	for _, route := range []string{"/", "/medidores/M-109", "/hallazgos/12"} {
		recorder := request(handler, http.MethodGet, route)
		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", route, recorder.Code)
		}
		if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
			t.Errorf("GET %s Content-Type = %q, want HTML", route, got)
		}
		if !strings.Contains(recorder.Body.String(), "<title>Energy</title>") {
			t.Errorf("GET %s did not serve the page: %s", route, recorder.Body)
		}
	}
}

func TestAMissingAssetIsNotAnsweredWithThePage(t *testing.T) {
	// Answering a missing module with index.html turns a build problem into a parse
	// error far from the cause, so it is a 404.
	handler := servedDashboard(t, buildDashboard(t))

	for _, asset := range []string{"/assets/missing.js", "/assets/app.css", "/favicon.ico"} {
		if recorder := request(handler, http.MethodGet, asset); recorder.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", asset, recorder.Code)
		}
	}
}

func TestAnAssetThatExistsIsServedWithItsOwnType(t *testing.T) {
	recorder := request(servedDashboard(t, buildDashboard(t)), http.MethodGet, "/assets/app.js")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "javascript") {
		t.Errorf("Content-Type = %q, want javascript: a module served as HTML will not parse", got)
	}
}

// TestTheAPIRoutesStillWinOverTheDashboard is the case that made the fallback a field
// on the handler rather than a wrapper around it: the dashboard's navigation uses the
// same shape of path as the API, so the wrong precedence loses the data.
func TestTheAPIRoutesStillWinOverTheDashboard(t *testing.T) {
	handler := servedDashboard(t, buildDashboard(t))

	// /meters/M-1 is an API route, and it must reach the data.
	api := request(handler, http.MethodGet, "/meters/M-1")
	if api.Code != http.StatusOK {
		t.Fatalf("GET /meters/M-1 = %d, want the API: %s", api.Code, api.Body)
	}
	if strings.Contains(api.Body.String(), "<title>Energy</title>") {
		t.Error("the dashboard answered an API path")
	}
	if !strings.Contains(api.Body.String(), "M-1") {
		t.Errorf("body = %s, want the meter's data", api.Body)
	}
	readings := request(handler, http.MethodGet, "/meters/M-1/readings")
	if readings.Code != http.StatusOK || !strings.Contains(readings.Header().Get("Content-Type"), "application/json") {
		t.Errorf("GET /meters/M-1/readings = %d %q, want JSON", readings.Code, readings.Header().Get("Content-Type"))
	}

	// /anomalies is both a route in the page and an API path. The API's registered
	// pattern wins, and the page reaches its own view through a path the API does not
	// claim.
	if listed := request(handler, http.MethodGet, "/anomalies"); listed.Code != http.StatusOK ||
		strings.Contains(listed.Body.String(), "<title>Energy</title>") {
		t.Errorf("GET /anomalies = %d %s, want the API's list", listed.Code, listed.Body)
	}
}

func TestAPathCannotEscapeTheBuiltDirectory(t *testing.T) {
	handler := servedDashboard(t, buildDashboard(t))

	// http.Dir refuses to serve anything above the directory it was given, and a
	// traversal must not become a way to read a file the build never contained.
	for _, attempt := range []string{
		"/../secret.txt",
		"/assets/../../secret.txt",
		"/..%2fsecret.txt",
		"/%2e%2e/go.mod",
	} {
		recorder := request(handler, http.MethodGet, attempt)
		if strings.Contains(recorder.Body.String(), "module github.com") {
			t.Errorf("GET %s read a file outside the build", attempt)
		}
	}
}
