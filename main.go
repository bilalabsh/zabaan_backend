// Package main runs the Zabaan API server.
//
// Clean architecture: modules (auth, user, health) each have handler → service → repository
// where applicable. Main wires dependencies and registers routes.
//
// @title           Zabaan API
// @version         1.0
// @description     Backend API for Zabaan mobile app
// @host            localhost:8080
// @BasePath        /
// @schemes         http
package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/bilalabsh/zabaan_backend/docs"
	"github.com/bilalabsh/zabaan_backend/internal/auth"
	"github.com/bilalabsh/zabaan_backend/internal/config"
	"github.com/bilalabsh/zabaan_backend/internal/database"
	"github.com/bilalabsh/zabaan_backend/internal/health"
	"github.com/bilalabsh/zabaan_backend/internal/middleware"
	"github.com/bilalabsh/zabaan_backend/internal/user"
	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("config validation failed", "err", err)
		os.Exit(1)
	}
	// Structured logging: JSON in production for aggregators, text in development for readability.
	if strings.ToLower(cfg.Environment) == "production" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	} else {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	}
	database.Init(cfg)
	defer database.Close()

	// Wire modules: repository → service → handler
	userRepo := user.NewRepository(database.DB)
	userSvc := user.NewService(userRepo)
	userHandler := user.NewHandler(userSvc)

	authSvc := auth.NewService(userRepo, cfg.JWTSecret, cfg.TokenExpiry, cfg.RevocationTolerance)
	authHandler := auth.NewHandler(authSvc)
	authRateLimiter := middleware.NewAuthRateLimiter(time.Minute, 10, cfg.TrustProxy)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", health.Check)
	mux.HandleFunc("/users", middleware.RequireAuth(authSvc, userHandler.Users))
	mux.HandleFunc("/users/", middleware.RequireAuth(authSvc, userHandler.Users))
	mux.HandleFunc("/signup", authRateLimiter.Wrap(authHandler.Signup))
	mux.HandleFunc("/signup/", authRateLimiter.Wrap(authHandler.Signup))
	mux.HandleFunc("/login", authRateLimiter.Wrap(authHandler.Login))
	mux.HandleFunc("/login/", authRateLimiter.Wrap(authHandler.Login))
	mux.HandleFunc("/getToken", authRateLimiter.Wrap(authHandler.GetToken))
	mux.HandleFunc("/getToken/", authRateLimiter.Wrap(authHandler.GetToken))
	mux.HandleFunc("/docs/", httpSwagger.WrapHandler)
	mux.HandleFunc("/", health.Root)

	serverAddr := ":" + cfg.Port
	slog.Info("server listening", "addr", serverAddr, "routes", "/signup, /login, /getToken, /users, /health")

	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		slog.Error("server failed to start", "err", err)
		os.Exit(1)
	}
}
