package domain

import (
	"context"
	"time"
)

type Session struct {
	ID                string
	UserID            string
	DeviceFingerprint string
	IPAddress         string
	UserAgent         string
	CreatedAt         time.Time
	ExpiresAt         time.Time
}

type SessionRepository interface {
	Create(ctx context.Context, session *Session) error

	FindByID(ctx context.Context, sessionID string) (*Session, error)

	FindByUserID(ctx context.Context, userID string) ([]*Session, error)

	Delete(ctx context.Context, sessionID string) error

	DeleteAllForUser(ctx context.Context, userID string) error

	Cleanup(ctx context.Context) error
}

func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

func (s *Session) IsValid() bool {
	return !s.IsExpired()
}
