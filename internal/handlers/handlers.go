package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"PR-reviewer/internal/logger"
	"PR-reviewer/internal/models"
	"PR-reviewer/internal/service"
)

type Handler struct {
	svc service.Service
	log logger.Logger
}

func NewHandler(s service.Service, l logger.Logger) *Handler {
	return &Handler{svc: s, log: l}
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, map[string]interface{}{
		"error": map[string]string{
			"code":    errCode,
			"message": msg,
		},
	})
}

// /team/add
func (h *Handler) AddTeam(w http.ResponseWriter, r *http.Request) {
	h.log.Info("received request AddTeam")
	var team models.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		h.log.Warn("invalid request body", "error", err)
		writeError(w, 400, "INVALID", "invalid body")
		return
	}
	if team.TeamName == "" {
		h.log.Warn("team_name missing in request")
		writeError(w, 400, "INVALID", "team_name required")
		return
	}
	if err := h.svc.AddTeam(team); err != nil {
		h.log.Error("failed to add team", "team", team.TeamName, "error", err)
		writeError(w, 500, "ERROR", err.Error())
		return
	}
	writeJSON(w, 201, map[string]interface{}{"team": team})
}

// /team/get?team_name=...
func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	h.log.Info("received request GetTeam", "team_name", teamName)
	if teamName == "" {
		h.log.Warn("team_name missing in request")
		writeError(w, 400, "INVALID", "team_name required")
		return
	}

	job := service.Job{
		Type: "get_team",
		Payload: map[string]interface{}{
			"team": teamName,
		},
		RespCh: make(chan service.JobResult, 1),
	}

	h.svc.EnqueueJob(job)
	res := <-job.RespCh
	if res.Error != nil {
		if errors.Is(res.Error, service.ErrNotFound) {
			writeError(w, 404, "NOT_FOUND", "team not found")
			return
		}
		writeError(w, 500, "ERROR", res.Error.Error())
		return
	}
	writeJSON(w, 200, res.Data)
}

// /users/setIsActive
func (h *Handler) SetIsActive(w http.ResponseWriter, r *http.Request) {
	h.log.Info("received request SetIsActive")
	var payload struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.log.Warn("invalid request body", "error", err)
		writeError(w, 400, "INVALID", "invalid body")
		return
	}
	if payload.UserID == "" {
		h.log.Warn("user_id missing in request")
		writeError(w, 400, "INVALID", "user_id required")
		return
	}

	job := service.Job{
		Type: "set_user_active",
		Payload: map[string]interface{}{
			"uid":    payload.UserID,
			"active": payload.IsActive,
		},
		RespCh: make(chan service.JobResult, 1),
	}
	h.svc.EnqueueJob(job)
	res := <-job.RespCh
	if res.Error != nil {
		if errors.Is(res.Error, service.ErrNotFound) {
			writeError(w, 404, "NOT_FOUND", "user not found")
			return
		}
		writeError(w, 500, "ERROR", res.Error.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{"user": res.Data})
}

// /pullRequest/create
func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	h.log.Info("received request CreatePR")
	b, _ := io.ReadAll(r.Body)
	var payload struct {
		PullRequestID   string `json:"pull_request_id"`
		PullRequestName string `json:"pull_request_name"`
		AuthorID        string `json:"author_id"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		h.log.Warn("invalid request body", "error", err)
		writeError(w, 400, "INVALID", "invalid body")
		return
	}
	if payload.PullRequestID == "" || payload.PullRequestName == "" || payload.AuthorID == "" {
		h.log.Warn("missing fields in request", "payload", payload)
		writeError(w, 400, "INVALID", "missing fields")
		return
	}

	pr := models.PullRequest{
		PullRequestID:   payload.PullRequestID,
		PullRequestName: payload.PullRequestName,
		AuthorID:        payload.AuthorID,
	}

	job := service.Job{
		Type: "create_pr",
		Payload: map[string]interface{}{
			"pr": pr,
		},
		RespCh: make(chan service.JobResult, 1),
	}
	h.svc.EnqueueJob(job)
	res := <-job.RespCh
	if res.Error != nil {
		switch {
		case errors.Is(res.Error, service.ErrNotFound):
			writeError(w, 404, "NOT_FOUND", "author/team not found")
		case errors.Is(res.Error, service.ErrPRExists):
			writeError(w, 409, "PR_EXISTS", "PR id already exists")
		default:
			writeError(w, 500, "ERROR", res.Error.Error())
		}
		return
	}

	writeJSON(w, 201, map[string]interface{}{"pr": res.Data})
}

// /pullRequest/merge
func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	h.log.Info("received request MergePR")
	var payload struct {
		PullRequestID string `json:"pull_request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.log.Warn("invalid request body", "error", err)
		writeError(w, 400, "INVALID", "invalid body")
		return
	}
	if payload.PullRequestID == "" {
		h.log.Warn("pull_request_id missing in request")
		writeError(w, 400, "INVALID", "pull_request_id required")
		return
	}

	job := service.Job{
		Type: "merge_pr",
		Payload: map[string]interface{}{
			"pr_id": payload.PullRequestID,
		},
		RespCh: make(chan service.JobResult, 1),
	}

	h.svc.EnqueueJob(job)
	res := <-job.RespCh

	if res.Error != nil {
		if errors.Is(res.Error, service.ErrNotFound) {
			writeError(w, 404, "NOT_FOUND", "pr not found")
			return
		}
		writeError(w, 500, "ERROR", res.Error.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{"pr": res.Data})
}

// /pullRequest/reassign
func (h *Handler) Reassign(w http.ResponseWriter, r *http.Request) {
	h.log.Info("received request Reassign")
	var payload struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.log.Warn("invalid request body", "error", err)
		writeError(w, 400, "INVALID", "invalid body")
		return
	}
	if payload.PullRequestID == "" || payload.OldUserID == "" {
		h.log.Warn("missing fields in request", "payload", payload)
		writeError(w, 400, "INVALID", "missing fields")
		return
	}

	job := service.Job{
		Type: "reassign_pr",
		Payload: map[string]interface{}{
			"pr_id":    payload.PullRequestID,
			"old_user": payload.OldUserID,
		},
		RespCh: make(chan service.JobResult, 1),
	}

	h.svc.EnqueueJob(job)
	res := <-job.RespCh

	if res.Error != nil {
		switch {
		case errors.Is(res.Error, service.ErrNotFound):
			writeError(w, 404, "NOT_FOUND", "pr or user not found")
		case errors.Is(res.Error, service.ErrPRMerged):
			writeError(w, 409, "PR_MERGED", "cannot reassign on merged PR")
		case errors.Is(res.Error, service.ErrNotAssigned):
			writeError(w, 409, "NOT_ASSIGNED", "reviewer is not assigned to this PR")
		case errors.Is(res.Error, service.ErrNoCandidate):
			writeError(w, 409, "NO_CANDIDATE", "no active replacement candidate in team")
		default:
			writeError(w, 500, "ERROR", res.Error.Error())
		}
		return
	}

	data := res.Data.(map[string]interface{})
	writeJSON(w, 200, data)
}

// /users/getReview?user_id=...
func (h *Handler) GetUserReviews(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("user_id")
	h.log.Info("received request GetUserReviews", "user_id", uid)
	if uid == "" {
		h.log.Warn("user_id missing in request")
		writeError(w, 400, "INVALID", "user_id required")
		return
	}

	job := service.Job{
		Type: "get_reviews",
		Payload: map[string]interface{}{
			"uid": uid,
		},
		RespCh: make(chan service.JobResult, 1),
	}

	h.svc.EnqueueJob(job)
	res := <-job.RespCh

	if res.Error != nil {
		writeError(w, 500, "ERROR", res.Error.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{"user_id": uid, "pull_requests": res.Data})
}
