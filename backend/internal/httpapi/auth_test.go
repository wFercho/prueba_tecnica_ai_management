package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/analysis"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/service"
	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/store/memory"
)

func TestLoginProtectsDataAndLogoutRevokesCookie(t *testing.T) {
	ctx := t.Context()
	data := memory.New()
	svc := service.New(data, nil, analysis.DefaultDetectorConfig())
	if err := svc.ProvisionUser(ctx, "admin@email.com", "admin"); err != nil {
		t.Fatal(err)
	}
	handler := New(svc, discardLogger())
	request := func(method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(payload))
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if got := request("GET", "/meters", nil, nil).Code; got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated meters = %d, want 401", got)
	}
	if got := request("POST", "/auth/login", map[string]string{"email": "admin@email.com", "password": "wrong"}, nil).Code; got != http.StatusUnauthorized {
		t.Fatalf("bad password = %d, want 401", got)
	}
	login := request("POST", "/auth/login", map[string]string{"email": "admin@email.com", "password": "admin"}, nil)
	if login.Code != http.StatusOK || len(login.Result().Cookies()) != 1 {
		t.Fatalf("login = %d, cookies = %v", login.Code, login.Result().Cookies())
	}
	cookie := login.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Value == "" {
		t.Fatalf("insecure login cookie: %+v", cookie)
	}
	if got := request("GET", "/meters", nil, cookie).Code; got != http.StatusOK {
		t.Fatalf("authorized meters = %d, want 200", got)
	}
	if got := request("POST", "/auth/logout", nil, cookie).Code; got != http.StatusNoContent {
		t.Fatalf("logout = %d, want 204", got)
	}
	if got := request("GET", "/meters", nil, cookie).Code; got != http.StatusUnauthorized {
		t.Fatalf("revoked cookie = %d, want 401", got)
	}
	if got := request("POST", "/auth/logout", nil, cookie).Code; got != http.StatusNoContent {
		t.Fatalf("second logout = %d, want 204", got)
	}
}
