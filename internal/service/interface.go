package service

import "PR-reviewer/internal/models"

type Service interface {
	AddTeam(models.Team) error
	GetTeam(name string) (models.Team, error)
	SetUserActive(userID string, active bool) (models.User, error)
	CreatePR(pr models.PullRequest) (models.PullRequest, error)
	MergePR(prID string) (models.PullRequest, error)
	Reassign(prID, oldUser string) (models.PullRequest, string, error)
	GetPRsByReviewer(userID string) ([]models.PullRequestShort, error)

	EnqueueJob(job Job)
	StopWorkers()
}
