package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zenkiet/zen-attendance/apps/server/internal/config"
	"github.com/zenkiet/zen-attendance/apps/server/internal/handler"
	"github.com/zenkiet/zen-attendance/apps/server/internal/repository"
	"github.com/zenkiet/zen-attendance/apps/server/internal/server"
	"github.com/zenkiet/zen-attendance/apps/server/internal/service"
	"github.com/zenkiet/zen-attendance/apps/server/pkg/database"
	"github.com/zenkiet/zen-attendance/apps/server/pkg/token"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Database
	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)
	db, err := database.NewPostgres(ctx, connString)
	if err != nil {
		log.Fatalf("Failed to connect to Database: %v", err)
	}
	defer db.Close()
	log.Println("Database connected")

	// Migration Database
	migrationDir := "./migrations"
	if err := database.Migrate(connString, migrationDir); err != nil {
		log.Fatalf("Failed to migrate Database: %v", err)
	}
	log.Println("Database migrated")

	// Initialize Redis
	redisAddr := fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort)
	rdb, err := database.NewRedis(ctx, redisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("Failed to connect to Cache Memory: %v", err)
	}
	defer rdb.Close()
	log.Println("Redis connected")

	// Initialize token maker
	tokenMaker, err := token.NewMaker(cfg.PasetoKey, []byte("zen-attendance"))
	if err != nil {
		log.Fatalf("Failed to create Token Maker: %v", err)
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	// Initialize authentication service
	authService := service.NewAuthService(userRepo, sessionRepo, tokenMaker, rdb)

	// Initialize HTTP handlers
	authHandler := handler.NewAuthHandler(authService)

	// Initialize server with all dependencies
	srv := server.New(cfg, db, rdb, authService, authHandler)

	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited properly")
}
