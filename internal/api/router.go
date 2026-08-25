package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Ribco/clicapot/internal/auth"
)

type Server struct {
	db *sql.DB
}

func NewRouter(db *sql.DB) http.Handler {
	s := &Server{db: db}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/v1/status", s.statusHandler)

	mux.HandleFunc("/api/v1/auth/register", s.registerHandler)
	mux.HandleFunc("/api/v1/auth/login", s.loginHandler)
	mux.HandleFunc("/api/v1/auth/logout", s.logoutHandler)

	mux.HandleFunc("/api/v1/me", s.meHandler)

	return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "clicapot",
		"status":  "ok",
		"version": "0.1.0",
	})
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service": "clicapot",
		"status":  "operational",
		"version": "0.1.0",
	})
}

type registerRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) registerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	var req registerRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON",
		})
		return
	}

	user, err := auth.Register(
		s.db,
		req.Email,
		req.Username,
		req.Password,
	)

	if err != nil {
		status := http.StatusBadRequest

		if err == auth.ErrUserExists {
			status = http.StatusConflict
		}

		writeJSON(w, status, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"user": user,
	})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
		return
	}

	var req loginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON",
		})
		return
	}

	user, token, err := auth.Login(
		s.db,
		req.Email,
		req.Password,
	)

	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "invalid credentials",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user":  user,
		"token": token,
	})
}

func (s *Server) logoutHandler(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)

	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "missing token",
		})
		return
	}

	if err := auth.Logout(s.db, token); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "logout failed",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "logged out",
	})
}

func (s *Server) meHandler(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)

	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "missing token",
		})
		return
	}

	user, err := auth.GetUserByToken(s.db, token)

	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "invalid session",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": user,
	})
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")

	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
