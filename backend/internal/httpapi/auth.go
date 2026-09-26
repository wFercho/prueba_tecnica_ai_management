package httpapi

import (
	"errors"
	"net/http"

	"github.com/wFercho/prueba_tecnica_ai_management/backend/internal/service"
)

const sessionCookie = "energy_session"

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var credentials struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &credentials); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Introduce un correo y una contraseña válidos.")
		return
	}
	user, token, err := h.service.Login(r.Context(), credentials.Email, credentials.Password)
	if errors.Is(err, service.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Correo o contraseña incorrectos.")
		return
	}
	if err != nil {
		h.fail(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", MaxAge: int(service.SessionLifetime.Seconds()),
		HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"user": map[string]any{"email": user.Email}})
}

func (h *Handler) session(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookie) // verified by ServeHTTP before routing
	user, err := h.service.SessionUser(r.Context(), cookie.Value)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": map[string]any{"email": user.Email}})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true,
		Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}
