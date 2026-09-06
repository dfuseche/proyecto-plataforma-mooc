package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type EnrollmentStatus string
type AcademicStatus string
type AttemptStatus string
type ProgressStatus string

const (
	EnrollmentActive    EnrollmentStatus = "active"
	EnrollmentWithdrawn EnrollmentStatus = "withdrawn"
)

const (
	AcademicEnrolled  AcademicStatus = "enrolled"
	AcademicCompleted AcademicStatus = "completed"
	AcademicApproved  AcademicStatus = "approved"
)

const (
	AttemptInProgress AttemptStatus = "in_progress"
	AttemptSubmitted  AttemptStatus = "submitted"
	AttemptExpired    AttemptStatus = "expired"
)

const (
	ProgressOpened    ProgressStatus = "opened"
	ProgressCompleted ProgressStatus = "completed"
)

var (
	ErrNotEnrolled             = errors.New("el estudiante no está inscrito en este curso")
	ErrAlreadyEnrolled         = errors.New("el estudiante ya se encuentra inscrito en este curso")
	ErrMaxAttemptsReached      = errors.New("se ha alcanzado el límite máximo de intentos para este quiz")
	ErrAttemptAlreadySubmitted = errors.New("este intento de quiz ya fue enviado y calificado")
	ErrClientProgressRejected  = errors.New("los porcentajes de avance enviados por el cliente son rechazados por seguridad; se requiere validación por heartbeats en servidor")
	ErrBadgeAlreadyIssued      = errors.New("la insignia para este curso ya fue emitida previamente")
)

type Enrollment struct {
	ID                 uuid.UUID        `json:"id"`
	StudentID          uuid.UUID        `json:"student_id"`
	CourseID           uuid.UUID        `json:"course_id"`
	Status             EnrollmentStatus `json:"status"`
	AcademicStatus     AcademicStatus   `json:"academic_status"`
	ProgressPercentage float64          `json:"progress_percentage"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

type Quiz struct {
	ID           uuid.UUID      `json:"id"`
	ResourceID   uuid.UUID      `json:"resource_id"`
	MaxAttempts  int            `json:"max_attempts"`
	PassingScore float64        `json:"passing_score"`
	CreatedAt    time.Time      `json:"created_at"`
	Questions    []QuizQuestion `json:"questions,omitempty"`
}

type QuizQuestion struct {
	ID           uuid.UUID    `json:"id"`
	QuizID       uuid.UUID    `json:"quiz_id"`
	QuestionText string       `json:"question_text"`
	Position     int          `json:"position"`
	Points       float64      `json:"points"`
	CreatedAt    time.Time    `json:"created_at"`
	Options      []QuizOption `json:"options,omitempty"`
}

// QuizOption contiene la respuesta correcta internamente
type QuizOption struct {
	ID         uuid.UUID `json:"id"`
	QuestionID uuid.UUID `json:"question_id"`
	OptionText string    `json:"option_text"`
	IsCorrect  bool      `json:"-"` // Oculto explícitamente de JSON para clientes
	Feedback   string    `json:"feedback,omitempty"`
	Position   int       `json:"position"`
}

// StudentQuizOption es el DTO seguro enviado al estudiante donde is_correct NUNCA existe
type StudentQuizOption struct {
	ID         uuid.UUID `json:"id"`
	QuestionID uuid.UUID `json:"question_id"`
	OptionText string    `json:"option_text"`
	Position   int       `json:"position"`
}

type QuizAttempt struct {
	ID            uuid.UUID         `json:"id"`
	StudentID     uuid.UUID         `json:"student_id"`
	QuizID        uuid.UUID         `json:"quiz_id"`
	AttemptNumber int               `json:"attempt_number"`
	Status        AttemptStatus     `json:"status"`
	Score         *float64          `json:"score,omitempty"`
	Answers       map[string]string `json:"answers"` // question_id -> option_id
	SubmittedAt   *time.Time        `json:"submitted_at,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
}

type ResourceProgress struct {
	ID                  uuid.UUID      `json:"id"`
	StudentID           uuid.UUID      `json:"student_id"`
	CourseID            uuid.UUID      `json:"course_id"`
	ResourceStableID    uuid.UUID      `json:"resource_stable_id"`
	Status              ProgressStatus `json:"status"`
	DwellTimeSeconds    int            `json:"dwell_time_seconds"`
	LastPositionSeconds int            `json:"last_position_seconds"`
	LastHeartbeatAt     time.Time      `json:"last_heartbeat_at"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
}

type Badge struct {
	ID               uuid.UUID `json:"id"`
	StudentID        uuid.UUID `json:"student_id"`
	CourseID         uuid.UUID `json:"course_id"`
	CourseVersionID  uuid.UUID `json:"course_version_id"`
	VerificationCode uuid.UUID `json:"verification_code"`
	ImageURL         string    `json:"image_url"`
	VerificationURL  string    `json:"verification_url"`
	IsRevoked        bool      `json:"is_revoked"`
	IssuedAt         time.Time `json:"issued_at"`
}

type LearningRepository interface {
	EnrollStudent(ctx context.Context, enrollment *Enrollment) error
	GetEnrollment(ctx context.Context, studentID, courseID uuid.UUID) (*Enrollment, error)
	UpdateEnrollment(ctx context.Context, enrollment *Enrollment) error

	CreateQuiz(ctx context.Context, quiz *Quiz) error
	GetQuizByResourceID(ctx context.Context, resourceID uuid.UUID) (*Quiz, error)
	GetQuizWithAnswers(ctx context.Context, quizID uuid.UUID) (*Quiz, error)

	CreateQuizAttempt(ctx context.Context, attempt *QuizAttempt) error
	GetAttemptByID(ctx context.Context, attemptID uuid.UUID) (*QuizAttempt, error)
	GetStudentAttemptsCount(ctx context.Context, studentID, quizID uuid.UUID) (int, error)
	UpdateQuizAttempt(ctx context.Context, attempt *QuizAttempt) error

	UpsertResourceProgress(ctx context.Context, progress *ResourceProgress) error
	GetStudentCourseProgress(ctx context.Context, studentID, courseID uuid.UUID) ([]*ResourceProgress, error)

	CreateBadge(ctx context.Context, badge *Badge) error
	GetBadgeByCode(ctx context.Context, code uuid.UUID) (*Badge, error)
	GetStudentBadgeForCourse(ctx context.Context, studentID, courseID uuid.UUID) (*Badge, error)
}
