package repo

import (
	"time"

	"PR-reviewer/internal/models"
)

//go:generate minimock -i PR-reviewer/internal/repo.Repo -o mock_repo_test.go -n RepoMock -p repo
type Repo interface {
	InsertTeam(team models.Team) error
	GetTeam(teamName string) (models.Team, error)
	UpdateUserActive(userID string, isActive bool) (models.User, error)

	CreatePR(pr models.PullRequest) error
	GetPR(prID string) (models.PullRequest, error)
	MergePR(prID string, t time.Time) (models.PullRequest, error)
	ReplaceReviewer(prID, oldUID, newUID string) (models.PullRequest, error)

	GetActiveTeamMembersExcept(teamName, exceptUser string) ([]string, error)
	GetUserTeam(userID string) (string, error)
	GetPRsByReviewer(userID string) ([]models.PullRequestShort, error)
}
