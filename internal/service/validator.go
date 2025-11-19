package service

import (
	"PR-reviewer/internal/models"
	"errors"
)

var (
	errMissingPRID     = errors.New("pull_request_id required")
	errMissingPRName   = errors.New("pull_request_name required")
	errMissingAuthorID = errors.New("author_id required")
	errMissingUserID   = errors.New("user_id required")
	errMissingTeamName = errors.New("team_name required")
	errDuplicates      = errors.New("duplicates user_id's")
)

func validatePullRequest(pr models.PullRequest) error {
	if pr.PullRequestID == "" {
		return errMissingPRID
	}
	if pr.PullRequestName == "" {
		return errMissingPRName
	}
	if pr.AuthorID == "" {
		return errMissingAuthorID
	}
	return nil
}

func validateUserID(userID string) error {
	if userID == "" {
		return errMissingUserID
	}
	return nil
}

func validateTeam(team models.Team) error {
	if team.TeamName == "" {
		return errMissingTeamName
	}
	userIDs := make(map[string]bool)
	for _, member := range team.Members {
		if userIDs[member.UserID] {
			return errDuplicates
		}
		userIDs[member.UserID] = true
	}
	return nil
}

func validateTeamName(name string) error {
	if name == "" {
		return errMissingTeamName
	}
	return nil
}
