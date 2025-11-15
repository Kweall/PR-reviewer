package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"PR-reviewer/internal/logger"
	"PR-reviewer/internal/models"
	"PR-reviewer/internal/repo"
)

const (
	numWorkers   = 3
	jobQueueSize = 200
	maxReviewers = 2
	kvsInitCap   = 10
)

type JobResult struct {
	Data  interface{}
	Error error
}

type Job struct {
	Ctx     context.Context
	Type    string
	Payload map[string]interface{}
	RespCh  chan JobResult
}

type PRService struct {
	repo    repo.Repo
	log     logger.Logger
	jobs    chan Job
	wg      sync.WaitGroup
	stopped chan struct{}
}

func NewService(r repo.Repo, l logger.Logger) *PRService {
	s := &PRService{
		repo:    r,
		log:     l,
		jobs:    make(chan Job, jobQueueSize),
		stopped: make(chan struct{}),
	}

	for i := 1; i <= numWorkers; i++ {
		s.wg.Add(1)
		go s.workerLoop(i)
	}

	s.log.Info("service initialized and workers started")
	return s
}

func (s *PRService) StopWorkers() {
	close(s.stopped)
	close(s.jobs)
	s.wg.Wait()
	s.log.Info("all workers stopped")
}

func (s *PRService) workerLoop(id int) {
	defer s.wg.Done()
	workerLog := s.log.WithWorker("worker-" + strconv.Itoa(id))

	for {
		select {
		case <-s.stopped:
			workerLog.Info("stop signal received, worker exiting")
			return

		case job, ok := <-s.jobs:
			if !ok {
				workerLog.Info("jobs channel closed, worker exiting")
				return
			}

			ctx := job.Ctx
			if ctx == nil {
				ctx = context.Background()
			}

			start := time.Now()

			res, kvs := s.handleJob(ctx, job, workerLog)

			duration := time.Since(start)
			ms := float64(duration.Nanoseconds()) / 1e6
			durationStr := fmt.Sprintf("%.1fms", ms)

			s.logJobResult(workerLog, job.Type, durationStr, kvs, res.Error)

			if job.RespCh != nil {
				select {
				case job.RespCh <- res:
				default:
					workerLog.Warn("response channel blocked, dropping result", "type", job.Type)
				}
			}
		}
	}
}

func (s *PRService) handleJob(ctx context.Context, job Job, workerLog logger.Logger) (JobResult, []any) {
	kvs := make([]any, 0, kvsInitCap)

	select {
	case <-ctx.Done():
		return JobResult{Data: nil, Error: ctx.Err()}, kvs
	default:
	}

	switch job.Type {
	case "create_pr":
		v, ok := job.Payload["pr"].(models.PullRequest)
		if !ok {
			return JobResult{Data: nil, Error: ErrUnknownJobType}, kvs
		}
		created, err := s.CreatePR(ctx, v)
		if err == nil {
			kvs = append(kvs, "pr", created.PullRequestID, "assigned", created.Assigned)
		}
		return JobResult{Data: created, Error: err}, kvs

	case "merge_pr":
		v, ok := job.Payload["pr_id"].(string)
		if !ok {
			return JobResult{Data: nil, Error: ErrUnknownJobType}, kvs
		}
		merged, err := s.MergePR(ctx, v)
		if err == nil {
			kvs = append(kvs, "pr", v)
		}
		return JobResult{Data: merged, Error: err}, kvs

	case "reassign_pr":
		prID, ok1 := job.Payload["pr_id"].(string)
		oldUser, ok2 := job.Payload["old_user"].(string)
		if !ok1 || !ok2 {
			return JobResult{Data: nil, Error: ErrUnknownJobType}, kvs
		}
		pr, newUID, err := s.Reassign(ctx, prID, oldUser)
		if err == nil {
			kvs = append(kvs, "pr", prID, "old_user", oldUser, "new_user", newUID)
			return JobResult{Data: map[string]interface{}{"pr": pr, "new_user": newUID}, Error: nil}, kvs
		}
		kvs = append(kvs, "pr", prID, "old_user", oldUser)
		return JobResult{Data: map[string]interface{}{"pr": pr, "new_user": newUID}, Error: err}, kvs

	case "get_team":
		name, ok := job.Payload["team"].(string)
		if !ok {
			return JobResult{Data: nil, Error: ErrUnknownJobType}, kvs
		}
		t, err := s.GetTeam(ctx, name)
		if err == nil {
			kvs = append(kvs, "team", name, "members", len(t.Members))
		} else {
			kvs = append(kvs, "team", name)
		}
		return JobResult{Data: t, Error: err}, kvs

	case "set_user_active":
		uid, ok1 := job.Payload["uid"].(string)
		active, ok2 := job.Payload["active"].(bool)
		if !ok1 || !ok2 {
			return JobResult{Data: nil, Error: ErrUnknownJobType}, kvs
		}
		u, err := s.SetUserActive(ctx, uid, active)
		kvs = append(kvs, "user", uid, "active", active)
		return JobResult{Data: u, Error: err}, kvs

	case "get_reviews":
		uid, ok := job.Payload["uid"].(string)
		if !ok {
			return JobResult{Data: nil, Error: ErrUnknownJobType}, kvs
		}
		data, err := s.GetPRsByReviewer(ctx, uid)
		if err == nil {
			kvs = append(kvs, "user", uid, "count", len(data))
		} else {
			kvs = append(kvs, "user", uid)
		}
		return JobResult{Data: data, Error: err}, kvs

	case "deactivate_team":
		teamName, ok := job.Payload["team_name"].(string)
		if !ok {
			return JobResult{Data: nil, Error: ErrUnknownJobType}, kvs
		}
		err := s.DeactivateTeam(ctx, teamName)
		if err == nil {
			kvs = append(kvs, "team", teamName)
			return JobResult{Data: map[string]string{"team": teamName}, Error: nil}, kvs
		}
		kvs = append(kvs, "team", teamName)
		return JobResult{Data: map[string]string{"team": teamName}, Error: err}, kvs

	default:
		return JobResult{Data: nil, Error: ErrUnknownJobType}, kvs
	}
}

func (s *PRService) logJobResult(workerLog logger.Logger, jobType, durationStr string, kvs []any, err error) {
	if err == nil {
		args := append([]any{"duration", durationStr}, kvs...)
		workerLog.Success(jobType+" succeeded", args...)
		return
	}
	workerLog.Error(err.Error(), kvs...)
}

func (s *PRService) EnqueueJob(job Job) {
	select {
	case <-s.stopped:
		if job.RespCh != nil {
			select {
			case job.RespCh <- JobResult{Error: context.Canceled}:
			default:
			}
		}
		return
	default:
	}

	select {
	case s.jobs <- job:
	default:
		s.log.Warn("job queue full, dropping job", "type", job.Type)
		if job.RespCh != nil {
			select {
			case job.RespCh <- JobResult{Error: ErrJobQueueFull}:
			default:
			}
		}
	}
}

func (s *PRService) AddTeam(ctx context.Context, team models.Team) error {
	if err := validateTeam(team); err != nil {
		return err
	}
	if err := s.repo.InsertTeam(ctx, team); err != nil {
		s.log.Error("failed to add team", "team", team.TeamName, "error", err)
		return err
	}
	s.log.Success("team added", "team", team.TeamName)
	return nil
}

func (s *PRService) GetTeam(ctx context.Context, name string) (models.Team, error) {
	if err := validateTeamName(name); err != nil {
		return models.Team{}, err
	}
	t, err := s.repo.GetTeam(ctx, name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return models.Team{}, ErrNotFound
		}
		s.log.Error("failed to get team", "team", name, "error", err)
		return models.Team{}, err
	}
	return t, nil
}

func (s *PRService) SetUserActive(ctx context.Context, userID string, active bool) (models.User, error) {
	if err := validateUserID(userID); err != nil {
		return models.User{}, err
	}
	u, err := s.repo.UpdateUserActive(ctx, userID, active)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return models.User{}, ErrNotFound
		}
		s.log.Error("failed to set user active", "user", userID, "error", err)
		return models.User{}, err
	}
	return u, nil
}

func (s *PRService) CreatePR(ctx context.Context, pullRequest models.PullRequest) (models.PullRequest, error) {
	if err := validatePullRequest(pullRequest); err != nil {
		return models.PullRequest{}, err
	}
	if _, err := s.repo.GetPR(ctx, pullRequest.PullRequestID); err == nil {
		return models.PullRequest{}, ErrPRExists
	} else if !strings.Contains(err.Error(), "not found") {
		s.log.Error("failed to check PR existence", "pr", pullRequest.PullRequestID, "error", err)
		return models.PullRequest{}, err
	}

	teamName, err := s.repo.GetUserTeam(ctx, pullRequest.AuthorID)
	if err != nil {
		return models.PullRequest{}, ErrNotFound
	}

	candidateIDs, err := s.repo.GetActiveTeamMembersExcept(ctx, teamName, pullRequest.AuthorID)
	if err != nil {
		s.log.Error("failed to get active candidates", "author", pullRequest.AuthorID, "error", err)
		return models.PullRequest{}, err
	}

	selected := []models.PRReviewer{}
	if len(candidateIDs) > 0 {
		for len(selected) < maxReviewers && len(candidateIDs) > 0 {

			select {
			case <-ctx.Done():
				return models.PullRequest{}, ctx.Err()
			default:
			}

			idx, err := cryptoRandInt(len(candidateIDs))
			if err != nil {
				continue
			}
			userID := candidateIDs[idx]

			user, err := s.repo.GetUser(ctx, userID)
			if err != nil {
				candidateIDs = append(candidateIDs[:idx], candidateIDs[idx+1:]...)
				continue
			}
			if !user.IsActive {
				candidateIDs = append(candidateIDs[:idx], candidateIDs[idx+1:]...)
				continue
			}

			selected = append(selected, models.PRReviewer{
				UserID:   user.UserID,
				Username: user.Username,
				IsActive: user.IsActive,
			})

			candidateIDs = append(candidateIDs[:idx], candidateIDs[idx+1:]...)
		}
	}

	pullRequest.Assigned = selected
	pullRequest.NeedMoreReviewers = len(selected) < maxReviewers
	pullRequest.Status = "OPEN"
	pullRequest.CreatedAt = time.Now().UTC()

	if err := s.repo.CreatePR(ctx, pullRequest); err != nil {
		s.log.Error("failed to create PR", "pr", pullRequest.PullRequestID, "error", err)
		return models.PullRequest{}, err
	}

	created, err := s.repo.GetPR(ctx, pullRequest.PullRequestID)
	if err != nil {
		s.log.Error("failed to fetch created PR", "pr", pullRequest.PullRequestID, "error", err)
		return models.PullRequest{}, err
	}

	return created, nil
}

func (s *PRService) MergePR(ctx context.Context, prID string) (models.PullRequest, error) {
	pr, err := s.repo.GetPR(ctx, prID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return models.PullRequest{}, ErrNotFound
		}
		s.log.Error("failed to fetch PR for merge", "pr", prID, "error", err)
		return models.PullRequest{}, err
	}

	if pr.Status == "MERGED" {
		return pr, nil
	}

	t := time.Now().UTC()
	merged, err := s.repo.MergePR(ctx, prID, t)
	if err != nil {
		s.log.Error("failed to merge PR", "pr", prID, "error", err)
		return models.PullRequest{}, err
	}

	return merged, nil
}

func (s *PRService) Reassign(ctx context.Context, prID, oldUser string) (models.PullRequest, string, error) {
	pr, err := s.repo.GetPR(ctx, prID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return models.PullRequest{}, "", ErrNotFound
		}
		s.log.Error("failed to fetch PR for reassign", "pr", prID, "error", err)
		return models.PullRequest{}, "", err
	}

	for i, r := range pr.Assigned {
		usr, err := s.repo.GetUser(ctx, r.UserID)
		if err == nil {
			pr.Assigned[i].IsActive = usr.IsActive
		}
	}

	if pr.Status == "MERGED" {
		return models.PullRequest{}, "", ErrPRMerged
	}

	assigned := false
	for _, r := range pr.Assigned {
		if r.UserID == oldUser {
			assigned = true
			break
		}
	}
	if !assigned {
		return models.PullRequest{}, "", ErrNotAssigned
	}

	u, err := s.repo.GetUser(ctx, oldUser)
	if err != nil {
		return models.PullRequest{}, "", ErrNotFound
	}
	if !u.IsActive {
		return models.PullRequest{}, "", ErrUserInactive
	}

	teamName, err := s.repo.GetUserTeam(ctx, oldUser)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return models.PullRequest{}, "", ErrNotFound
		}
		s.log.Error("failed to get user's team", "user", oldUser, "error", err)
		return models.PullRequest{}, "", err
	}

	cands, err := s.repo.GetActiveTeamMembersExcept(ctx, teamName, "")
	if err != nil {
		s.log.Error("failed to get active candidates for reassign", "team", teamName, "error", err)
		return models.PullRequest{}, "", err
	}

	assignedSet := map[string]struct{}{}
	for _, a := range pr.Assigned {
		assignedSet[a.UserID] = struct{}{}
	}

	avail := make([]string, 0, len(cands))
	for _, c := range cands {
		if c == oldUser {
			continue
		}
		if _, ok := assignedSet[c]; ok {
			continue
		}
		avail = append(avail, c)
	}

	if len(avail) == 0 {
		return models.PullRequest{}, "", ErrNoCandidate
	}

	select {
	case <-ctx.Done():
		return models.PullRequest{}, "", ctx.Err()
	default:
	}

	idx, err := cryptoRandInt(len(avail))
	if err != nil {
		return models.PullRequest{}, "", err
	}
	newUID := avail[idx]

	nu, err := s.repo.GetUser(ctx, newUID)
	if err != nil || !nu.IsActive {
		return models.PullRequest{}, "", ErrNoCandidate
	}

	pr, err = s.repo.ReplaceReviewer(ctx, prID, oldUser, newUID)
	if err != nil {
		s.log.Error("failed to replace reviewer", "pr", prID, "oldUser", oldUser, "error", err)
		return models.PullRequest{}, "", err
	}

	pr.NeedMoreReviewers = len(pr.Assigned) < maxReviewers

	return pr, newUID, nil
}

func (s *PRService) GetPRsByReviewer(ctx context.Context, userID string) ([]models.PullRequestShort, error) {
	return s.repo.GetPRsByReviewer(ctx, userID)
}

func (s *PRService) DeactivateTeam(ctx context.Context, teamName string) error {
	team, err := s.GetTeam(ctx, teamName)
	if err != nil {
		return err
	}

	if err := s.repo.SetTeamActive(ctx, teamName, false); err != nil {
		s.log.Error("failed to deactivate team", "team", teamName, "error", err)
		return err
	}

	for _, member := range team.Members {

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		prs, err := s.repo.GetPRsByReviewer(ctx, member.UserID)
		if err != nil {
			s.log.Error("failed to get PRs for member", "user", member.UserID, "error", err)
			continue
		}

		for _, prShort := range prs {

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			pr, err := s.repo.GetPR(ctx, prShort.PullRequestID)
			if err != nil {
				s.log.Error("failed to get full PR", "pr", prShort.PullRequestID, "error", err)
				continue
			}

			if pr.Status != "OPEN" {
				continue
			}

			updated := false
			for _, rev := range pr.Assigned {
				user, err := s.repo.GetUser(ctx, rev.UserID)
				if err != nil {
					s.log.Warn("could not fetch user while deactivating team", "user", rev.UserID, "pr", pr.PullRequestID, "error", err)
					continue
				}
				if !user.IsActive {
					newUID, err := s.reassignReviewer(ctx, pr.PullRequestID, rev.UserID, teamName)
					if err != nil {
						s.log.Warn("no replacement found for inactive reviewer", "pr", pr.PullRequestID, "user", rev.UserID)
						continue
					}
					s.log.Info("reviewer replaced", "pr", pr.PullRequestID, "old_user", rev.UserID, "new_user", newUID)
					updated = true
				}
			}
			if updated {
				pr.NeedMoreReviewers = len(pr.Assigned) < maxReviewers
			}
		}
	}

	s.log.Success("team deactivated", "team", teamName)
	return nil
}

func (s *PRService) reassignReviewer(ctx context.Context, prID, oldUID, teamName string) (string, error) {
	cands, err := s.repo.GetActiveTeamMembersExcept(ctx, teamName, "")
	if err != nil {
		return "", err
	}

	pr, err := s.repo.GetPR(ctx, prID)
	if err != nil {
		return "", err
	}

	assignedSet := map[string]struct{}{}
	for _, a := range pr.Assigned {
		assignedSet[a.UserID] = struct{}{}
	}

	avail := []string{}
	for _, c := range cands {
		if _, ok := assignedSet[c]; ok {
			continue
		}
		avail = append(avail, c)
	}
	if len(avail) == 0 {
		return "", ErrNoCandidate
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	idx, err := cryptoRandInt(len(avail))
	if err != nil {
		return "", err
	}
	newUID := avail[idx]

	_, err = s.repo.ReplaceReviewer(ctx, prID, oldUID, newUID)
	if err != nil {
		return "", err
	}
	return newUID, nil
}

func (s *PRService) GetStats(ctx context.Context) (map[string]int, error) {
	start := time.Now()
	stats, err := s.repo.GetReviewerStats(ctx)
	ms := float64(time.Since(start).Nanoseconds()) / 1e6
	durationStr := fmt.Sprintf("%.1fms", ms)

	if err == nil {
		workerLog := s.log.WithWorker("worker-stats")
		workerLog.Success("get_stats succeeded", "duration", durationStr, "stats", stats)
	}
	return stats, err
}

func cryptoRandInt(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("invalid n for cryptoRandInt: %d", n)
	}
	bigN := big.NewInt(int64(n))
	r, err := rand.Int(rand.Reader, bigN)
	if err != nil {
		return 0, fmt.Errorf("crypto rand failed: %w", err)
	}
	return int(r.Int64()), nil
}
