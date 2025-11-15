package service

import (
	"PR-reviewer/internal/models"
	"context"
)

type Service interface {
	AddTeam(ctx context.Context, m models.Team) error
	GetTeam(ctx context.Context, name string) (models.Team, error)
	SetUserActive(ctx context.Context, userID string, active bool) (models.User, error)
	CreatePR(ctx context.Context, pr models.PullRequest) (models.PullRequest, error)
	MergePR(ctx context.Context, prID string) (models.PullRequest, error)
	Reassign(ctx context.Context, prID, oldUser string) (models.PullRequest, string, error)
	GetPRsByReviewer(ctx context.Context, userID string) ([]models.PullRequestShort, error)
	GetStats(ctx context.Context) (map[string]int, error)
	DeactivateTeam(ctx context.Context, teamName string) error

	EnqueueJob(job Job)
	StopWorkers()
}
