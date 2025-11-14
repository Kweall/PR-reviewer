package service

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"PR-reviewer/internal/logger"
	"PR-reviewer/internal/models"
	"PR-reviewer/internal/repo"
)

type JobResult struct {
	Data  interface{}
	Error error
}

type Job struct {
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
		jobs:    make(chan Job, 200),
		stopped: make(chan struct{}),
	}

	// 3 воркера
	for i := 1; i <= 3; i++ {
		s.wg.Add(1)
		go s.workerLoop(i)
	}

	s.log.Info("service initialized and workers started")
	return s
}

func (s *PRService) StopWorkers() {
	close(s.jobs)
	s.wg.Wait()
	close(s.stopped)
	s.log.Info("all workers stopped")
}

func (s *PRService) workerLoop(id int) {
	defer s.wg.Done()
	workerLog := s.log.WithWorker("worker-" + strconv.Itoa(id))

	for job := range s.jobs {
		start := time.Now()

		var res JobResult

		kvs := make([]any, 0, 10)

		switch job.Type {

		case "create_pr":
			pr := job.Payload["pr"].(models.PullRequest)
			data, err := s.CreatePR(pr)
			res = JobResult{Data: data, Error: err}
			if err == nil {
				kvs = append(kvs, "pr", data.PullRequestID, "assigned", data.Assigned)
			}

		case "merge_pr":
			id := job.Payload["pr_id"].(string)
			data, err := s.MergePR(id)
			res = JobResult{Data: data, Error: err}
			if err == nil {
				kvs = append(kvs, "pr", id)
			}

		case "reassign_pr":
			prID := job.Payload["pr_id"].(string)
			oldUser := job.Payload["old_user"].(string)
			data, newUID, err := s.Reassign(prID, oldUser)
			res = JobResult{Data: map[string]interface{}{"pr": data, "new_user": newUID}, Error: err}
			if err == nil {
				kvs = append(kvs, "pr", prID, "old_user", oldUser, "new_user", newUID)
			} else {
				kvs = append(kvs, "pr", prID, "old_user", oldUser)
			}

		case "get_team":
			name := job.Payload["team"].(string)
			data, err := s.GetTeam(name)
			res = JobResult{Data: data, Error: err}
			if err == nil {
				kvs = append(kvs, "team", name, "members", len(data.Members))
			} else {
				kvs = append(kvs, "team", name)
			}

		case "set_user_active":
			uid := job.Payload["uid"].(string)
			active := job.Payload["active"].(bool)
			data, err := s.SetUserActive(uid, active)
			res = JobResult{Data: data, Error: err}
			if err == nil {
				kvs = append(kvs, "user", uid, "active", active)
			} else {
				kvs = append(kvs, "user", uid, "active", active)
			}

		case "get_reviews":
			uid := job.Payload["uid"].(string)
			data, err := s.GetPRsByReviewer(uid)
			res = JobResult{Data: data, Error: err}
			if err == nil {
				kvs = append(kvs, "user", uid, "count", len(data))
			} else {
				kvs = append(kvs, "user", uid)
			}

		default:
			res = JobResult{Data: nil, Error: ErrUnknownJobType}
		}

		duration := time.Since(start)
		ms := float64(duration.Nanoseconds()) / 1e6
		durationStr := fmt.Sprintf("%.1fms", ms)

		if res.Error == nil {
			args := make([]any, 0, 2+len(kvs))
			args = append(args, "duration", durationStr)
			args = append(args, kvs...)
			workerLog.Success(job.Type+" succeeded", args...)
		} else {
			args := kvs
			args = append([]any{}, args...)
			workerLog.Error(res.Error.Error(), args...)
		}

		if job.RespCh != nil {
			select {
			case job.RespCh <- res:
			default:
				workerLog.Warn("response channel blocked, dropping result", "type", job.Type)
			}
		}
	}
}

func (s *PRService) EnqueueJob(job Job) {
	select {
	case s.jobs <- job:
		// s.log.Info("job enqueued", "type", job.Type)
	default:
		// queue full
		s.log.Warn("job queue full, dropping job", "type", job.Type)
		// notify caller if they provided RespCh
		if job.RespCh != nil {
			select {
			case job.RespCh <- JobResult{Error: ErrJobQueueFull}:
			default:
			}
		}
	}
}

// ------------------- Бизнес-логика -------------------

// AddTeam - create/update users and team
func (s *PRService) AddTeam(team models.Team) error {
	if err := s.repo.InsertTeam(team); err != nil {
		s.log.Error("failed to add team", "team", team.TeamName, "error", err)
		return err
	}
	s.log.Info("team added", "team", team.TeamName)
	return nil
}

func (s *PRService) GetTeam(name string) (models.Team, error) {
	t, err := s.repo.GetTeam(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return models.Team{}, ErrNotFound
		}
		s.log.Error("failed to get team", "team", name, "error", err)
		return models.Team{}, err
	}
	return t, nil
}

func (s *PRService) SetUserActive(userID string, active bool) (models.User, error) {
	u, err := s.repo.UpdateUserActive(userID, active)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return models.User{}, ErrNotFound
		}
		s.log.Error("failed to set user active", "user", userID, "error", err)
		return models.User{}, err
	}
	return u, nil
}

func (s *PRService) CreatePR(pr models.PullRequest) (models.PullRequest, error) {
	if _, err := s.repo.GetPR(pr.PullRequestID); err == nil {
		return models.PullRequest{}, ErrPRExists
	} else if !strings.Contains(err.Error(), "not found") {
		s.log.Error("failed to check PR existence", "pr", pr.PullRequestID, "error", err)
		return models.PullRequest{}, err
	}

	team, err := s.repo.GetUserTeam(pr.AuthorID)
	if err != nil {
		return models.PullRequest{}, ErrNotFound
	}

	candidates, err := s.repo.GetActiveTeamMembersExcept(team, pr.AuthorID)
	if err != nil {
		s.log.Error("failed to get active candidates", "author", pr.AuthorID, "error", err)
		return models.PullRequest{}, err
	}

	rand.Seed(time.Now().UnixNano())
	selected := []string{}
	if len(candidates) > 0 {
		perm := rand.Perm(len(candidates))
		for i := 0; i < len(perm) && len(selected) < 2; i++ {
			selected = append(selected, candidates[perm[i]])
		}
	}

	pr.Assigned = selected
	pr.Status = "OPEN"
	pr.CreatedAt = time.Now().UTC()

	if err := s.repo.CreatePR(pr); err != nil {
		s.log.Error("failed to create PR", "pr", pr.PullRequestID, "error", err)
		return models.PullRequest{}, err
	}

	created, err := s.repo.GetPR(pr.PullRequestID)
	if err != nil {
		s.log.Error("failed to fetch created PR", "pr", pr.PullRequestID, "error", err)
		return models.PullRequest{}, err
	}

	return created, nil
}

func (s *PRService) MergePR(prID string) (models.PullRequest, error) {
	pr, err := s.repo.GetPR(prID)
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
	merged, err := s.repo.MergePR(prID, t)
	if err != nil {
		s.log.Error("failed to merge PR", "pr", prID, "error", err)
		return models.PullRequest{}, err
	}

	return merged, nil
}

func (s *PRService) Reassign(prID, oldUser string) (models.PullRequest, string, error) {
	pr, err := s.repo.GetPR(prID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return models.PullRequest{}, "", ErrNotFound
		}
		s.log.Error("failed to fetch PR for reassign", "pr", prID, "error", err)
		return models.PullRequest{}, "", err
	}

	if pr.Status == "MERGED" {
		return models.PullRequest{}, "", ErrPRMerged
	}

	assigned := false
	for _, u := range pr.Assigned {
		if u == oldUser {
			assigned = true
			break
		}
	}
	if !assigned {
		return models.PullRequest{}, "", ErrNotAssigned
	}

	team, err := s.repo.GetUserTeam(oldUser)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return models.PullRequest{}, "", ErrNotFound
		}
		s.log.Error("failed to get user's team", "user", oldUser, "error", err)
		return models.PullRequest{}, "", err
	}

	cands, err := s.repo.GetActiveTeamMembersExcept(team, "")
	if err != nil {
		s.log.Error("failed to get active candidates for reassign", "team", team, "error", err)
		return models.PullRequest{}, "", err
	}

	avail := []string{}
	assignedSet := map[string]struct{}{}
	for _, a := range pr.Assigned {
		assignedSet[a] = struct{}{}
	}
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

	rand.Seed(time.Now().UnixNano())
	newUID := avail[rand.Intn(len(avail))]

	pr, err = s.repo.ReplaceReviewer(prID, oldUser, newUID)
	if err != nil {
		s.log.Error("failed to replace reviewer", "pr", prID, "oldUser", oldUser, "error", err)
		return models.PullRequest{}, "", err
	}

	return pr, newUID, nil
}

func (s *PRService) GetPRsByReviewer(userID string) ([]models.PullRequestShort, error) {
	return s.repo.GetPRsByReviewer(userID)
}
