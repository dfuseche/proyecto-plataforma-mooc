package learning

import (
	"context"
	"errors"
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

func (uc *UseCase) CreateQuiz(ctx context.Context, teacherID uuid.UUID, quiz *domain.Quiz) (*domain.Quiz, error) {
	if quiz.ID == uuid.Nil {
		quiz.ID = uuid.New()
	}
	if quiz.MaxAttempts <= 0 {
		quiz.MaxAttempts = 3
	}
	if quiz.PassingScore <= 0 {
		quiz.PassingScore = 70.0
	}

	if err := uc.repo.CreateQuiz(ctx, quiz); err != nil {
		return nil, err
	}

	return quiz, nil
}

type QuizSnapshotResponse struct {
	ID           uuid.UUID                 `json:"id"`
	ResourceID   uuid.UUID                 `json:"resource_id"`
	MaxAttempts  int                       `json:"max_attempts"`
	PassingScore float64                   `json:"passing_score"`
	Questions    []QuizSnapshotQuestionDTO `json:"questions"`
}

type QuizSnapshotQuestionDTO struct {
	ID           uuid.UUID                  `json:"id"`
	QuestionText string                     `json:"question_text"`
	Position     int                        `json:"position"`
	Points       float64                    `json:"points"`
	Options      []domain.StudentQuizOption `json:"options"`
}

func (uc *UseCase) GetQuizSnapshot(ctx context.Context, quizID uuid.UUID) (*QuizSnapshotResponse, error) {
	quiz, err := uc.repo.GetQuizWithAnswers(ctx, quizID)
	if err != nil {
		return nil, err
	}

	questionsDTO := make([]QuizSnapshotQuestionDTO, 0)
	for _, quest := range quiz.Questions {
		optsDTO := make([]domain.StudentQuizOption, 0)
		for _, opt := range quest.Options {
			optsDTO = append(optsDTO, domain.StudentQuizOption{
				ID:         opt.ID,
				QuestionID: opt.QuestionID,
				OptionText: opt.OptionText,
				Position:   opt.Position,
			})
		}
		questionsDTO = append(questionsDTO, QuizSnapshotQuestionDTO{
			ID:           quest.ID,
			QuestionText: quest.QuestionText,
			Position:     quest.Position,
			Points:       quest.Points,
			Options:      optsDTO,
		})
	}

	return &QuizSnapshotResponse{
		ID:           quiz.ID,
		ResourceID:   quiz.ResourceID,
		MaxAttempts:  quiz.MaxAttempts,
		PassingScore: quiz.PassingScore,
		Questions:    questionsDTO,
	}, nil
}

func (uc *UseCase) StartQuizAttempt(ctx context.Context, studentID, quizID uuid.UUID) (*domain.QuizAttempt, error) {
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

	attempt := &domain.QuizAttempt{
		ID:            uuid.New(),
		StudentID:     studentID,
		QuizID:        quizID,
		AttemptNumber: attemptsCount + 1,
		Status:        domain.AttemptInProgress,
		Answers:       make(map[string]string),
	}

	if err := uc.repo.CreateQuizAttempt(ctx, attempt); err != nil {
		return nil, err
	}

	return attempt, nil
}

func (uc *UseCase) SavePartialAttempt(ctx context.Context, studentID, attemptID uuid.UUID, answers map[string]string) (*domain.QuizAttempt, error) {
	attempt, err := uc.repo.GetAttemptByID(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	if attempt.StudentID != studentID {
		return nil, domain.ErrForbidden
	}

	if attempt.Status != domain.AttemptInProgress {
		return nil, domain.ErrAttemptAlreadySubmitted
	}

	for k, v := range answers {
		attempt.Answers[k] = v
	}

	if err := uc.repo.UpdateQuizAttempt(ctx, attempt); err != nil {
		return nil, err
	}

	return attempt, nil
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
	if input.DwellTimeSeconds < 0 || input.LastPositionSeconds < 0 {
		return nil, errors.New("los valores de tiempo de permanencia y posición deben ser no negativos")
	}

	enrollment, err := uc.repo.GetEnrollment(ctx, input.StudentID, input.CourseID)
	if err != nil {
		return nil, domain.ErrNotEnrolled
	}

	// Limitar permanencia reportada por pulso individual a máximo 300 segundos por seguridad anti-manipulación
	dwellTime := input.DwellTimeSeconds
	if dwellTime > 300 {
		dwellTime = 300
	}

	status := domain.ProgressOpened
	if dwellTime >= 10 {
		status = domain.ProgressCompleted
	}

	prog := &domain.ResourceProgress{
		ID:                  uuid.New(),
		StudentID:           input.StudentID,
		CourseID:            input.CourseID,
		ResourceStableID:    input.ResourceStableID,
		Status:              status,
		DwellTimeSeconds:    dwellTime,
		LastPositionSeconds: input.LastPositionSeconds,
	}

	if err := uc.repo.UpsertResourceProgress(ctx, prog); err != nil {
		return nil, err
	}

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
		return existing, nil
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

func (uc *UseCase) RevokeBadge(ctx context.Context, adminID, badgeID uuid.UUID) error {
	badge, err := uc.repo.GetBadgeByID(ctx, badgeID)
	if err != nil {
		return err
	}
	if badge.IsRevoked {
		return nil
	}
	if err := uc.repo.RevokeBadge(ctx, badgeID); err != nil {
		return err
	}

	_ = uc.userRepo.CreateAuditLog(ctx, &domain.AuditLog{
		ActorID:        &adminID,
		Action:         "BADGE_REVOKED",
		TargetResource: "badges",
		TargetID:       &badgeID,
		Payload:        map[string]any{"student_id": badge.StudentID, "course_id": badge.CourseID},
	})
	return nil
}

func (uc *UseCase) VerifyBadge(ctx context.Context, verificationCode uuid.UUID) (*domain.Badge, error) {
	return uc.repo.GetBadgeByCode(ctx, verificationCode)
}
