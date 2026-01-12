package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/zenkiet/zen-attendance/apps/server/internal/config"
	"github.com/zenkiet/zen-attendance/apps/server/internal/handler"
	customMiddleware "github.com/zenkiet/zen-attendance/apps/server/internal/middleware"
	"github.com/zenkiet/zen-attendance/apps/server/internal/service"
)

type Server struct {
	router      *chi.Mux
	db          *pgxpool.Pool
	redis       *redis.Client
	cfg         *config.Config
	server      *http.Server
	authService *service.AuthService
	authHandler *handler.AuthHandler
}

func New(cfg *config.Config, db *pgxpool.Pool, rdb *redis.Client, authService *service.AuthService, authHandler *handler.AuthHandler) *Server {
	s := &Server{
		router:      chi.NewRouter(),
		db:          db,
		redis:       rdb,
		cfg:         cfg,
		authService: authService,
		authHandler: authHandler,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))
	s.router.Use(customMiddleware.JA4Middleware)
}

func (s *Server) setupRoutes() {
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Post("/register", s.authHandler.Register)
		r.Post("/login", s.authHandler.Login)
		r.Post("/refresh", s.authHandler.Refresh)

		// Protected routes (authentication required)
		r.Group(func(r chi.Router) {
			// Apply authentication middleware
			r.Use(customMiddleware.AuthMiddleware(s.authService))

			// Authenticated endpoints
			r.Get("/me", s.authHandler.GetMe)
			r.Post("/logout", s.authHandler.Logout)

			// Future: Attendance endpoints will go here
			// r.Post("/attendance", s.handleAttendance)
		})
	})
}

func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:    ":" + s.cfg.Port,
		Handler: s.router,
	}

	fmt.Printf("Server starting on port %s\n", s.cfg.Port)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
