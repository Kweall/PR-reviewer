package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"

	"PR-reviewer/internal/handlers"
	"PR-reviewer/internal/logger"
	"PR-reviewer/internal/repo"
	"PR-reviewer/internal/service"
)

func mustEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	dsn := mustEnv("DATABASE_DSN", "postgres://pruser:prpass@localhost:5432/prdb?sslmode=disable")
	port := mustEnv("PORT", "8080")

	appLog := logger.NewStdLogger(os.Stdout, "debug")

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		appLog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	for range 10 {
		if err := db.Ping(); err != nil {
			fmt.Println("Waiting for DB to be ready...")
			time.Sleep(1 * time.Second)
			continue
		}
		fmt.Println("DB is ready!")
		break
	}

	repo := repo.NewPostgresRepo(db)
	svc := service.NewService(repo, appLog)
	h := handlers.NewHandler(svc, appLog)

	r := chi.NewRouter()
	r.Post("/team/add", h.AddTeam)
	r.Get("/team/get", h.GetTeam)
	r.Post("/users/setIsActive", h.SetIsActive)
	r.Post("/pullRequest/create", h.CreatePR)
	r.Post("/pullRequest/merge", h.MergePR)
	r.Post("/pullRequest/reassign", h.Reassign)
	r.Get("/users/getReview", h.GetUserReviews)

	s := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	appLog.Info("server starting", "port", port)
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		appLog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
