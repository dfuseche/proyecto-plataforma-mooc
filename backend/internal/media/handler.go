package media

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type HTTPHandler struct {
	storage *StorageService
}

func NewHTTPHandler(storage *StorageService) *HTTPHandler {
	return &HTTPHandler{storage: storage}
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/media", func(r chi.Router) {
		r.Post("/presigned-upload", h.GeneratePresignedUpload)
		r.Get("/presigned-download", h.GeneratePresignedDownload)
	})
}

type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{Success: true, Data: data})
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(APIResponse{Success: false, Error: message})
}

func (h *HTTPHandler) GeneratePresignedUpload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ObjectKey   string `json:"object_key"`
		ContentType string `json:"content_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ObjectKey == "" {
		respondError(w, http.StatusBadRequest, "Se requiere object_key válido")
		return
	}

	out, err := h.storage.GeneratePresignedUploadURL(r.Context(), req.ObjectKey, req.ContentType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, out)
}

func (h *HTTPHandler) GeneratePresignedDownload(w http.ResponseWriter, r *http.Request) {
	objectKey := r.URL.Query().Get("object_key")
	if objectKey == "" {
		respondError(w, http.StatusBadRequest, "Se requiere el parámetro object_key")
		return
	}

	downloadURL, err := h.storage.GeneratePresignedDownloadURL(r.Context(), objectKey, 1*time.Hour)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"download_url": downloadURL,
	})
}
