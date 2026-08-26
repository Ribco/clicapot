package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/Ribco/clicapot/internal/apikeys"
	"github.com/Ribco/clicapot/internal/auth"
	"github.com/Ribco/clicapot/internal/projects"
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

	mux.HandleFunc("/api/v1/projects", s.projectsHandler)
	mux.HandleFunc("/api/v1/projects/", s.projectHandler)
	mux.HandleFunc("/api/v1/api-keys", s.apiKeysHandler)
	mux.HandleFunc("/api/v1/api-keys/", s.apiKeyHandler)

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
		"services": map[string]string{
			"api":          "operational",
			"database":     "operational",
			"dns":          "planned",
			"edge":         "planned",
			"shield":       "planned",
			"compute":      "planned",
			"pages":        "planned",
			"storage":      "planned",
			"kv":           "planned",
			"tunnel":       "planned",
			"access":       "planned",
			"loadbalancer": "planned",
			"analytics":    "planned",
			"queues":       "planned",
			"scheduler":    "planned",
		},
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

	user, err := auth.Register(s.db, req.Email, req.Username, req.Password)
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

	user, token, err := auth.Login(s.db, req.Email, req.Password)
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
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": user,
	})
}

type projectRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (s *Server) projectsHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		result, err := projects.List(s.db, user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to list projects",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"projects": result,
		})

	case http.MethodPost:
		var req projectRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "invalid JSON",
			})
			return
		}

		project, err := projects.Create(s.db, user.ID, req.Name, req.Slug)

		if err != nil {
			status := http.StatusBadRequest

			if err == projects.ErrExists {
				status = http.StatusConflict
			}

			writeJSON(w, status, map[string]string{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"project": project,
		})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
	}
}

func (s *Server) projectHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	idText := strings.TrimPrefix(r.URL.Path, "/api/v1/projects/")

	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid project id",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		project, err := projects.Get(s.db, user.ID, id)

		if err == projects.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "project not found",
			})
			return
		}

		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to get project",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"project": project,
		})

	case http.MethodDelete:
		err := projects.Delete(s.db, user.ID, id)

		if err == projects.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "project not found",
			})
			return
		}

		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "failed to delete project",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "deleted",
		})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error": "method not allowed",
		})
	}
}

type createAPIKeyRequest struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

func (s *Server) apiKeysHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		keys, err := apikeys.List(s.db, user.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list API keys"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"api_keys": keys})

	case http.MethodPost:
		var req createAPIKeyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		apiKey, rawKey, err := apikeys.Create(s.db, user.ID, req.Name, req.Scopes)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"api_key": apiKey,
			"key":     rawKey,
		})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) apiKeyHandler(w http.ResponseWriter, r *http.Request) {
	user, ok := s.requireUser(w, r)
	if !ok {
		return
	}

	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	idText := strings.TrimPrefix(r.URL.Path, "/api/v1/api-keys/")
	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid API key id"})
		return
	}

	if err := apikeys.Delete(s.db, user.ID, id); err != nil {
		if err == apikeys.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "API key not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete API key"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	token := bearerToken(r)

	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "missing token",
		})
		return auth.User{}, false
	}

	// Machine/API key authentication.
	if strings.HasPrefix(token, "cp_") {
		userID, err := apikeys.Authenticate(s.db, token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "invalid API key",
			})
			return auth.User{}, false
		}

		var user auth.User

		err = s.db.QueryRow(`
			SELECT id, email, username, created_at
			FROM users
			WHERE id = ?
		`, userID).Scan(
			&user.ID,
			&user.Email,
			&user.Username,
			&user.CreatedAt,
		)

		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "user not found",
			})
			return auth.User{}, false
		}

		return user, true
	}

	// Normal user session authentication.
	user, err := auth.GetUserByToken(s.db, token)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "invalid session",
		})
		return auth.User{}, false
	}

	return user, true
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
