package models

import "time"

type TeamMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

type Team struct {
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

type User struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}

type PullRequest struct {
	PullRequestID     string       `json:"pull_request_id"`
	PullRequestName   string       `json:"pull_request_name"`
	AuthorID          string       `json:"author_id"`
	Status            string       `json:"status"`
	Assigned          []PRReviewer `json:"assigned_reviewers"`
	NeedMoreReviewers bool         `json:"need_more_reviewers"`
	CreatedAt         time.Time    `json:"createdAt,omitempty"`
	MergedAt          *time.Time   `json:"mergedAt,omitempty"`
}

type PRReviewer struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}
type PullRequestShort struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
	Status          string `json:"status"`
}
