package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Role string
type UserStatus string

const (
	RoleAdmin   Role = "admin"
	RoleTeacher Role = "teacher"
	RoleStudent Role = "student"
)

const (
	StatusUnverified UserStatus = "unverified"
	StatusActive     UserStatus = "active"
	StatusSuspended  UserStatus = "suspended"
)

type User struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	PasswordHash    string     `json:"-"`
	FullName        string     `json:"full_name"`
	Role            Role       `json:"role"`
	Status          UserStatus `json:"status"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type UserSession struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Token     string    `json:"token"`
	UserAgent string    `json:"user_agent"`
	IPAddress string    `json:"ip_address"`
	IsRevoked bool      `json:"is_revoked"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type UserToken struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Token     string    `json:"token"`
	Type      string    `json:"type"` // email_verification, password_reset
	Used      bool      `json:"used"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLog struct {
	ID             uuid.UUID         `json:"id"`
	ActorID        *uuid.UUID        `json:"actor_id,omitempty"`
	Action         string            `json:"action"`
	TargetResource string            `json:"target_resource"`
	TargetID       *uuid.UUID        `json:"target_id,omitempty"`
	Payload        map[string]any    `json:"payload,omitempty"`
	IPAddress      string            `json:"ip_address,omitempty"`
	UserAgent      string            `json:"user_agent,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

type UserRepository interface {
	CreateUser(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	CountActiveAdmins(ctx context.Context) (int, error)
	
	CreateSession(ctx context.Context, session *UserSession) error
	GetSessionByToken(ctx context.Context, token string) (*UserSession, error)
	RevokeSession(ctx context.Context, sessionID uuid.UUID) error
	RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) error
	
	CreateToken(ctx context.Context, token *UserToken) error
	GetToken(ctx context.Context, tokenStr string, tokenType string) (*UserToken, error)
	MarkTokenUsed(ctx context.Context, id uuid.UUID) error
	
	CreateAuditLog(ctx context.Context, log *AuditLog) error
}
