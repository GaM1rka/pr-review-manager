package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"pr-review-manager/internal/handlers"
	"pr-review-manager/internal/repository"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	dbURL := os.Getenv("DATABASE_URL")
	db, err := repository.NewStorage(dbURL)
	if err != nil {
		logger.Error("Error while initialization of DB", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/team/add", handlers.AddHandler)
	mux.HandleFunc("/team/get", handlers.GetHandler)
	mux.HandleFunc("/users/setIsActive", handlers.SetIsActiveHandler)
	mux.HandleFunc("/users/getReview", handlers.GetReviewHandler)
	mux.HandleFunc("/pullRequest/create", handlers.CreateHandler)
	mux.HandleFunc("/pullRequest/merge", handlers.MergeHandler)
	mux.HandleFunc("/pullRequest/reassign", handlers.ReassignHandler)

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

	if err := db.Close(); err != nil {
		logger.Warn("Database close error", err)
	}
}
