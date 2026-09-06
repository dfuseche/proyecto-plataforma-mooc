package learning

import (
	"encoding/json"
	"net/http"

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
	r.Route("/api/v1/learning", func(r chi.Router) {
		r.Post("/enrollments", h.EnrollStudent)
		r.Post("/quizzes/{quizId}/attempts", h.SubmitQuizAttempt)
		r.Post("/heartbeat", h.RecordHeartbeat)
	})

	r.Route("/api/v1/badges", func(r chi.Router) {
		r.Get("/verify/{code}", h.VerifyBadge)
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

func (h *HTTPHandler) EnrollStudent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StudentID string `json:"student_id"`
		CourseID  string `json:"course_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	studentID, err1 := uuid.Parse(req.StudentID)
	courseID, err2 := uuid.Parse(req.CourseID)
	if err1 != nil || err2 != nil {
		respondError(w, http.StatusBadRequest, "IDs de estudiante o curso inválidos")
		return
	}

	enrollment, err := h.useCase.EnrollStudent(r.Context(), studentID, courseID)
	if err != nil {
		if err == domain.ErrCourseNotFound {
			respondError(w, http.StatusNotFound, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, enrollment)
}

func (h *HTTPHandler) SubmitQuizAttempt(w http.ResponseWriter, r *http.Request) {
	quizIDStr := chi.URLParam(r, "quizId")
	quizID, err := uuid.Parse(quizIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de quiz inválido")
		return
	}

	var req struct {
		StudentID string            `json:"student_id"`
		Answers   map[string]string `json:"answers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	studentID, err := uuid.Parse(req.StudentID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de estudiante inválido")
		return
	}

	attempt, err := h.useCase.SubmitQuizAttempt(r.Context(), studentID, quizID, req.Answers)
	if err != nil {
		if err == domain.ErrMaxAttemptsReached {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, attempt)
}

func (h *HTTPHandler) RecordHeartbeat(w http.ResponseWriter, r *http.Request) {
	var input HeartbeatInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	enrollment, err := h.useCase.RecordHeartbeat(r.Context(), input)
	if err != nil {
		if err == domain.ErrNotEnrolled {
			respondError(w, http.StatusForbidden, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, enrollment)
}

func (h *HTTPHandler) VerifyBadge(w http.ResponseWriter, r *http.Request) {
	codeStr := chi.URLParam(r, "code")
	code, err := uuid.Parse(codeStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Código de verificación de insignia inválido")
		return
	}

	badge, err := h.useCase.VerifyBadge(r.Context(), code)
	if err != nil {
		respondError(w, http.StatusNotFound, "Insignia no encontrada o inválida")
		return
	}

	respondJSON(w, http.StatusOK, badge)
}
