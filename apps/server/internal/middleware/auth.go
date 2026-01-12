package middleware

import (
	"net/http"
	"strings"

	"github.com/zenkiet/zen-attendance/apps/server/internal/handler"
	"github.com/zenkiet/zen-attendance/apps/server/internal/service"
)

// AuthMiddleware creates middleware that requires authentication
func AuthMiddleware(authService *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondUnauthorized(w, "missing_token", "Authorization header required")
				return
			}

			// Check Bearer prefix
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				respondUnauthorized(w, "invalid_token_format", "Authorization header must be: Bearer <token>")
				return
			}

			tokenString := parts[1]

			// Validate token
			user, session, err := authService.ValidateToken(r.Context(), tokenString)
			if err != nil {
				if err == service.ErrInvalidToken {
					respondUnauthorized(w, "invalid_token", "Invalid or expired token")
					return
				}
				if err == service.ErrSessionExpired {
					respondUnauthorized(w, "session_expired", "Session has expired")
					return
				}
				if err == service.ErrAccountSuspended {
					respondForbidden(w, "account_suspended", "Account has been suspended")
					return
				}
				respondUnauthorized(w, "authentication_failed", "Authentication failed")
				return
			}

			// Inject user and session into context
			ctx := handler.SetUserInContext(r.Context(), user)
			ctx = handler.SetSessionInContext(ctx, session)

			// Continue to next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Helper to respond with 401 Unauthorized
func respondUnauthorized(w http.ResponseWriter, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":"` + errorCode + `","message":"` + message + `"}`))
}

// Helper to respond with 403 Forbidden
func respondForbidden(w http.ResponseWriter, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":"` + errorCode + `","message":"` + message + `"}`))
}
