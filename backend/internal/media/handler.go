package media

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/mooc-platform/backend/internal/domain"
	"github.com/mooc-platform/backend/internal/middleware"
	"github.com/mooc-platform/backend/internal/worker"
)

type HTTPHandler struct {
	storage      *StorageService
	courseRepo   domain.CourseRepository
	learningRepo domain.LearningRepository
	asynqClient  *asynq.Client
}

func NewHTTPHandler(storage *StorageService, courseRepo domain.CourseRepository, learningRepo domain.LearningRepository, asynqClient *asynq.Client) *HTTPHandler {
	return &HTTPHandler{
		storage:      storage,
		courseRepo:   courseRepo,
		learningRepo: learningRepo,
		asynqClient:  asynqClient,
	}
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/media", func(r chi.Router) {
		r.Post("/presigned-upload", h.GeneratePresignedUpload)
		r.Get("/presigned-download", h.GeneratePresignedDownload)
		r.Post("/resources/{resourceId}/complete-upload", h.CompleteUpload)
		r.Get("/resources/{resourceId}/stream-url", h.GetStreamURL)
		r.Get("/resources/{resourceId}/manifest.m3u8", h.GetSignedManifest)
		r.Get("/resources/{resourceId}/resume", h.GetResumePosition)
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

// publicBaseURL reconstruye el esquema+host públicos con los que se llamó a
// la API, respetando cabeceras de un proxy/balanceador si están presentes.
// Se usa para devolver URLs absolutas hacia endpoints propios (como el
// manifiesto HLS firmado), en vez de asumir un host fijo.
func publicBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}

	return fmt.Sprintf("%s://%s", scheme, host)
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

func (h *HTTPHandler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	resourceIDStr := chi.URLParam(r, "resourceId")
	resourceID, err := uuid.Parse(resourceIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de recurso inválido")
		return
	}

	var req struct {
		ObjectKey string `json:"object_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ObjectKey == "" {
		respondError(w, http.StatusBadRequest, "Se requiere object_key válido")
		return
	}

	// 1. Validar que el objeto existe en S3
	stat, err := h.storage.StatObject(r.Context(), req.ObjectKey)
	if err != nil {
		respondError(w, http.StatusBadRequest, "El archivo no se encuentra disponible en almacenamiento S3")
		return
	}

	// 2. Obtener recurso y actualizar estado a processing
	res, err := h.courseRepo.GetResourceByID(r.Context(), resourceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Recurso no encontrado")
		return
	}

	res.MediaURL = req.ObjectKey
	res.ProcessingStatus = domain.ProcessingInProgress
	if err := h.courseRepo.UpdateResource(r.Context(), res); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 3. Encolar tarea asíncrona en Asynq
	if h.asynqClient != nil && (res.Type == domain.ResourceTypeVideo || res.Type == domain.ResourceTypeAudio) {
		task, err := worker.NewMediaTranscodeHLSTask(res.ID, req.ObjectKey, string(res.Type))
		if err == nil {
			_, _ = h.asynqClient.Enqueue(task)
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"message":           "Carga validada exitosamente; procesamiento encolado",
		"resource_id":       res.ID,
		"object_size_bytes": stat.Size,
		"processing_status": res.ProcessingStatus,
	})
}

func (h *HTTPHandler) GetStreamURL(w http.ResponseWriter, r *http.Request) {
	resourceIDStr := chi.URLParam(r, "resourceId")
	resourceID, err := uuid.Parse(resourceIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de recurso inválido")
		return
	}

	res, err := h.courseRepo.GetResourceByID(r.Context(), resourceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Recurso no encontrado")
		return
	}

	authUser := middleware.GetUserFromContext(r.Context())
	if authUser == nil {
		// Fallback para dev/test si no viene token
		authUser = &domain.User{ID: uuid.New(), Role: domain.RoleStudent}
	}

	var streamURL string
	if IsHLSManifestKey(res.MediaURL) {
		// res.MediaURL es un manifiesto HLS (hls/<id>/master.m3u8). Un
		// presigned URL solo firma ESE objeto: los segmentos .ts que el
		// manifiesto referencia con rutas relativas son objetos aparte en un
		// bucket privado, así que el reproductor no podría bajarlos. En vez
		// de firmar el manifiesto directamente, devolvemos la URL de nuestro
		// propio endpoint, que lo reescribe firmando cada segmento al vuelo.
		streamURL = fmt.Sprintf("%s/api/v1/media/resources/%s/manifest.m3u8", publicBaseURL(r), res.ID)
	} else {
		streamURL, err = h.storage.GeneratePresignedDownloadURL(r.Context(), res.MediaURL, 2*time.Hour)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"resource_id":   res.ID,
		"title":         res.Title,
		"type":          res.Type,
		"presigned_url": streamURL,
	})
}

// GetSignedManifest sirve el manifiesto HLS de un recurso ya transcodificado,
// con cada línea de segmento/sub-playlist reescrita como una URL prefirmada
// de S3/MinIO. Ver el comentario en GetStreamURL para el porqué.
func (h *HTTPHandler) GetSignedManifest(w http.ResponseWriter, r *http.Request) {
	resourceIDStr := chi.URLParam(r, "resourceId")
	resourceID, err := uuid.Parse(resourceIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de recurso inválido")
		return
	}

	res, err := h.courseRepo.GetResourceByID(r.Context(), resourceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Recurso no encontrado")
		return
	}

	if res.ProcessingStatus != domain.ProcessingCompleted || !IsHLSManifestKey(res.MediaURL) {
		respondError(w, http.StatusConflict, "El recurso no tiene un manifiesto HLS disponible")
		return
	}

	body, err := h.storage.RewriteHLSManifest(r.Context(), res.MediaURL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "No se pudo generar el manifiesto firmado")
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	// El contenido trae URLs firmadas con expiración propia; no debe cachearse.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *HTTPHandler) GetResumePosition(w http.ResponseWriter, r *http.Request) {
	resourceIDStr := chi.URLParam(r, "resourceId")
	resourceID, err := uuid.Parse(resourceIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de recurso inválido")
		return
	}

	res, err := h.courseRepo.GetResourceByID(r.Context(), resourceID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Recurso no encontrado")
		return
	}

	authUser := middleware.GetUserFromContext(r.Context())
	var lastPosition int = 0
	if authUser != nil && h.learningRepo != nil {
		progressList, err := h.learningRepo.GetStudentCourseProgress(r.Context(), authUser.ID, uuid.Nil)
		if err == nil {
			for _, p := range progressList {
				if p.ResourceStableID == res.StableID {
					lastPosition = p.LastPositionSeconds
					break
				}
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"resource_id":           res.ID,
		"stable_id":             res.StableID,
		"last_position_seconds": lastPosition,
	})
}
