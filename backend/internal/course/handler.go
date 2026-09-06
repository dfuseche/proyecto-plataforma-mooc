package course

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/mooc-platform/backend/internal/domain"
)

type HTTPHandler struct {
	useCase *UseCase
}

func NewHTTPHandler(uc *UseCase) *HTTPHandler {
	return &HTTPHandler{useCase: uc}
}

func (h *HTTPHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1/courses", func(r chi.Router) {
		r.Get("/", h.ListCourses)
		r.Post("/", h.CreateCourse)
		r.Get("/{id}", h.GetCourseHierarchy)
		r.Post("/versions/{versionId}/modules", h.AddModule)
		r.Post("/modules/{moduleId}/units", h.AddUnit)
		r.Post("/units/{unitId}/resources", h.AddResource)
		r.Post("/versions/{versionId}/publish", h.PublishVersion)
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

func (h *HTTPHandler) ListCourses(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(offsetStr)

	courses, err := h.useCase.GetCourseCatalog(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, courses)
}

func (h *HTTPHandler) CreateCourse(w http.ResponseWriter, r *http.Request) {
	teacherIDStr := r.Header.Get("X-Teacher-ID")
	if teacherIDStr == "" {
		teacherIDStr = r.Header.Get("X-Admin-ID")
	}

	var teacherID uuid.UUID
	var err error
	if teacherIDStr != "" {
		teacherID, err = uuid.Parse(teacherIDStr)
	}
	if err != nil || teacherID == uuid.Nil {
		// Mock ID por defecto para pruebas sencillas
		teacherID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	}

	var input CreateCourseInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	course, version, err := h.useCase.CreateCourse(r.Context(), teacherID, input)
	if err != nil {
		if err == domain.ErrUnauthorizedCourseMutation {
			respondError(w, http.StatusForbidden, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"course":  course,
		"version": version,
	})
}

func (h *HTTPHandler) GetCourseHierarchy(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	courseID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de curso inválido")
		return
	}

	hierarchy, err := h.useCase.GetCourseHierarchy(r.Context(), courseID)
	if err != nil {
		if err == domain.ErrCourseNotFound || err == domain.ErrVersionNotFound {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, hierarchy)
}

func (h *HTTPHandler) AddModule(w http.ResponseWriter, r *http.Request) {
	versionIDStr := chi.URLParam(r, "versionId")
	versionID, err := uuid.Parse(versionIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de versión inválido")
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Position    int    `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	mod, err := h.useCase.AddModule(r.Context(), uuid.Nil, versionID, req.Title, req.Description, req.Position)
	if err != nil {
		if err == domain.ErrVersionImmutable {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, mod)
}

func (h *HTTPHandler) AddUnit(w http.ResponseWriter, r *http.Request) {
	moduleIDStr := chi.URLParam(r, "moduleId")
	moduleID, err := uuid.Parse(moduleIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de módulo inválido")
		return
	}

	var req struct {
		Title    string `json:"title"`
		Position int    `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	unit, err := h.useCase.AddUnit(r.Context(), uuid.Nil, moduleID, req.Title, req.Position)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, unit)
}

func (h *HTTPHandler) AddResource(w http.ResponseWriter, r *http.Request) {
	unitIDStr := chi.URLParam(r, "unitId")
	unitID, err := uuid.Parse(unitIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de unidad inválido")
		return
	}

	var res domain.Resource
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		respondError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}
	res.UnitID = unitID

	createdRes, err := h.useCase.AddResource(r.Context(), uuid.Nil, &res)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, createdRes)
}

func (h *HTTPHandler) PublishVersion(w http.ResponseWriter, r *http.Request) {
	versionIDStr := chi.URLParam(r, "versionId")
	versionID, err := uuid.Parse(versionIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de versión inválido")
		return
	}

	pubVersion, err := h.useCase.PublishVersion(r.Context(), uuid.Nil, versionID)
	if err != nil {
		if err == domain.ErrInvalidPublishStructure {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err == domain.ErrVersionImmutable {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, pubVersion)
}
