package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"PR-reviewer/internal/models"
)

var (
	errMissingTeamName      = errors.New("team_name required")
	errMissingUserID        = errors.New("user_id required")
	errMissingPullRequestID = errors.New("pull_request_id required")
	errMissingFieldsPR      = errors.New("missing fields")
	errInvalidBody          = errors.New("invalid body")
)

func decodeBody(r *http.Request, dst interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return errInvalidBody
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return errInvalidBody
	}
	return nil
}

func validateTeam(team models.Team) error {
	if team.TeamName == "" {
		return errMissingTeamName
	}
	return nil
}

func validateSetActivePayload(payload struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}) error {
	if payload.UserID == "" {
		return errMissingUserID
	}
	return nil
}

func validateCreatePRPayload(payload struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
}) error {
	if payload.PullRequestID == "" || payload.PullRequestName == "" || payload.AuthorID == "" {
		return errMissingFieldsPR
	}
	return nil
}

func validateMergePRPayload(payload struct {
	PullRequestID string `json:"pull_request_id"`
}) error {
	if payload.PullRequestID == "" {
		return errMissingPullRequestID
	}
	return nil
}

func validateReassignPayload(payload struct {
	PullRequestID string `json:"pull_request_id"`
	OldUserID     string `json:"old_user_id"`
}) error {
	if payload.PullRequestID == "" || payload.OldUserID == "" {
		return errMissingFieldsPR
	}
	return nil
}

func validateGetTeamRequest(req getTeamRequest) error {
	if req.TeamName == "" {
		return errMissingTeamName
	}
	return nil
}

func validateGetUserReviewsRequest(req getUserReviewsRequest) error {
	if req.UserID == "" {
		return errMissingUserID
	}
	return nil
}
