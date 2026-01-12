package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/zenkiet/zen-attendance/apps/server/internal/domain"
	"github.com/zenkiet/zen-attendance/apps/server/internal/repository"
	"github.com/zenkiet/zen-attendance/apps/server/pkg/hash"
	"github.com/zenkiet/zen-attendance/apps/server/pkg/token"
	"github.com/zenkiet/zen-attendance/apps/server/pkg/validator"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountSuspended   = errors.New("account has been suspended")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrSessionExpired     = errors.New("session has expired")
)

const (
	DefaultTokenDuration   = 24 * time.Hour     // 24 hours
	DefaultRefreshDuration = 7 * 24 * time.Hour // 7 days
)

type DeviceInfo struct {
	IPAddress         string
	UserAgent         string
	DeviceFingerprint string
}

type AuthService struct {
	userRepo      domain.UserRepository
	sessionRepo   domain.SessionRepository
	tokenMaker    *token.Maker
	redisClient   *redis.Client
	tokenDuration time.Duration
}

func NewAuthService(
	userRepo domain.UserRepository,
	sessionRepo domain.SessionRepository,
	tokenMaker *token.Maker,
	redisClient *redis.Client,
) *AuthService {
	return &AuthService{
		userRepo:      userRepo,
		sessionRepo:   sessionRepo,
		tokenMaker:    tokenMaker,
		redisClient:   redisClient,
		tokenDuration: DefaultTokenDuration,
	}
}

func (s *AuthService) Register(ctx context.Context, email, password, fullName string) (*domain.User, error) {
	validatedEmail, err := validator.ValidateEmail(email)
	if err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}

	if err := validator.ValidatePassword(password); err != nil {
		return nil, err
	}

	if err := validator.ValidateName(fullName); err != nil {
		return nil, err
	}

	sanitizedName := validator.SanitizeName(fullName)

	// Check if user already exists
	existingUser, err := s.userRepo.FindByEmail(ctx, validatedEmail)
	if err == nil && existingUser != nil {
		return nil, repository.ErrUserAlreadyExists
	}

	// Hash password
	passwordHash, err := hash.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &domain.User{
		Email:        validatedEmail,
		PasswordHash: passwordHash,
		FullName:     sanitizedName,
		Role:         domain.RoleEmployee, // Default role
		Status:       domain.StatusActive,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// Login authenticates a user and creates a session
func (s *AuthService) Login(ctx context.Context, email, password string, deviceInfo DeviceInfo) (string, *domain.User, error) {
	// Validate email
	validatedEmail, err := validator.ValidateEmail(email)
	if err != nil {
		return "", nil, ErrInvalidCredentials
	}

	// Find user by email
	user, err := s.userRepo.FindByEmail(ctx, validatedEmail)
	if err != nil {
		// Don't reveal whether email exists or not
		return "", nil, ErrInvalidCredentials
	}

	// Check account status
	if !user.IsActive() {
		if user.Status == domain.StatusSuspended {
			return "", nil, ErrAccountSuspended
		}
		return "", nil, ErrInvalidCredentials
	}

	// Verify password (constant-time comparison)
	valid, err := hash.VerifyPassword(password, user.PasswordHash)
	if err != nil || !valid {
		return "", nil, ErrInvalidCredentials
	}

	// Create session
	sessionID := uuid.New().String()
	session := &domain.Session{
		ID:                sessionID,
		UserID:            user.ID,
		DeviceFingerprint: deviceInfo.DeviceFingerprint,
		IPAddress:         deviceInfo.IPAddress,
		UserAgent:         deviceInfo.UserAgent,
		ExpiresAt:         time.Now().Add(s.tokenDuration),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return "", nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Generate PASETO token with session ID as subject
	tokenString, err := s.tokenMaker.CreateToken(sessionID, s.tokenDuration)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create token: %w", err)
	}

	// Update last login timestamp
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		// Non-critical error, just log it
		fmt.Printf("Warning: failed to update last login for user %s: %v\n", user.ID, err)
	}

	return tokenString, user, nil
}

// ValidateToken validates a token and returns the associated user
func (s *AuthService) ValidateToken(ctx context.Context, tokenString string) (*domain.User, *domain.Session, error) {
	// Parse and verify PASETO token
	parsedToken, err := s.tokenMaker.VerifyToken(tokenString)
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	// Extract session ID from subject claim
	sessionID, err := parsedToken.GetSubject()
	if err != nil {
		return nil, nil, ErrInvalidToken
	}

	// Find session in database
	session, err := s.sessionRepo.FindByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil, nil, ErrInvalidToken
		}
		return nil, nil, fmt.Errorf("failed to find session: %w", err)
	}

	// Check if session is expired
	if session.IsExpired() {
		// Delete expired session
		_ = s.sessionRepo.Delete(ctx, sessionID)
		return nil, nil, ErrSessionExpired
	}

	// Find user
	user, err := s.userRepo.FindByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, nil, ErrInvalidToken
		}
		return nil, nil, fmt.Errorf("failed to find user: %w", err)
	}

	// Check if user is still active
	if !user.IsActive() {
		// Invalidate session for suspended/deleted users
		_ = s.sessionRepo.Delete(ctx, sessionID)
		return nil, nil, ErrAccountSuspended
	}

	return user, session, nil
}

// Logout invalidates a session
func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	if err := s.sessionRepo.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}

	return nil
}

// LogoutAll invalidates all sessions for a user
func (s *AuthService) LogoutAll(ctx context.Context, userID string) error {
	if err := s.sessionRepo.DeleteAllForUser(ctx, userID); err != nil {
		return fmt.Errorf("failed to logout all sessions: %w", err)
	}

	return nil
}

// RefreshToken creates a new token for an existing valid session
func (s *AuthService) RefreshToken(ctx context.Context, oldTokenString string) (string, error) {
	// Validate old token
	_, session, err := s.ValidateToken(ctx, oldTokenString)
	if err != nil {
		return "", err
	}

	// Create new session
	newSessionID := uuid.New().String()
	newSession := &domain.Session{
		ID:                newSessionID,
		UserID:            session.UserID,
		DeviceFingerprint: session.DeviceFingerprint,
		IPAddress:         session.IPAddress,
		UserAgent:         session.UserAgent,
		ExpiresAt:         time.Now().Add(s.tokenDuration),
	}

	if err := s.sessionRepo.Create(ctx, newSession); err != nil {
		return "", fmt.Errorf("failed to create new session: %w", err)
	}

	// Delete old session
	if err := s.sessionRepo.Delete(ctx, session.ID); err != nil {
		// Non-critical, log and continue
		fmt.Printf("Warning: failed to delete old session %s: %v\n", session.ID, err)
	}

	// Generate new token
	newToken, err := s.tokenMaker.CreateToken(newSessionID, s.tokenDuration)
	if err != nil {
		return "", fmt.Errorf("failed to create new token: %w", err)
	}

	return newToken, nil
}

// CleanupExpiredSessions removes expired sessions from database
func (s *AuthService) CleanupExpiredSessions(ctx context.Context) error {
	return s.sessionRepo.Cleanup(ctx)
}

// SetTokenDuration allows customizing token duration
func (s *AuthService) SetTokenDuration(duration time.Duration) {
	s.tokenDuration = duration
}
