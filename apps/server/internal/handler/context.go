package handler

import (
	"context"
	"errors"

	"github.com/zenkiet/zen-attendance/apps/server/internal/domain"
)

type contextKey string

const (
	userContextKey    contextKey = "user"
	sessionContextKey contextKey = "session"
)

var (
	ErrUserNotInContext    = errors.New("user not found in context")
	ErrSessionNotInContext = errors.New("session not found in context")
)

// Context helper functions for handlers and middleware

func SetUserInContext(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func GetUserFromContext(ctx context.Context) (*domain.User, error) {
	user, ok := ctx.Value(userContextKey).(*domain.User)
	if !ok || user == nil {
		return nil, ErrUserNotInContext
	}
	return user, nil
}

func SetSessionInContext(ctx context.Context, session *domain.Session) context.Context {
	return context.WithValue(ctx, sessionContextKey, session)
}

func GetSessionFromContext(ctx context.Context) (*domain.Session, error) {
	session, ok := ctx.Value(sessionContextKey).(*domain.Session)
	if !ok || session == nil {
		return nil, ErrSessionNotInContext
	}
	return session, nil
}
