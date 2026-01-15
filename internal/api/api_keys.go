package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"eddisonso.com/edd-compute/internal/auth"
	"eddisonso.com/edd-compute/internal/db"
)

const maxAPIKeysPerUser = 5

type apiKeyRequest struct {
	Name string `json:"name"`
}

type apiKeyResponse struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Key       *string `json:"key,omitempty"` // Only returned on creation
	CreatedAt string  `json:"created_at"`
	LastUsed  *string `json:"last_used,omitempty"`
}

func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := getUserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	keys, err := h.db.ListAPIKeysByUser(userID)
	if err != nil {
		slog.Error("failed to list api keys", "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := make([]apiKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, apiKeyToResponse(k, false))
	}

	writeJSON(w, resp)
}

func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := getUserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req apiKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, "name is required", http.StatusBadRequest)
		return
	}

	// Check key limit
	count, err := h.db.CountAPIKeysByUser(userID)
	if err != nil {
		slog.Error("failed to count api keys", "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if count >= maxAPIKeysPerUser {
		writeError(w, fmt.Sprintf("API key limit reached (%d)", maxAPIKeysPerUser), http.StatusBadRequest)
		return
	}

	// Generate new API key
	plaintext, keyHash, err := auth.GenerateAPIKey()
	if err != nil {
		slog.Error("failed to generate api key", "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	key := &db.APIKey{
		UserID:  userID,
		KeyHash: keyHash,
		Name:    req.Name,
	}

	if err := h.db.CreateAPIKey(key); err != nil {
		slog.Error("failed to create api key", "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Return response with plaintext key (only time it's shown)
	resp := apiKeyToResponse(key, true)
	resp.Key = &plaintext

	writeJSON(w, resp)
}

func (h *Handler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := getUserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteAPIKey(id, userID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "API key not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to delete api key", "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}

func apiKeyToResponse(k *db.APIKey, includeKey bool) apiKeyResponse {
	resp := apiKeyResponse{
		ID:        k.ID,
		Name:      k.Name,
		CreatedAt: k.CreatedAt.Format(time.RFC3339),
	}

	if k.LastUsed.Valid {
		lastUsed := k.LastUsed.Time.Format(time.RFC3339)
		resp.LastUsed = &lastUsed
	}

	return resp
}
