package user

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
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", h.RegisterStudent)
		r.Post("/verify-email", h.VerifyEmail)
		r.Post("/login", h.Login)
	})

	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Post("/teachers", h.CreateTeacher)
		r.Post("/users/invite-teacher", h.CreateTeacher)
		r.Patch("/users/{id}/status", h.ChangeUserStatus)
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

func (h *HTTPHandler) RegisterStudent(w http.ResponseWriter, r *http.Request) {
	var input RegisterStudentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	user, token, err := h.useCase.RegisterStudent(r.Context(), input)
	if err != nil {
		if err == domain.ErrUserAlreadyExists {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"user":              user,
		"verification_code": token,
	})
}

func (h *HTTPHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		respondError(w, http.StatusBadRequest, "Se requiere el parámetro token")
		return
	}

	if err := h.useCase.VerifyEmail(r.Context(), req.Token); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Correo electrónico verificado exitosamente"})
}

func (h *HTTPHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	input.UserAgent = r.UserAgent()
	input.IPAddress = r.RemoteAddr

	output, err := h.useCase.Login(r.Context(), input)
	if err != nil {
		if err == domain.ErrInvalidCredentials || err == domain.ErrEmailNotVerified {
			respondError(w, http.StatusUnauthorized, err.Error())
			return
		}
		if err == domain.ErrUserSuspended {
			respondError(w, http.StatusForbidden, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, output)
}

func (h *HTTPHandler) CreateTeacher(w http.ResponseWriter, r *http.Request) {
	adminIDStr := r.Header.Get("X-Admin-ID")
	var adminID uuid.UUID
	var err error
	if adminIDStr != "" {
		adminID, err = uuid.Parse(adminIDStr)
	}
	if err != nil || adminID == uuid.Nil {
		// Usar ID del Admin inicial por defecto
		adminID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	}

	var input CreateTeacherInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	teacher, err := h.useCase.CreateTeacher(r.Context(), adminID, input)
	if err != nil {
		if err == domain.ErrForbidden {
			respondError(w, http.StatusForbidden, err.Error())
			return
		}
		if err == domain.ErrUserAlreadyExists {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, teacher)
}

func (h *HTTPHandler) ChangeUserStatus(w http.ResponseWriter, r *http.Request) {
	adminIDStr := r.Header.Get("X-Admin-ID")
	var adminID uuid.UUID
	var err error
	if adminIDStr != "" {
		adminID, err = uuid.Parse(adminIDStr)
	}
	if err != nil || adminID == uuid.Nil {
		adminID = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	}

	targetIDStr := chi.URLParam(r, "id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "ID de usuario inválido")
		return
	}

	var req struct {
		Status domain.UserStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Payload JSON inválido")
		return
	}

	if err := h.useCase.ChangeUserStatus(r.Context(), adminID, targetID, req.Status); err != nil {
		if err == domain.ErrLastAdminProtection {
			respondError(w, http.StatusConflict, err.Error())
			return
		}
		if err == domain.ErrForbidden {
			respondError(w, http.StatusForbidden, err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "Estado de usuario actualizado exitosamente"})
}
