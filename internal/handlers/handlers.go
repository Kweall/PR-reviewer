package handlers

import (
	"context"
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

func decodeJSON(body io.ReadCloser, v interface{}) error {
	defer body.Close()
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

func (h *Handler) AddTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.log.Info("received request AddTeam")

	var team models.Team
	if err := decodeBody(r, &team); err != nil {
		h.log.Warn("invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID", "invalid body")
		return
	}

	if err := validateTeam(team); err != nil {
		h.log.Warn("validation failed", "team", team, "error", err)
		writeError(w, http.StatusBadRequest, "INVALID", err.Error())
		return
	}

	if err := h.svc.AddTeam(ctx, team); err != nil {
		h.log.Error("failed to add team", "team", team.TeamName, "error", err)
		writeError(w, http.StatusInternalServerError, "ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"team": team})
}

func (h *Handler) SetIsActive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.log.Info("received request SetIsActive")

	var payload struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}
	if err := decodeBody(r, &payload); err != nil {
		h.log.Warn("invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID", "invalid body")
		return
	}

	if err := validateSetActivePayload(payload); err != nil {
		h.log.Warn("validation failed", "user_id", payload.UserID, "error", err)
		writeError(w, http.StatusBadRequest, "INVALID", err.Error())
		return
	}

	job := service.Job{
		Type: "set_user_active",
		Payload: map[string]interface{}{
			"uid":    payload.UserID,
			"active": payload.IsActive,
		},
		RespCh: make(chan service.JobResult, 1),
		Ctx:    ctx,
	}
	h.svc.EnqueueJob(job)

	res, err := waitJob(ctx, job.RespCh)
	if err != nil {
		writeError(w, http.StatusGatewayTimeout, "CANCELED", "request canceled")
		return
	}

	if res.Error != nil {
		if errors.Is(res.Error, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "ERROR", res.Error.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"user": res.Data})
}

func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.log.Info("received request CreatePR")

	var payload struct {
		PullRequestID   string `json:"pull_request_id"`
		PullRequestName string `json:"pull_request_name"`
		AuthorID        string `json:"author_id"`
	}
	if err := decodeBody(r, &payload); err != nil {
		h.log.Warn("invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID", "invalid body")
		return
	}

	if err := validateCreatePRPayload(payload); err != nil {
		h.log.Warn("validation failed", "payload", payload, "error", err)
		writeError(w, http.StatusBadRequest, "INVALID", err.Error())
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
		Ctx:    ctx,
	}
	h.svc.EnqueueJob(job)

	res, err := waitJob(ctx, job.RespCh)
	if err != nil {
		writeError(w, http.StatusGatewayTimeout, "CANCELED", "request canceled")
		return
	}

	if res.Error != nil {
		switch {
		case errors.Is(res.Error, service.ErrNotFound):
			writeError(w, http.StatusNotFound, "NOT_FOUND", "author/team not found")
		case errors.Is(res.Error, service.ErrPRExists):
			writeError(w, http.StatusConflict, "PR_EXISTS", "PR id already exists")
		default:
			writeError(w, http.StatusInternalServerError, "ERROR", res.Error.Error())
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"pr": res.Data})
}

func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.log.Info("received request MergePR")

	var payload struct {
		PullRequestID string `json:"pull_request_id"`
	}
	if err := decodeBody(r, &payload); err != nil {
		h.log.Warn("invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID", "invalid body")
		return
	}

	if err := validateMergePRPayload(payload); err != nil {
		h.log.Warn("validation failed", "pull_request_id", payload.PullRequestID, "error", err)
		writeError(w, http.StatusBadRequest, "INVALID", err.Error())
		return
	}

	job := service.Job{
		Type: "merge_pr",
		Payload: map[string]interface{}{
			"pr_id": payload.PullRequestID,
		},
		RespCh: make(chan service.JobResult, 1),
		Ctx:    ctx,
	}
	h.svc.EnqueueJob(job)

	res, err := waitJob(ctx, job.RespCh)
	if err != nil {
		writeError(w, http.StatusGatewayTimeout, "CANCELED", "request canceled")
		return
	}

	if res.Error != nil {
		if errors.Is(res.Error, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "pr not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "ERROR", res.Error.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"pr": res.Data})
}

func (h *Handler) Reassign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.log.Info("received request Reassign")

	var payload struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
	}
	if err := decodeBody(r, &payload); err != nil {
		h.log.Warn("invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID", "invalid body")
		return
	}

	if err := validateReassignPayload(payload); err != nil {
		h.log.Warn("validation failed", "payload", payload, "error", err)
		writeError(w, http.StatusBadRequest, "INVALID", err.Error())
		return
	}

	job := service.Job{
		Type: "reassign_pr",
		Payload: map[string]interface{}{
			"pr_id":    payload.PullRequestID,
			"old_user": payload.OldUserID,
		},
		RespCh: make(chan service.JobResult, 1),
		Ctx:    ctx,
	}
	h.svc.EnqueueJob(job)

	res, err := waitJob(ctx, job.RespCh)
	if err != nil {
		writeError(w, http.StatusGatewayTimeout, "CANCELED", "request canceled")
		return
	}

	if res.Error != nil {
		switch {
		case errors.Is(res.Error, service.ErrNotFound):
			writeError(w, http.StatusNotFound, "NOT_FOUND", "pr or user not found")
		case errors.Is(res.Error, service.ErrPRMerged):
			writeError(w, http.StatusConflict, "PR_MERGED", "cannot reassign on merged PR")
		case errors.Is(res.Error, service.ErrNotAssigned):
			writeError(w, http.StatusConflict, "NOT_ASSIGNED", "reviewer is not assigned to this PR")
		case errors.Is(res.Error, service.ErrNoCandidate):
			writeError(w, http.StatusConflict, "NO_CANDIDATE", "no active replacement candidate in team")
		default:
			writeError(w, http.StatusInternalServerError, "ERROR", res.Error.Error())
		}
		return
	}

	data := res.Data.(map[string]interface{})
	writeJSON(w, http.StatusOK, data)
}

type getTeamRequest struct {
	TeamName string
}

func (h *Handler) GetTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := getTeamRequest{
		TeamName: r.URL.Query().Get("team_name"),
	}

	if err := validateGetTeamRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID", err.Error())
		return
	}

	job := service.Job{
		Type: "get_team",
		Payload: map[string]interface{}{
			"team": req.TeamName,
		},
		RespCh: make(chan service.JobResult, 1),
		Ctx:    ctx,
	}
	h.svc.EnqueueJob(job)

	res, err := waitJob(ctx, job.RespCh)
	if err != nil {
		writeError(w, http.StatusGatewayTimeout, "CANCELED", "request canceled")
		return
	}

	if res.Error != nil {
		if errors.Is(res.Error, service.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "team not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "ERROR", res.Error.Error())
		return
	}

	writeJSON(w, http.StatusOK, res.Data)
}

type getUserReviewsRequest struct {
	UserID string
}

func (h *Handler) GetUserReviews(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.log.Info("received request GetUserReviews")
	req := getUserReviewsRequest{
		UserID: r.URL.Query().Get("user_id"),
	}

	if err := validateGetUserReviewsRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID", err.Error())
		return
	}

	job := service.Job{
		Type: "get_reviews",
		Payload: map[string]interface{}{
			"uid": req.UserID,
		},
		RespCh: make(chan service.JobResult, 1),
		Ctx:    ctx,
	}
	h.svc.EnqueueJob(job)

	res, err := waitJob(ctx, job.RespCh)
	if err != nil {
		writeError(w, http.StatusGatewayTimeout, "CANCELED", "request canceled")
		return
	}

	if res.Error != nil {
		writeError(w, http.StatusInternalServerError, "ERROR", res.Error.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"user_id": req.UserID, "pull_requests": res.Data})
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.log.Info("received request GetStats")
	stats, err := h.svc.GetStats(ctx)
	if err != nil {
		h.log.Error("failed to get stats", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": map[string]string{
				"code":    "ERROR",
				"message": err.Error(),
			},
		})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) DeactivateTeam(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.log.Info("received request deactivate team")

	type req struct {
		Team string `json:"team_name"`
	}
	var body req
	if err := decodeJSON(r.Body, &body); err != nil {
		h.log.Error("invalid request body", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	respCh := make(chan service.JobResult, 1)
	job := service.Job{
		Type: "deactivate_team",
		Payload: map[string]interface{}{
			"team_name": body.Team,
		},
		RespCh: respCh,
		Ctx:    ctx,
	}
	h.svc.EnqueueJob(job)

	res, err := waitJob(ctx, job.RespCh)
	if err != nil {
		writeError(w, http.StatusGatewayTimeout, "CANCELED", "request canceled")
		return
	}

	if res.Error != nil {
		h.log.Error("failed to deactivate team", "team_name", body.Team, "error", res.Error)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": res.Error.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func waitJob(ctx context.Context, ch <-chan service.JobResult) (service.JobResult, error) {
	select {
	case res := <-ch:
		return res, nil
	case <-ctx.Done():
		return service.JobResult{}, ctx.Err()
	}
}
