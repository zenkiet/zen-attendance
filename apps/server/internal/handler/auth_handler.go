package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/zenkiet/zen-attendance/apps/server/internal/domain"
	"github.com/zenkiet/zen-attendance/apps/server/internal/repository"
	"github.com/zenkiet/zen-attendance/apps/server/internal/service"
)

// DTOs for request/response
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"fullName"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	Token string `json:"token"`
}

type AuthResponse struct {
	Token     string  `json:"token"`
	ExpiresIn int64   `json:"expiresIn"` // seconds
	User      UserDTO `json:"user"`
}

type UserDTO struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"fullName"`
	Role     string `json:"role"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Register handles user registration
// POST /api/v1/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Register user
	user, err := h.authService.Register(r.Context(), req.Email, req.Password, req.FullName)
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			respondError(w, http.StatusConflict, "email_exists", "Email already registered")
			return
		}
		// Validation errors from validator package
		if strings.Contains(err.Error(), "password") || strings.Contains(err.Error(), "email") || strings.Contains(err.Error(), "name") {
			respondError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "Failed to register user")
		return
	}

	// Auto-login after registration
	deviceInfo := extractDeviceInfo(r)
	token, _, err := h.authService.Login(r.Context(), req.Email, req.Password, deviceInfo)
	if err != nil {
		// Registration succeeded but login failed - still return success
		respondJSON(w, http.StatusCreated, map[string]interface{}{
			"message": "Registration successful. Please login.",
			"user":    toUserDTO(user),
		})
		return
	}

	// Return token
	respondJSON(w, http.StatusCreated, AuthResponse{
		Token:     token,
		ExpiresIn: int64(service.DefaultTokenDuration.Seconds()),
		User:      toUserDTO(user),
	})
}

// Login handles user authentication
// POST /api/v1/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	deviceInfo := extractDeviceInfo(r)
	token, user, err := h.authService.Login(r.Context(), req.Email, req.Password, deviceInfo)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			respondError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid email or password")
			return
		}
		if errors.Is(err, service.ErrAccountSuspended) {
			respondError(w, http.StatusForbidden, "account_suspended", "Your account has been suspended")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "Login failed")
		return
	}

	respondJSON(w, http.StatusOK, AuthResponse{
		Token:     token,
		ExpiresIn: int64(service.DefaultTokenDuration.Seconds()),
		User:      toUserDTO(user),
	})
}

// GetMe returns current authenticated user info
// GET /api/v1/me
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	respondJSON(w, http.StatusOK, toUserDTO(user))
}

// Logout invalidates the current session
// POST /api/v1/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session, err := GetSessionFromContext(r.Context())
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if err := h.authService.Logout(r.Context(), session.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "Logout failed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully",
	})
}

// Refresh generates a new token from an existing valid token
// POST /api/v1/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	newToken, err := h.authService.RefreshToken(r.Context(), req.Token)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) || errors.Is(err, service.ErrSessionExpired) {
			respondError(w, http.StatusUnauthorized, "invalid_token", "Token is invalid or expired")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", "Token refresh failed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"token":     newToken,
		"expiresIn": int64(service.DefaultTokenDuration.Seconds()),
	})
}

// Helper functions

func toUserDTO(user *domain.User) UserDTO {
	return UserDTO{
		ID:       user.ID,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     user.Role,
	}
}

func extractDeviceInfo(r *http.Request) service.DeviceInfo {
	return service.DeviceInfo{
		IPAddress:         getRealIP(r),
		UserAgent:         r.UserAgent(),
		DeviceFingerprint: "", // TODO: Extract from JA4 middleware
	}
}

func getRealIP(r *http.Request) string {
	// Check X-Forwarded-For header (from chi RealIP middleware)
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Fallback to RemoteAddr
	return strings.Split(r.RemoteAddr, ":")[0]
}

func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	respondJSON(w, statusCode, ErrorResponse{
		Error:   errorCode,
		Message: message,
	})
}
