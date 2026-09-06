package course

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mooc-platform/backend/internal/domain"
)

type mockCourseRepo struct {
	courses  map[uuid.UUID]*domain.Course
	versions map[uuid.UUID]*domain.CourseVersion
	modules  map[uuid.UUID]*domain.Module
	units    map[uuid.UUID]*domain.Unit
	resources map[uuid.UUID]*domain.Resource
}

func newMockCourseRepo() *mockCourseRepo {
	return &mockCourseRepo{
		courses:   make(map[uuid.UUID]*domain.Course),
		versions:  make(map[uuid.UUID]*domain.CourseVersion),
		modules:   make(map[uuid.UUID]*domain.Module),
		units:     make(map[uuid.UUID]*domain.Unit),
		resources: make(map[uuid.UUID]*domain.Resource),
	}
}

func (m *mockCourseRepo) CreateCourse(ctx context.Context, c *domain.Course, v *domain.CourseVersion) error {
	m.courses[c.ID] = c
	m.versions[v.ID] = v
	return nil
}

func (m *mockCourseRepo) GetCourseByID(ctx context.Context, id uuid.UUID) (*domain.Course, error) {
	c, ok := m.courses[id]
	if !ok {
		return nil, domain.ErrCourseNotFound
	}
	return c, nil
}

func (m *mockCourseRepo) GetCourseBySlug(ctx context.Context, slug string) (*domain.Course, error) {
	for _, c := range m.courses {
		if c.Slug == slug {
			return c, nil
		}
	}
	return nil, domain.ErrCourseNotFound
}

func (m *mockCourseRepo) ListCourses(ctx context.Context, limit, offset int) ([]*domain.Course, error) {
	list := make([]*domain.Course, 0)
	for _, c := range m.courses {
		list = append(list, c)
	}
	return list, nil
}

func (m *mockCourseRepo) UpdateCourse(ctx context.Context, c *domain.Course) error {
	m.courses[c.ID] = c
	return nil
}

func (m *mockCourseRepo) CreateVersion(ctx context.Context, v *domain.CourseVersion) error {
	m.versions[v.ID] = v
	return nil
}

func (m *mockCourseRepo) GetVersionByID(ctx context.Context, versionID uuid.UUID) (*domain.CourseVersion, error) {
	v, ok := m.versions[versionID]
	if !ok {
		return nil, domain.ErrVersionNotFound
	}
	return v, nil
}

func (m *mockCourseRepo) GetFullVersionHierarchy(ctx context.Context, versionID uuid.UUID) (*domain.CourseVersion, error) {
	v, ok := m.versions[versionID]
	if !ok {
		return nil, domain.ErrVersionNotFound
	}

	mods := make([]domain.Module, 0)
	for _, mod := range m.modules {
		if mod.VersionID == versionID {
			unts := make([]domain.Unit, 0)
			for _, u := range m.units {
				if u.ModuleID == mod.ID {
					resList := make([]domain.Resource, 0)
					for _, r := range m.resources {
						if r.UnitID == u.ID {
							resList = append(resList, *r)
						}
					}
					u.Resources = resList
					unts = append(unts, *u)
				}
			}
			mod.Units = unts
			mods = append(mods, *mod)
		}
	}
	vCopy := *v
	vCopy.Modules = mods
	return &vCopy, nil
}

func (m *mockCourseRepo) PublishVersion(ctx context.Context, courseID uuid.UUID, versionID uuid.UUID) error {
	v, ok := m.versions[versionID]
	if !ok {
		return domain.ErrVersionNotFound
	}
	v.Status = domain.VersionStatusPublished
	now := time.Now()
	v.PublishedAt = &now

	c, ok := m.courses[courseID]
	if ok {
		c.CurrentPublishedVersionID = &versionID
	}
	return nil
}

func (m *mockCourseRepo) CreateModule(ctx context.Context, mod *domain.Module) error {
	m.modules[mod.ID] = mod
	return nil
}

func (m *mockCourseRepo) CreateUnit(ctx context.Context, u *domain.Unit) error {
	m.units[u.ID] = u
	return nil
}

func (m *mockCourseRepo) CreateResource(ctx context.Context, res *domain.Resource) error {
	m.resources[res.ID] = res
	return nil
}

func (m *mockCourseRepo) UpdateResource(ctx context.Context, res *domain.Resource) error {
	m.resources[res.ID] = res
	return nil
}

type mockUserRepo struct{}
func (u *mockUserRepo) CreateUser(ctx context.Context, user *domain.User) error { return nil }
func (u *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return &domain.User{ID: id, Role: domain.RoleTeacher, Status: domain.StatusActive}, nil
}
func (u *mockUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) { return nil, nil }
func (u *mockUserRepo) UpdateUser(ctx context.Context, user *domain.User) error { return nil }
func (u *mockUserRepo) CountActiveAdmins(ctx context.Context) (int, error) { return 1, nil }
func (u *mockUserRepo) CreateSession(ctx context.Context, session *domain.UserSession) error { return nil }
func (u *mockUserRepo) GetSessionByToken(ctx context.Context, token string) (*domain.UserSession, error) { return nil, nil }
func (u *mockUserRepo) RevokeSession(ctx context.Context, sessionID uuid.UUID) error { return nil }
func (u *mockUserRepo) RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error { return nil }
func (u *mockUserRepo) CreateToken(ctx context.Context, token *domain.UserToken) error { return nil }
func (u *mockUserRepo) GetToken(ctx context.Context, tokenStr string, tokenType string) (*domain.UserToken, error) { return nil, nil }
func (u *mockUserRepo) MarkTokenUsed(ctx context.Context, id uuid.UUID) error { return nil }
func (u *mockUserRepo) CreateAuditLog(ctx context.Context, log *domain.AuditLog) error { return nil }

func TestCoursePublicationValidation(t *testing.T) {
	cRepo := newMockCourseRepo()
	uRepo := &mockUserRepo{}
	uc := NewUseCase(cRepo, uRepo)
	ctx := context.Background()

	teacherID := uuid.New()
	_, version, err := uc.CreateCourse(ctx, teacherID, CreateCourseInput{
		Title:        "Curso de Go Avanzado",
		Summary:      "Aprende Go y Docker",
		PassingScore: 80.0,
	})
	if err != nil {
		t.Fatalf("Creación de curso falló: %v", err)
	}

	// 1. Intentar publicar sin estructura mínima (debe fallar)
	_, err = uc.PublishVersion(ctx, teacherID, version.ID)
	if err != domain.ErrInvalidPublishStructure {
		t.Errorf("Esperaba ErrInvalidPublishStructure al publicar sin jerarquía, obtenido: %v", err)
	}

	// 2. Agregar Módulo, Unidad y Recurso visible y disponible
	mod, _ := uc.AddModule(ctx, teacherID, version.ID, "Módulo 1: Introducción", "Conceptos básicos", 1)
	unit, _ := uc.AddUnit(ctx, teacherID, mod.ID, "Unidad 1.1: Hola Mundo", 1)
	_, _ = uc.AddResource(ctx, teacherID, &domain.Resource{
		UnitID:           unit.ID,
		Title:            "Primer Programa en Go",
		Type:             domain.ResourceTypeText,
		IsVisible:        true,
		IsMandatory:      true,
		ProcessingStatus: domain.ProcessingCompleted,
		Position:         1,
	})

	// 3. Publicar con estructura completa (debe ser exitoso)
	pubVersion, err := uc.PublishVersion(ctx, teacherID, version.ID)
	if err != nil {
		t.Fatalf("Publicación válida falló: %v", err)
	}
	if pubVersion.Status != domain.VersionStatusPublished {
		t.Errorf("Esperaba estado published, obtenido: %s", pubVersion.Status)
	}

	// 4. Intentar modificar una versión ya publicada (debe rehusar por inmutabilidad)
	_, err = uc.AddModule(ctx, teacherID, version.ID, "Módulo 2: Concurrencia", "Goroutines", 2)
	if err != domain.ErrVersionImmutable {
		t.Errorf("Esperaba ErrVersionImmutable al editar versión publicada, obtenido: %v", err)
	}
}
