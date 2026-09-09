package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/mooc-platform/backend/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type UseCase struct {
	repo domain.UserRepository
}

func NewUseCase(repo domain.UserRepository) *UseCase {
	return &UseCase{repo: repo}
}

// SeedDefaultAdmin crea el administrador principal por defecto si la BD está vacía
func (uc *UseCase) SeedDefaultAdmin(ctx context.Context) error {
	count, err := uc.repo.CountActiveAdmins(ctx)
	if err == nil && count > 0 {
		return nil
	}

	adminEmail := "admin@mooc.com"
	adminPass := "Admin123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	now := time.Now()
	admin := &domain.User{
		ID:              uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Email:           adminEmail,
		PasswordHash:    string(hash),
		FullName:        "Administrador Principal MOOC",
		Role:            domain.RoleAdmin,
		Status:          domain.StatusActive,
		EmailVerifiedAt: &now,
	}

	if err := uc.repo.CreateUser(ctx, admin); err != nil {
		return err
	}

	log.Printf("[SEED-ADMIN] Administrador inicial creado exitosamente (Email: %s, Pass: %s)", adminEmail, adminPass)
	return nil
}

type RegisterStudentInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

func (uc *UseCase) RegisterStudent(ctx context.Context, input RegisterStudentInput) (*domain.User, string, error) {
	existing, err := uc.repo.GetByEmail(ctx, input.Email)
	if err == nil && existing != nil {
		return nil, "", domain.ErrUserAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        input.Email,
		PasswordHash: string(hash),
		FullName:     input.FullName,
		Role:         domain.RoleStudent,
		Status:       domain.StatusUnverified,
	}

	if err := uc.repo.CreateUser(ctx, user); err != nil {
		return nil, "", err
	}

	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	tokenStr := hex.EncodeToString(tokenBytes)

	verificationToken := &domain.UserToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Token:     tokenStr,
		Type:      "email_verification",
		Used:      false,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := uc.repo.CreateToken(ctx, verificationToken); err != nil {
		return nil, "", err
	}

	_ = uc.repo.CreateAuditLog(ctx, &domain.AuditLog{
		Action:         "USER_REGISTERED",
		TargetResource: "users",
		TargetID:       &user.ID,
		Payload:        map[string]any{"email": user.Email, "role": user.Role},
	})

	return user, tokenStr, nil
}

func (uc *UseCase) VerifyEmail(ctx context.Context, tokenStr string) error {
	token, err := uc.repo.GetToken(ctx, tokenStr, "email_verification")
	if err != nil {
		return err
	}
	if token.Used || time.Now().After(token.ExpiresAt) {
		return domain.ErrInvalidToken
	}

	user, err := uc.repo.GetByID(ctx, token.UserID)
	if err != nil {
		return err
	}

	now := time.Now()
	user.Status = domain.StatusActive
	user.EmailVerifiedAt = &now

	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return err
	}

	if err := uc.repo.MarkTokenUsed(ctx, token.ID); err != nil {
		return err
	}

	_ = uc.repo.CreateAuditLog(ctx, &domain.AuditLog{
		Action:         "USER_EMAIL_VERIFIED",
		TargetResource: "users",
		TargetID:       &user.ID,
	})

	return nil
}

type LoginInput struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	UserAgent string `json:"user_agent"`
	IPAddress string `json:"ip_address"`
}

type LoginOutput struct {
	User    *domain.User        `json:"user"`
	Session *domain.UserSession `json:"session"`
}

func (uc *UseCase) Login(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	user, err := uc.repo.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if user.Status == domain.StatusSuspended {
		return nil, domain.ErrUserSuspended
	}

	if user.Status == domain.StatusUnverified {
		return nil, domain.ErrEmailNotVerified
	}

	sessionTokenBytes := make([]byte, 32)
	rand.Read(sessionTokenBytes)
	sessionToken := hex.EncodeToString(sessionTokenBytes)

	session := &domain.UserSession{
		ID:        uuid.New(),
		UserID:    user.ID,
		Token:     sessionToken,
		UserAgent: input.UserAgent,
		IPAddress: input.IPAddress,
		IsRevoked: false,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	}

	if err := uc.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	_ = uc.repo.CreateAuditLog(ctx, &domain.AuditLog{
		ActorID:        &user.ID,
		Action:         "USER_LOGGED_IN",
		TargetResource: "user_sessions",
		TargetID:       &session.ID,
		IPAddress:      input.IPAddress,
		UserAgent:      input.UserAgent,
	})

	return &LoginOutput{
		User:    user,
		Session: session,
	}, nil
}

func (uc *UseCase) Logout(ctx context.Context, sessionToken string) error {
	session, err := uc.repo.GetSessionByToken(ctx, sessionToken)
	if err != nil {
		return nil
	}
	if err := uc.repo.RevokeSession(ctx, session.ID); err != nil {
		return err
	}
	_ = uc.repo.CreateAuditLog(ctx, &domain.AuditLog{
		ActorID:        &session.UserID,
		Action:         "USER_LOGGED_OUT",
		TargetResource: "user_sessions",
		TargetID:       &session.ID,
	})
	return nil
}

func (uc *UseCase) ForgotPassword(ctx context.Context, email string) (string, error) {
	user, err := uc.repo.GetByEmail(ctx, email)
	if err != nil {
		// Retornar genérico sin revelar existencia de correo por seguridad
		return "", nil
	}

	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	tokenStr := hex.EncodeToString(tokenBytes)

	resetToken := &domain.UserToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Token:     tokenStr,
		Type:      "password_reset",
		Used:      false,
		ExpiresAt: time.Now().Add(2 * time.Hour),
	}

	if err := uc.repo.CreateToken(ctx, resetToken); err != nil {
		return "", err
	}

	_ = uc.repo.CreateAuditLog(ctx, &domain.AuditLog{
		ActorID:        &user.ID,
		Action:         "PASSWORD_RESET_REQUESTED",
		TargetResource: "users",
		TargetID:       &user.ID,
	})

	return tokenStr, nil
}

func (uc *UseCase) ResetPassword(ctx context.Context, tokenStr string, newPassword string) error {
	token, err := uc.repo.GetToken(ctx, tokenStr, "password_reset")
	if err != nil {
		return domain.ErrInvalidToken
	}
	if token.Used || time.Now().After(token.ExpiresAt) {
		return domain.ErrInvalidToken
	}

	user, err := uc.repo.GetByID(ctx, token.UserID)
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hash)
	if err := uc.repo.UpdateUser(ctx, user); err != nil {
		return err
	}

	_ = uc.repo.MarkTokenUsed(ctx, token.ID)
	_ = uc.repo.RevokeAllUserSessions(ctx, user.ID)

	_ = uc.repo.CreateAuditLog(ctx, &domain.AuditLog{
		ActorID:        &user.ID,
		Action:         "PASSWORD_RESET_COMPLETED",
		TargetResource: "users",
		TargetID:       &user.ID,
	})

	return nil
}

type CreateTeacherInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

func (uc *UseCase) CreateTeacher(ctx context.Context, adminID uuid.UUID, input CreateTeacherInput) (*domain.User, error) {
	admin, err := uc.repo.GetByID(ctx, adminID)
	if err != nil || admin.Role != domain.RoleAdmin {
		return nil, domain.ErrForbidden
	}

	existing, err := uc.repo.GetByEmail(ctx, input.Email)
	if err == nil && existing != nil {
		return nil, domain.ErrUserAlreadyExists
	}

	password := input.Password
	if password == "" {
		password = "TeacherPass123!"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	teacher := &domain.User{
		ID:              uuid.New(),
		Email:           input.Email,
		PasswordHash:    string(hash),
		FullName:        input.FullName,
		Role:            domain.RoleTeacher,
		Status:          domain.StatusActive,
		EmailVerifiedAt: &now,
	}

	if err := uc.repo.CreateUser(ctx, teacher); err != nil {
		return nil, err
	}

	_ = uc.repo.CreateAuditLog(ctx, &domain.AuditLog{
		ActorID:        &adminID,
		Action:         "TEACHER_CREATED",
		TargetResource: "users",
		TargetID:       &teacher.ID,
		Payload:        map[string]any{"email": teacher.Email, "role": teacher.Role},
	})

	return teacher, nil
}

func (uc *UseCase) ChangeUserStatus(ctx context.Context, adminID uuid.UUID, targetUserID uuid.UUID, newStatus domain.UserStatus) error {
	admin, err := uc.repo.GetByID(ctx, adminID)
	if err != nil || admin.Role != domain.RoleAdmin {
		return domain.ErrForbidden
	}

	target, err := uc.repo.GetByID(ctx, targetUserID)
	if err != nil {
		return err
	}

	if target.Role == domain.RoleAdmin && (newStatus == domain.StatusSuspended || newStatus == domain.StatusUnverified) {
		activeAdmins, err := uc.repo.CountActiveAdmins(ctx)
		if err != nil {
			return err
		}
		if activeAdmins <= 1 {
			return domain.ErrLastAdminProtection
		}
	}

	target.Status = newStatus
	if err := uc.repo.UpdateUser(ctx, target); err != nil {
		return err
	}

	if newStatus == domain.StatusSuspended {
		_ = uc.repo.RevokeAllUserSessions(ctx, target.ID)
	}

	_ = uc.repo.CreateAuditLog(ctx, &domain.AuditLog{
		ActorID:        &adminID,
		Action:         "USER_STATUS_CHANGED",
		TargetResource: "users",
		TargetID:       &target.ID,
		Payload:        map[string]any{"new_status": newStatus},
	})

	return nil
}

func (uc *UseCase) ListUsers(ctx context.Context, role *domain.Role, status *domain.UserStatus, search string, limit, offset int) ([]*domain.User, int, error) {
	return uc.repo.ListUsers(ctx, role, status, search, limit, offset)
}

func (uc *UseCase) ListAuditLogs(ctx context.Context, limit, offset int) ([]*domain.AuditLog, int, error) {
	return uc.repo.ListAuditLogs(ctx, limit, offset)
}
