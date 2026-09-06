package learning

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mooc-platform/backend/internal/domain"
)

type UseCase struct {
	repo       domain.LearningRepository
	courseRepo domain.CourseRepository
	userRepo   domain.UserRepository
	baseURL    string
}

func NewUseCase(repo domain.LearningRepository, courseRepo domain.CourseRepository, userRepo domain.UserRepository, baseURL string) *UseCase {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &UseCase{
		repo:       repo,
		courseRepo: courseRepo,
		userRepo:   userRepo,
		baseURL:    baseURL,
	}
}

func (uc *UseCase) EnrollStudent(ctx context.Context, studentID, courseID uuid.UUID) (*domain.Enrollment, error) {
	_, err := uc.courseRepo.GetCourseByID(ctx, courseID)
	if err != nil {
		return nil, err
	}

	enrollment := &domain.Enrollment{
		ID:                 uuid.New(),
		StudentID:          studentID,
		CourseID:           courseID,
		Status:             domain.EnrollmentActive,
		AcademicStatus:     domain.AcademicEnrolled,
		ProgressPercentage: 0.0,
	}

	if err := uc.repo.EnrollStudent(ctx, enrollment); err != nil {
		return nil, err
	}

	return enrollment, nil
}

func (uc *UseCase) SubmitQuizAttempt(ctx context.Context, studentID, quizID uuid.UUID, answers map[string]string) (*domain.QuizAttempt, error) {
	quiz, err := uc.repo.GetQuizWithAnswers(ctx, quizID)
	if err != nil {
		return nil, err
	}

	attemptsCount, err := uc.repo.GetStudentAttemptsCount(ctx, studentID, quizID)
	if err != nil {
		return nil, err
	}

	if attemptsCount >= quiz.MaxAttempts {
		return nil, domain.ErrMaxAttemptsReached
	}

	// Calificación en Servidor de forma reproducible
	var totalPoints float64 = 0
	var earnedPoints float64 = 0

	for _, quest := range quiz.Questions {
		totalPoints += quest.Points

		selectedOptionIDStr, answered := answers[quest.ID.String()]
		if !answered {
			continue
		}

		selectedOptionID, err := uuid.Parse(selectedOptionIDStr)
		if err != nil {
			continue
		}

		for _, opt := range quest.Options {
			if opt.ID == selectedOptionID && opt.IsCorrect {
				earnedPoints += quest.Points
				break
			}
		}
	}

	var score float64 = 0
	if totalPoints > 0 {
		score = (earnedPoints / totalPoints) * 100.0
	}

	now := time.Now()
	attempt := &domain.QuizAttempt{
		ID:            uuid.New(),
		StudentID:     studentID,
		QuizID:        quizID,
		AttemptNumber: attemptsCount + 1,
		Status:        domain.AttemptSubmitted,
		Score:         &score,
		Answers:       answers,
		SubmittedAt:   &now,
	}

	if err := uc.repo.CreateQuizAttempt(ctx, attempt); err != nil {
		return nil, err
	}

	return attempt, nil
}

type HeartbeatInput struct {
	StudentID           uuid.UUID `json:"student_id"`
	CourseID            uuid.UUID `json:"course_id"`
	ResourceStableID    uuid.UUID `json:"resource_stable_id"`
	DwellTimeSeconds    int       `json:"dwell_time_seconds"`
	LastPositionSeconds int       `json:"last_position_seconds"`
}

func (uc *UseCase) RecordHeartbeat(ctx context.Context, input HeartbeatInput) (*domain.Enrollment, error) {
	enrollment, err := uc.repo.GetEnrollment(ctx, input.StudentID, input.CourseID)
	if err != nil {
		return nil, domain.ErrNotEnrolled
	}

	// Mínimo de permanencia de 10 segundos para considerar el recurso como completado
	status := domain.ProgressOpened
	if input.DwellTimeSeconds >= 10 {
		status = domain.ProgressCompleted
	}

	prog := &domain.ResourceProgress{
		ID:                  uuid.New(),
		StudentID:           input.StudentID,
		CourseID:            input.CourseID,
		ResourceStableID:    input.ResourceStableID,
		Status:              status,
		DwellTimeSeconds:    input.DwellTimeSeconds,
		LastPositionSeconds: input.LastPositionSeconds,
	}

	if err := uc.repo.UpsertResourceProgress(ctx, prog); err != nil {
		return nil, err
	}

	// Recalcular progreso global basado en recursos obligatorios de la versión publicada actual
	course, err := uc.courseRepo.GetCourseByID(ctx, input.CourseID)
	if err != nil || course.CurrentPublishedVersionID == nil {
		return enrollment, nil
	}

	version, err := uc.courseRepo.GetFullVersionHierarchy(ctx, *course.CurrentPublishedVersionID)
	if err != nil {
		return enrollment, nil
	}

	mandatoryResourceStableIDs := make(map[uuid.UUID]bool)
	for _, mod := range version.Modules {
		for _, unit := range mod.Units {
			for _, res := range unit.Resources {
				if res.IsVisible && res.IsMandatory {
					mandatoryResourceStableIDs[res.StableID] = true
				}
			}
		}
	}

	totalMandatory := len(mandatoryResourceStableIDs)
	if totalMandatory == 0 {
		return enrollment, nil
	}

	studentProgress, err := uc.repo.GetStudentCourseProgress(ctx, input.StudentID, input.CourseID)
	if err != nil {
		return nil, err
	}

	completedCount := 0
	for _, p := range studentProgress {
		if p.Status == domain.ProgressCompleted && mandatoryResourceStableIDs[p.ResourceStableID] {
			completedCount++
		}
	}

	newPercentage := (float64(completedCount) / float64(totalMandatory)) * 100.0
	enrollment.ProgressPercentage = newPercentage

	if newPercentage >= 100.0 {
		enrollment.AcademicStatus = domain.AcademicApproved
		_, _ = uc.IssueBadge(ctx, input.StudentID, input.CourseID, version.ID)
	}

	if err := uc.repo.UpdateEnrollment(ctx, enrollment); err != nil {
		return nil, err
	}

	return enrollment, nil
}

func (uc *UseCase) IssueBadge(ctx context.Context, studentID, courseID, versionID uuid.UUID) (*domain.Badge, error) {
	existing, err := uc.repo.GetStudentBadgeForCourse(ctx, studentID, courseID)
	if err == nil && existing != nil {
		return existing, nil // Idempotente: retorna la insignia previa
	}

	verificationCode := uuid.New()
	verificationURL := fmt.Sprintf("%s/api/v1/badges/verify/%s", uc.baseURL, verificationCode.String())
	imageURL := fmt.Sprintf("%s/media/badges/%s.png", uc.baseURL, verificationCode.String())

	badge := &domain.Badge{
		ID:               uuid.New(),
		StudentID:        studentID,
		CourseID:         courseID,
		CourseVersionID:  versionID,
		VerificationCode: verificationCode,
		ImageURL:         imageURL,
		VerificationURL:  verificationURL,
		IsRevoked:        false,
		IssuedAt:         time.Now(),
	}

	if err := uc.repo.CreateBadge(ctx, badge); err != nil {
		return nil, err
	}

	return badge, nil
}

func (uc *UseCase) VerifyBadge(ctx context.Context, verificationCode uuid.UUID) (*domain.Badge, error) {
	return uc.repo.GetBadgeByCode(ctx, verificationCode)
}
