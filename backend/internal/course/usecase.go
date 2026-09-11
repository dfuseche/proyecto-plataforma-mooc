package course

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/mooc-platform/backend/internal/domain"
)

type UseCase struct {
	repo     domain.CourseRepository
	userRepo domain.UserRepository
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

func (uc *UseCase) CreateDraftFromPublished(ctx context.Context, teacherID uuid.UUID, courseID uuid.UUID) (*domain.CourseVersion, error) {
	course, err := uc.repo.GetCourseByID(ctx, courseID)
	if err != nil {
		return nil, err
	}

	if course.CurrentPublishedVersionID == nil {
		return nil, domain.ErrVersionNotFound
	}

	pubVersion, err := uc.repo.GetFullVersionHierarchy(ctx, *course.CurrentPublishedVersionID)
	if err != nil {
		return nil, err
	}

	newVersion := &domain.CourseVersion{
		ID:            uuid.New(),
		CourseID:      courseID,
		VersionNumber: pubVersion.VersionNumber + 1,
		Status:        domain.VersionStatusDraft,
		PassingScore:  pubVersion.PassingScore,
	}

	if err := uc.repo.CreateVersion(ctx, newVersion); err != nil {
		return nil, err
	}

	// Copiar jerarquía profunda preservando stable_ids
	for _, mod := range pubVersion.Modules {
		newMod := &domain.Module{
			ID:          uuid.New(),
			VersionID:   newVersion.ID,
			StableID:    mod.StableID, // Preservado
			Title:       mod.Title,
			Description: mod.Description,
			Position:    mod.Position,
		}
		_ = uc.repo.CreateModule(ctx, newMod)

		for _, u := range mod.Units {
			newUnit := &domain.Unit{
				ID:       uuid.New(),
				ModuleID: newMod.ID,
				StableID: u.StableID, // Preservado
				Title:    u.Title,
				Position: u.Position,
			}
			_ = uc.repo.CreateUnit(ctx, newUnit)

			for _, r := range u.Resources {
				newRes := &domain.Resource{
					ID:                uuid.New(),
					UnitID:            newUnit.ID,
					StableID:          r.StableID, // Preservado
					Title:             r.Title,
					Type:              r.Type,
					CanonicalMarkdown: r.CanonicalMarkdown,
					MediaURL:          r.MediaURL,
					IsVisible:         r.IsVisible,
					IsMandatory:       r.IsMandatory,
					IsDownloadable:    r.IsDownloadable,
					Position:          r.Position,
					ProcessingStatus:  r.ProcessingStatus,
				}
				_ = uc.repo.CreateResource(ctx, newRes)
			}
		}
	}

	return uc.repo.GetFullVersionHierarchy(ctx, newVersion.ID)
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

func (uc *UseCase) UpdateResource(ctx context.Context, teacherID uuid.UUID, resource *domain.Resource) (*domain.Resource, error) {
	existing, err := uc.repo.GetResourceByID(ctx, resource.ID)
	if err != nil {
		return nil, err
	}

	if resource.Title == "" {
		resource.Title = existing.Title
	}
	if resource.Type == "" {
		resource.Type = existing.Type
	}
	if resource.CanonicalMarkdown == "" {
		resource.CanonicalMarkdown = existing.CanonicalMarkdown
	}
	if resource.MediaURL == "" {
		resource.MediaURL = existing.MediaURL
	}
	if resource.Position == 0 {
		resource.Position = existing.Position
	}
	if resource.ProcessingStatus == "" {
		resource.ProcessingStatus = existing.ProcessingStatus
	}
	resource.UnitID = existing.UnitID
	resource.StableID = existing.StableID
	resource.CreatedAt = existing.CreatedAt

	if err := uc.repo.UpdateResource(ctx, resource); err != nil {
		return nil, err
	}
	return uc.repo.GetResourceByID(ctx, resource.ID)
}

func (uc *UseCase) DeleteResource(ctx context.Context, teacherID uuid.UUID, resourceID uuid.UUID) error {
	return uc.repo.DeleteResource(ctx, resourceID)
}

func (uc *UseCase) PublishVersion(ctx context.Context, teacherID uuid.UUID, versionID uuid.UUID) (*domain.CourseVersion, error) {
	versionHierarchy, err := uc.repo.GetFullVersionHierarchy(ctx, versionID)
	if err != nil {
		return nil, err
	}

	if versionHierarchy.Status != domain.VersionStatusDraft {
		return nil, domain.ErrVersionImmutable
	}

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

func (uc *UseCase) UnpublishCourse(ctx context.Context, teacherID uuid.UUID, courseID uuid.UUID) error {
	return uc.repo.UnpublishCourse(ctx, courseID)
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

func (uc *UseCase) GetCourseDraftPreview(ctx context.Context, teacherID uuid.UUID, courseID uuid.UUID) (*domain.CourseVersion, error) {
	draft, err := uc.repo.GetLatestDraftVersion(ctx, courseID)
	if err != nil {
		return nil, err
	}
	return uc.repo.GetFullVersionHierarchy(ctx, draft.ID)
}

func (uc *UseCase) ReorderModules(ctx context.Context, teacherID uuid.UUID, versionID uuid.UUID, orderedIDs []uuid.UUID) error {
	return uc.repo.ReorderModules(ctx, versionID, orderedIDs)
}

func (uc *UseCase) ReorderUnits(ctx context.Context, teacherID uuid.UUID, moduleID uuid.UUID, orderedIDs []uuid.UUID) error {
	return uc.repo.ReorderUnits(ctx, moduleID, orderedIDs)
}

func (uc *UseCase) ReorderResources(ctx context.Context, teacherID uuid.UUID, unitID uuid.UUID, orderedIDs []uuid.UUID) error {
	return uc.repo.ReorderResources(ctx, unitID, orderedIDs)
}
