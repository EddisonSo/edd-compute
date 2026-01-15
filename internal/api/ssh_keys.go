package api

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"eddisonso.com/edd-compute/internal/db"
)

const maxSSHKeysPerUser = 10

type sshKeyRequest struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

type sshKeyResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	CreatedAt   string `json:"created_at"`
}

func (h *Handler) ListSSHKeys(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := getUserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	keys, err := h.db.ListSSHKeysByUser(userID)
	if err != nil {
		slog.Error("failed to list ssh keys", "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp := make([]sshKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, sshKeyToResponse(k))
	}

	writeJSON(w, resp)
}

func (h *Handler) AddSSHKey(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := getUserFromContext(r.Context())
	if !ok {
		writeError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req sshKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.PublicKey = strings.TrimSpace(req.PublicKey)

	if req.Name == "" {
		writeError(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.PublicKey == "" {
		writeError(w, "public_key is required", http.StatusBadRequest)
		return
	}

	// Validate SSH public key format
	if !isValidSSHPublicKey(req.PublicKey) {
		writeError(w, "invalid SSH public key format", http.StatusBadRequest)
		return
	}

	// Check key limit
	count, err := h.db.CountSSHKeysByUser(userID)
	if err != nil {
		slog.Error("failed to count ssh keys", "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}
	if count >= maxSSHKeysPerUser {
		writeError(w, fmt.Sprintf("SSH key limit reached (%d)", maxSSHKeysPerUser), http.StatusBadRequest)
		return
	}

	fingerprint := sshKeyFingerprint(req.PublicKey)

	key := &db.SSHKey{
		UserID:      userID,
		Name:        req.Name,
		PublicKey:   req.PublicKey,
		Fingerprint: fingerprint,
	}

	if err := h.db.CreateSSHKey(key); err != nil {
		slog.Error("failed to create ssh key", "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, sshKeyToResponse(key))
}

func (h *Handler) DeleteSSHKey(w http.ResponseWriter, r *http.Request) {
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

	if err := h.db.DeleteSSHKey(id, userID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, "SSH key not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to delete ssh key", "error", err)
		writeError(w, "internal error", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"status": "ok"})
}

func sshKeyToResponse(k *db.SSHKey) sshKeyResponse {
	return sshKeyResponse{
		ID:          k.ID,
		Name:        k.Name,
		Fingerprint: k.Fingerprint,
		CreatedAt:   k.CreatedAt.Format(time.RFC3339),
	}
}

func isValidSSHPublicKey(key string) bool {
	parts := strings.Fields(key)
	if len(parts) < 2 {
		return false
	}

	keyType := parts[0]
	validTypes := []string{"ssh-rsa", "ssh-ed25519", "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521", "ssh-dss"}
	valid := false
	for _, t := range validTypes {
		if keyType == t {
			valid = true
			break
		}
	}
	if !valid {
		return false
	}

	// Try to decode the key data
	_, err := base64.StdEncoding.DecodeString(parts[1])
	return err == nil
}

func sshKeyFingerprint(key string) string {
	parts := strings.Fields(key)
	if len(parts) < 2 {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	hash := md5.Sum(decoded)
	fingerprint := make([]string, len(hash))
	for i, b := range hash {
		fingerprint[i] = fmt.Sprintf("%02x", b)
	}

	return strings.Join(fingerprint, ":")
}
