package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mooc-platform/backend/internal/domain"
	"github.com/mooc-platform/backend/internal/learning"
	"github.com/mooc-platform/backend/internal/user"
)

type mockE2ERepo struct {
	users       map[string]*domain.User
	usersByID   map[uuid.UUID]*domain.User
	tokens      map[string]*domain.UserToken
	courses     map[uuid.UUID]*domain.Course
	versions    map[uuid.UUID]*domain.CourseVersion
	enrollments map[string]*domain.Enrollment
	badges      map[string]*domain.Badge
}

func newMockE2ERepo() *mockE2ERepo {
	return &mockE2ERepo{
		users:       make(map[string]*domain.User),
		usersByID:   make(map[uuid.UUID]*domain.User),
		tokens:      make(map[string]*domain.UserToken),
		courses:     make(map[uuid.UUID]*domain.Course),
		versions:    make(map[uuid.UUID]*domain.CourseVersion),
		enrollments: make(map[string]*domain.Enrollment),
		badges:      make(map[string]*domain.Badge),
	}
}

func (m *mockE2ERepo) CreateUser(ctx context.Context, u *domain.User) error {
	m.users[u.Email] = u
	m.usersByID[u.ID] = u
	return nil
}
func (m *mockE2ERepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := m.usersByID[id]
	if !ok { return nil, domain.ErrUserNotFound }
	return u, nil
}
func (m *mockE2ERepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	u, ok := m.users[email]
	if !ok { return nil, domain.ErrUserNotFound }
	return u, nil
}
func (m *mockE2ERepo) UpdateUser(ctx context.Context, u *domain.User) error {
	m.users[u.Email] = u
	m.usersByID[u.ID] = u
	return nil
}
func (m *mockE2ERepo) CountActiveAdmins(ctx context.Context) (int, error) { return 1, nil }
func (m *mockE2ERepo) CreateSession(ctx context.Context, s *domain.UserSession) error { return nil }
func (m *mockE2ERepo) GetSessionByToken(ctx context.Context, t string) (*domain.UserSession, error) { return nil, nil }
func (m *mockE2ERepo) RevokeSession(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockE2ERepo) RevokeAllUserSessions(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockE2ERepo) CreateToken(ctx context.Context, t *domain.UserToken) error {
	m.tokens[t.Token] = t
	return nil
}
func (m *mockE2ERepo) GetToken(ctx context.Context, str, typ string) (*domain.UserToken, error) {
	t, ok := m.tokens[str]
	if !ok || t.Type != typ { return nil, domain.ErrInvalidToken }
	return t, nil
}
func (m *mockE2ERepo) MarkTokenUsed(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockE2ERepo) CreateAuditLog(ctx context.Context, l *domain.AuditLog) error { return nil }

func (m *mockE2ERepo) CreateCourse(ctx context.Context, c *domain.Course, v *domain.CourseVersion) error {
	m.courses[c.ID] = c
	m.versions[v.ID] = v
	return nil
}
func (m *mockE2ERepo) GetCourseByID(ctx context.Context, id uuid.UUID) (*domain.Course, error) { return m.courses[id], nil }
func (m *mockE2ERepo) GetCourseBySlug(ctx context.Context, slug string) (*domain.Course, error) { return nil, nil }
func (m *mockE2ERepo) ListCourses(ctx context.Context, limit, offset int) ([]*domain.Course, error) { return nil, nil }
func (m *mockE2ERepo) UpdateCourse(ctx context.Context, c *domain.Course) error { return nil }
func (m *mockE2ERepo) CreateVersion(ctx context.Context, v *domain.CourseVersion) error { return nil }
func (m *mockE2ERepo) GetVersionByID(ctx context.Context, id uuid.UUID) (*domain.CourseVersion, error) { return m.versions[id], nil }
func (m *mockE2ERepo) GetFullVersionHierarchy(ctx context.Context, id uuid.UUID) (*domain.CourseVersion, error) { return m.versions[id], nil }
func (m *mockE2ERepo) PublishVersion(ctx context.Context, cid, vid uuid.UUID) error {
	v := m.versions[vid]
	v.Status = domain.VersionStatusPublished
	now := time.Now()
	v.PublishedAt = &now
	c := m.courses[cid]
	c.CurrentPublishedVersionID = &vid
	return nil
}
func (m *mockE2ERepo) CreateModule(ctx context.Context, mod *domain.Module) error { return nil }
func (m *mockE2ERepo) CreateUnit(ctx context.Context, u *domain.Unit) error { return nil }
func (m *mockE2ERepo) CreateResource(ctx context.Context, res *domain.Resource) error { return nil }
func (m *mockE2ERepo) UpdateResource(ctx context.Context, res *domain.Resource) error { return nil }

func (m *mockE2ERepo) EnrollStudent(ctx context.Context, e *domain.Enrollment) error {
	key := e.StudentID.String() + ":" + e.CourseID.String()
	m.enrollments[key] = e
	return nil
}
func (m *mockE2ERepo) GetEnrollment(ctx context.Context, sid, cid uuid.UUID) (*domain.Enrollment, error) {
	key := sid.String() + ":" + cid.String()
	e, ok := m.enrollments[key]
	if !ok { return nil, domain.ErrNotEnrolled }
	return e, nil
}
func (m *mockE2ERepo) UpdateEnrollment(ctx context.Context, e *domain.Enrollment) error {
	key := e.StudentID.String() + ":" + e.CourseID.String()
	m.enrollments[key] = e
	return nil
}
func (m *mockE2ERepo) CreateQuiz(ctx context.Context, q *domain.Quiz) error { return nil }
func (m *mockE2ERepo) GetQuizByResourceID(ctx context.Context, rid uuid.UUID) (*domain.Quiz, error) { return nil, nil }
func (m *mockE2ERepo) GetQuizWithAnswers(ctx context.Context, qid uuid.UUID) (*domain.Quiz, error) { return nil, nil }
func (m *mockE2ERepo) CreateQuizAttempt(ctx context.Context, a *domain.QuizAttempt) error { return nil }
func (m *mockE2ERepo) GetAttemptByID(ctx context.Context, aid uuid.UUID) (*domain.QuizAttempt, error) { return nil, nil }
func (m *mockE2ERepo) GetStudentAttemptsCount(ctx context.Context, sid, qid uuid.UUID) (int, error) { return 0, nil }
func (m *mockE2ERepo) UpdateQuizAttempt(ctx context.Context, a *domain.QuizAttempt) error { return nil }
func (m *mockE2ERepo) UpsertResourceProgress(ctx context.Context, p *domain.ResourceProgress) error { return nil }
func (m *mockE2ERepo) GetStudentCourseProgress(ctx context.Context, sid, cid uuid.UUID) ([]*domain.ResourceProgress, error) { return nil, nil }
func (m *mockE2ERepo) CreateBadge(ctx context.Context, b *domain.Badge) error {
	key := b.StudentID.String() + ":" + b.CourseID.String()
	m.badges[key] = b
	return nil
}
func (m *mockE2ERepo) GetBadgeByCode(ctx context.Context, code uuid.UUID) (*domain.Badge, error) { return nil, nil }
func (m *mockE2ERepo) GetStudentBadgeForCourse(ctx context.Context, sid, cid uuid.UUID) (*domain.Badge, error) {
	key := sid.String() + ":" + cid.String()
	b, ok := m.badges[key]
	if !ok { return nil, domain.ErrCourseNotFound }
	return b, nil
}

func TestE2ECriticalFlowsSuite(t *testing.T) {
	repo := newMockE2ERepo()
	userUC := user.NewUseCase(repo)
	learningUC := learning.NewUseCase(repo, repo, repo, "http://localhost:8080")
	ctx := context.Background()

	// Flujo 1: Registro de Estudiante
	student, token, err := userUC.RegisterStudent(ctx, user.RegisterStudentInput{
		Email:    "e2e.estudiante@mooc.com",
		Password: "Password123!",
		FullName: "Estudiante E2E",
	})
	if err != nil || token == "" {
		t.Fatalf("Flujo 1 Falló: Registro de estudiante: %v", err)
	}

	// Flujo 2: Verificación de Correo
	if err := userUC.VerifyEmail(ctx, token); err != nil {
		t.Fatalf("Flujo 2 Falló: Verificación de correo: %v", err)
	}

	// Flujo 3: Inscripción de Estudiante
	courseID := uuid.New()
	versionID := uuid.New()
	course := &domain.Course{ID: courseID, CurrentPublishedVersionID: &versionID}
	version := &domain.CourseVersion{ID: versionID, CourseID: courseID, Status: domain.VersionStatusPublished}
	repo.CreateCourse(ctx, course, version)

	enrollment, err := learningUC.EnrollStudent(ctx, student.ID, courseID)
	if err != nil || enrollment.Status != domain.EnrollmentActive {
		t.Fatalf("Flujo 3 Falló: Inscripción de estudiante: %v", err)
	}

	// Flujo 4: Emisión de Insignia Verificable
	badge, err := learningUC.IssueBadge(ctx, student.ID, courseID, versionID)
	if err != nil || badge.VerificationURL == "" {
		t.Fatalf("Flujo 4 Falló: Emisión de insignia verificable: %v", err)
	}

	t.Log("=== SUITE DE PRUEBAS DE FLUJOS CRÍTICOS E2E COMPLETADA CON ÉXITO ===")
}
