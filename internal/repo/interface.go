package repo

import (
	"context"
	"time"

	"PR-reviewer/internal/models"
)

//go:generate minimock -i PR-reviewer/internal/repo.Repo -o mock_repo_test.go -n RepoMock -p repo
type Repo interface {
	InsertTeam(ctx context.Context, team models.Team) error
	GetTeam(ctx context.Context, teamName string) (models.Team, error)
	UpdateUserActive(ctx context.Context, userID string, isActive bool) (models.User, error)

	CreatePR(ctx context.Context, pr models.PullRequest) error
	GetPR(ctx context.Context, prID string) (models.PullRequest, error)
	MergePR(ctx context.Context, prID string, t time.Time) (models.PullRequest, error)
	ReplaceReviewer(ctx context.Context, prID, oldUID, newUID string) (models.PullRequest, error)

	GetActiveTeamMembersExcept(ctx context.Context, teamName, exceptUser string) ([]string, error)
	GetUserTeam(ctx context.Context, userID string) (string, error)
	GetPRsByReviewer(ctx context.Context, userID string) ([]models.PullRequestShort, error)
	GetUser(ctx context.Context, userID string) (models.User, error)
	GetReviewerStats(ctx context.Context) (map[string]int, error)
	SetTeamActive(ctx context.Context, teamName string, isActive bool) error
}
