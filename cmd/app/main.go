package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"pr-review-manager/internal/handlers"
	"pr-review-manager/internal/repository"
	"pr-review-manager/internal/service"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Error("DATABASE_URL is not set")
		os.Exit(1)
	}

	storage, err := repository.NewStorage(dbURL)
	if err != nil {
		logger.Error("Failed to initialize DB", err)
		os.Exit(1)
	}
	logger.Info("DB connection established successfully")

	// Создание таблиц при старте приложения
	logger.Info("Creating tables if not exist")
	if err := storage.CreateTables(logger); err != nil {
		logger.Error("Failed to create tables", err)
		os.Exit(1)
	}
	logger.Info("Tables creation completed")

	svc := service.NewService(storage, logger)
	h := handlers.NewHandler(svc, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/stats/users", h.StatsUsersHandler)
	mux.HandleFunc("/team/add", h.AddHandler)
	mux.HandleFunc("/team/get", h.GetHandler)
	mux.HandleFunc("/users/setIsActive", h.SetIsActiveHandler)
	mux.HandleFunc("/users/getReview", h.GetReviewHandler)
	mux.HandleFunc("/pullRequest/create", h.CreateHandler)
	mux.HandleFunc("/pullRequest/merge", h.MergeHandler)
	mux.HandleFunc("/pullRequest/reassign", h.ReassignHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server ListenAndServe error", err)
		}
	}()

	logger.Info("Server started on :8080")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Warn("Server forced to shutdown", err)
	} else {
		logger.Info("Server stopped gracefully")
	}

	if err := storage.Close(); err != nil {
		logger.Warn("Database close error", err)
	} else {
		logger.Info("Database connection closed")
	}
}
