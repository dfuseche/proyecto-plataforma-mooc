package course

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mooc-platform/backend/internal/domain"
)

type UseCase struct {
	repo       domain.CourseRepository
	userRepo   domain.UserRepository
}

func NewUseCase(repo domain.CourseRepository, userRepo domain.UserRepository) *UseCase {
	return &UseCase{repo: repo, userRepo: userRepo}
}

type CreateCourseInput struct {
	Title        string  `json:"title"`
	Summary      string  `json:"summary"`
	PassingScore float64 `json:"passing_score"`
}

func (uc *UseCase) CreateCourse(ctx context.Context, teacherID uuid.UUID, input CreateCourseInput) (*domain.Course, *domain.CourseVersion, error) {
	teacher, err := uc.userRepo.GetByID(ctx, teacherID)
	if err != nil || (teacher.Role != domain.RoleTeacher && teacher.Role != domain.RoleAdmin) {
		return nil, nil, domain.ErrUnauthorizedCourseMutation
	}

	slug := strings.ToLower(strings.ReplaceAll(input.Title, " ", "-"))

	course := &domain.Course{
		ID:                 uuid.New(),
		Slug:               slug,
		Title:              input.Title,
		Summary:            input.Summary,
		CreatedByTeacherID: teacherID,
	}

	passingScore := input.PassingScore
	if passingScore <= 0 {
		passingScore = 70.0
	}

	version := &domain.CourseVersion{
		ID:            uuid.New(),
		CourseID:      course.ID,
		VersionNumber: 1,
		Status:        domain.VersionStatusDraft,
		PassingScore:  passingScore,
	}

	if err := uc.repo.CreateCourse(ctx, course, version); err != nil {
		return nil, nil, err
	}

	return course, version, nil
}

func (uc *UseCase) AddModule(ctx context.Context, teacherID uuid.UUID, versionID uuid.UUID, title string, description string, position int) (*domain.Module, error) {
	version, err := uc.repo.GetVersionByID(ctx, versionID)
	if err != nil {
		return nil, err
	}

	if version.Status != domain.VersionStatusDraft {
		return nil, domain.ErrVersionImmutable
	}

	module := &domain.Module{
		ID:          uuid.New(),
		VersionID:   versionID,
		StableID:    uuid.New(),
		Title:       title,
		Description: description,
		Position:    position,
	}

	if err := uc.repo.CreateModule(ctx, module); err != nil {
		return nil, err
	}

	return module, nil
}

func (uc *UseCase) AddUnit(ctx context.Context, teacherID uuid.UUID, moduleID uuid.UUID, title string, position int) (*domain.Unit, error) {
	unit := &domain.Unit{
		ID:        uuid.New(),
		ModuleID:  moduleID,
		StableID:  uuid.New(),
		Title:     title,
		Position:  position,
	}

	if err := uc.repo.CreateUnit(ctx, unit); err != nil {
		return nil, err
	}

	return unit, nil
}

func (uc *UseCase) AddResource(ctx context.Context, teacherID uuid.UUID, resource *domain.Resource) (*domain.Resource, error) {
	if resource.ID == uuid.Nil {
		resource.ID = uuid.New()
	}
	if resource.StableID == uuid.Nil {
		resource.StableID = uuid.New()
	}

	if err := uc.repo.CreateResource(ctx, resource); err != nil {
		return nil, err
	}

	return resource, nil
}

func (uc *UseCase) PublishVersion(ctx context.Context, teacherID uuid.UUID, versionID uuid.UUID) (*domain.CourseVersion, error) {
	versionHierarchy, err := uc.repo.GetFullVersionHierarchy(ctx, versionID)
	if err != nil {
		return nil, err
	}

	if versionHierarchy.Status != domain.VersionStatusDraft {
		return nil, domain.ErrVersionImmutable
	}

	// Validación estricta de estructura mínima publicable (Módulo -> Unidad -> Recurso visible y disponible)
	hasValidStructure := false
	if len(versionHierarchy.Modules) > 0 {
		for _, mod := range versionHierarchy.Modules {
			if len(mod.Units) > 0 {
				for _, u := range mod.Units {
					if len(u.Resources) > 0 {
						for _, r := range u.Resources {
							if r.IsVisible && r.ProcessingStatus == domain.ProcessingCompleted {
								hasValidStructure = true
								break
							}
						}
					}
					if hasValidStructure {
						break
					}
				}
			}
			if hasValidStructure {
				break
			}
		}
	}

	if !hasValidStructure {
		return nil, domain.ErrInvalidPublishStructure
	}

	if err := uc.repo.PublishVersion(ctx, versionHierarchy.CourseID, versionID); err != nil {
		return nil, err
	}

	versionHierarchy.Status = domain.VersionStatusPublished
	return versionHierarchy, nil
}

func (uc *UseCase) GetCourseCatalog(ctx context.Context, limit, offset int) ([]*domain.Course, error) {
	return uc.repo.ListCourses(ctx, limit, offset)
}

func (uc *UseCase) GetCourseHierarchy(ctx context.Context, courseID uuid.UUID) (*domain.CourseVersion, error) {
	course, err := uc.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return nil, err
	}

	if course.CurrentPublishedVersionID == nil {
		return nil, domain.ErrVersionNotFound
	}

	return uc.repo.GetFullVersionHierarchy(ctx, *course.CurrentPublishedVersionID)
}
