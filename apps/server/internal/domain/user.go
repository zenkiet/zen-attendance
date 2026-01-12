package domain

import (
	"context"
	"time"
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	FullName     string
	Role         string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLoginAt  *time.Time
}

const (
	RoleEmployee   = "employee"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "superadmin"
)

const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusDeleted   = "deleted"
)

type UserRepository interface {
	Create(ctx context.Context, user *User) error

	FindByEmail(ctx context.Context, email string) (*User, error)

	FindByID(ctx context.Context, id string) (*User, error)

	Update(ctx context.Context, user *User) error

	UpdateLastLogin(ctx context.Context, userID string) error

	Delete(ctx context.Context, userID string) error
}

func (u *User) IsActive() bool {
	return u.Status == StatusActive
}

func (u *User) IsEmployee() bool {
	return u.Role == RoleEmployee
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin || u.Role == RoleSuperAdmin
}

func (u *User) IsSuperAdmin() bool {
	return u.Role == RoleSuperAdmin
}
