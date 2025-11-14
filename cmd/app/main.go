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
	storage, err := repository.NewStorage(dbURL)
	if err != nil {
		logger.Error("Error while initialization of DB", err)
	}
	service := service.NewService(storage)
	h := handlers.NewHandler(service)

	mux := http.NewServeMux()
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
			logger.Error("Listen error:", err)
		}
	}()
	logger.Info("App started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Warn("Server forced to shutdown", err)
	}

	if err := storage.Close(); err != nil {
		logger.Warn("Database close error", err)
	}
}
