package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"eddisonso.com/edd-compute/internal/auth"
	"eddisonso.com/edd-compute/internal/db"
	"eddisonso.com/edd-compute/internal/k8s"
)

type Handler struct {
	db        *db.DB
	k8s       *k8s.Client
	validator *auth.SessionValidator
	mux       *http.ServeMux
}

func NewHandler(database *db.DB, k8sClient *k8s.Client) http.Handler {
	h := &Handler{
		db:        database,
		k8s:       k8sClient,
		validator: auth.NewSessionValidator("http://simple-file-share-backend"),
		mux:       http.NewServeMux(),
	}

	// Health check (both paths for internal probes and external ingress access)
	h.mux.HandleFunc("GET /healthz", h.Healthz)
	h.mux.HandleFunc("GET /compute/healthz", h.Healthz)

	// Container endpoints
	h.mux.HandleFunc("GET /compute/containers", h.authMiddleware(h.ListContainers))
	h.mux.HandleFunc("POST /compute/containers", h.authMiddleware(h.CreateContainer))
	h.mux.HandleFunc("GET /compute/containers/{id}", h.authMiddleware(h.GetContainer))
	h.mux.HandleFunc("DELETE /compute/containers/{id}", h.authMiddleware(h.DeleteContainer))
	h.mux.HandleFunc("POST /compute/containers/{id}/stop", h.authMiddleware(h.StopContainer))
	h.mux.HandleFunc("POST /compute/containers/{id}/start", h.authMiddleware(h.StartContainer))

	// SSH key endpoints
	h.mux.HandleFunc("GET /compute/ssh-keys", h.authMiddleware(h.ListSSHKeys))
	h.mux.HandleFunc("POST /compute/ssh-keys", h.authMiddleware(h.AddSSHKey))
	h.mux.HandleFunc("DELETE /compute/ssh-keys/{id}", h.authMiddleware(h.DeleteSSHKey))

	// API key endpoints
	h.mux.HandleFunc("GET /compute/api-keys", h.authMiddleware(h.ListAPIKeys))
	h.mux.HandleFunc("POST /compute/api-keys", h.authMiddleware(h.CreateAPIKey))
	h.mux.HandleFunc("DELETE /compute/api-keys/{id}", h.authMiddleware(h.DeleteAPIKey))

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// authMiddleware validates session or API key and injects user info into context
func (h *Handler) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try session cookie first
		token := auth.GetSessionToken(r)
		if token != "" {
			username, err := h.validator.ValidateSession(token)
			if err != nil {
				slog.Error("session validation failed", "error", err)
				http.Error(w, "authentication error", http.StatusInternalServerError)
				return
			}
			if username != "" {
				// For now, use username as user ID (simplified)
				// In production, would lookup user ID from username
				r = r.WithContext(setUserContext(r.Context(), 1, username))
				next(w, r)
				return
			}
		}

		// Try API key
		apiKey := auth.GetAPIKeyFromRequest(r)
		if apiKey != "" {
			keyHash := auth.HashAPIKey(apiKey)
			key, err := h.db.GetAPIKeyByHash(keyHash)
			if err != nil {
				slog.Error("api key lookup failed", "error", err)
				http.Error(w, "authentication error", http.StatusInternalServerError)
				return
			}
			if key != nil {
				// Update last used
				_ = h.db.UpdateAPIKeyLastUsed(key.ID)
				r = r.WithContext(setUserContext(r.Context(), key.UserID, ""))
				next(w, r)
				return
			}
		}

		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

func writeError(w http.ResponseWriter, message string, code int) {
	http.Error(w, message, code)
}
