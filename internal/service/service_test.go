package service_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"PR-reviewer/internal/logger"
	"PR-reviewer/internal/models"
	"PR-reviewer/internal/service"
)

type dummyLogger struct{}

func (m *dummyLogger) Info(msg string, kv ...any)               {}
func (m *dummyLogger) Success(msg string, kv ...any)            {}
func (m *dummyLogger) Warn(msg string, kv ...any)               {}
func (m *dummyLogger) Error(msg string, kv ...any)              {}
func (m *dummyLogger) WithWorker(workerID string) logger.Logger { return m }

type mockRepo struct {
	InsertTeamFunc                 func(ctx context.Context, t models.Team) error
	GetTeamFunc                    func(ctx context.Context, name string) (models.Team, error)
	UpdateUserActiveFunc           func(ctx context.Context, userID string, active bool) (models.User, error)
	GetPRFunc                      func(ctx context.Context, prID string) (models.PullRequest, error)
	CreatePRFunc                   func(ctx context.Context, pr models.PullRequest) error
	MergePRFunc                    func(ctx context.Context, prID string, t time.Time) (models.PullRequest, error)
	AddReviewerFunc                func(ctx context.Context, prID, userID string) error
	CleanupInactiveReviewersFunc   func(ctx context.Context, prID string) error
	GetUserTeamFunc                func(ctx context.Context, userID string) (string, error)
	GetActiveTeamMembersExceptFunc func(ctx context.Context, teamName, exclude string) ([]string, error)
	GetUserFunc                    func(ctx context.Context, userID string) (models.User, error)
	ReplaceReviewerFunc            func(ctx context.Context, prID, oldUser, newUser string) (models.PullRequest, error)
	GetPRsByReviewerFunc           func(ctx context.Context, userID string) ([]models.PullRequestShort, error)
	SetTeamActiveFunc              func(ctx context.Context, teamName string, active bool) error
	GetReviewerStatsFunc           func(ctx context.Context) (map[string]int, error)
}

func (m *mockRepo) InsertTeam(ctx context.Context, t models.Team) error {
	if m.InsertTeamFunc != nil {
		return m.InsertTeamFunc(ctx, t)
	}
	return nil
}
func (m *mockRepo) GetTeam(ctx context.Context, name string) (models.Team, error) {
	if m.GetTeamFunc != nil {
		return m.GetTeamFunc(ctx, name)
	}
	return models.Team{}, nil
}
func (m *mockRepo) UpdateUserActive(ctx context.Context, userID string, active bool) (models.User, error) {
	if m.UpdateUserActiveFunc != nil {
		return m.UpdateUserActiveFunc(ctx, userID, active)
	}
	return models.User{}, nil
}
func (m *mockRepo) GetPR(ctx context.Context, prID string) (models.PullRequest, error) {
	if m.GetPRFunc != nil {
		return m.GetPRFunc(ctx, prID)
	}
	return models.PullRequest{}, nil
}
func (m *mockRepo) CreatePR(ctx context.Context, pr models.PullRequest) error {
	if m.CreatePRFunc != nil {
		return m.CreatePRFunc(ctx, pr)
	}
	return nil
}
func (m *mockRepo) MergePR(ctx context.Context, prID string, t time.Time) (models.PullRequest, error) {
	if m.MergePRFunc != nil {
		return m.MergePRFunc(ctx, prID, t)
	}
	return models.PullRequest{}, nil
}
func (m *mockRepo) AddReviewer(ctx context.Context, prID, userID string) (models.PullRequest, error) {
	return models.PullRequest{}, m.AddReviewerFunc(ctx, prID, userID)
}
func (m *mockRepo) CleanupInactiveReviewers(ctx context.Context, prID string) error {
	return m.CleanupInactiveReviewersFunc(ctx, prID)
}
func (m *mockRepo) GetUserTeam(ctx context.Context, userID string) (string, error) {
	if m.GetUserTeamFunc != nil {
		return m.GetUserTeamFunc(ctx, userID)
	}
	return "", nil
}
func (m *mockRepo) GetActiveTeamMembersExcept(ctx context.Context, teamName, exclude string) ([]string, error) {
	if m.GetActiveTeamMembersExceptFunc != nil {
		return m.GetActiveTeamMembersExceptFunc(ctx, teamName, exclude)
	}
	return nil, nil
}
func (m *mockRepo) GetUser(ctx context.Context, userID string) (models.User, error) {
	if m.GetUserFunc != nil {
		return m.GetUserFunc(ctx, userID)
	}
	return models.User{}, nil
}
func (m *mockRepo) ReplaceReviewer(ctx context.Context, prID, oldUser, newUser string) (models.PullRequest, error) {
	if m.ReplaceReviewerFunc != nil {
		return m.ReplaceReviewerFunc(ctx, prID, oldUser, newUser)
	}
	return models.PullRequest{}, nil
}
func (m *mockRepo) GetPRsByReviewer(ctx context.Context, userID string) ([]models.PullRequestShort, error) {
	if m.GetPRsByReviewerFunc != nil {
		return m.GetPRsByReviewerFunc(ctx, userID)
	}
	return nil, nil
}
func (m *mockRepo) SetTeamActive(ctx context.Context, teamName string, active bool) error {
	if m.SetTeamActiveFunc != nil {
		return m.SetTeamActiveFunc(ctx, teamName, active)
	}
	return nil
}
func (m *mockRepo) GetReviewerStats(ctx context.Context) (map[string]int, error) {
	if m.GetReviewerStatsFunc != nil {
		return m.GetReviewerStatsFunc(ctx)
	}
	return nil, nil
}

func newTestService(mockR *mockRepo) *service.PRService {
	mockL := &dummyLogger{}
	return service.NewService(mockR, mockL)
}

func TestAddTeam(t *testing.T) {
	mockR := &mockRepo{}
	svc := newTestService(mockR)

	mockR.InsertTeamFunc = func(ctx context.Context, t models.Team) error {
		if t.TeamName != "alpha" {
		}
		return nil
	}

	err := svc.AddTeam(context.Background(), models.Team{TeamName: "alpha"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mockR.InsertTeamFunc = func(ctx context.Context, t models.Team) error {
		return errors.New("db fail")
	}
	err = svc.AddTeam(context.Background(), models.Team{TeamName: "fail"})
	if err == nil || err.Error() != "db fail" {
		t.Fatalf("expected db fail error, got %v", err)
	}
}

func TestGetTeam(t *testing.T) {
	mockR := &mockRepo{}
	svc := newTestService(mockR)

	mockR.GetTeamFunc = func(ctx context.Context, name string) (models.Team, error) {
		if name == "exists" {
			return models.Team{TeamName: "exists"}, nil
		}
		return models.Team{}, errors.New("not found")
	}

	team, err := svc.GetTeam(context.Background(), "exists")
	if err != nil || team.TeamName != "exists" {
		t.Fatalf("expected team exists, got %v, err=%v", team, err)
	}

	_, err = svc.GetTeam(context.Background(), "missing")
	if err != service.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSetUserActive(t *testing.T) {
	mockR := &mockRepo{}
	svc := newTestService(mockR)

	mockR.UpdateUserActiveFunc = func(ctx context.Context, uid string, active bool) (models.User, error) {
		if uid == "u1" {
			return models.User{UserID: uid, IsActive: active}, nil
		}
		return models.User{}, errors.New("not found")
	}

	u, err := svc.SetUserActive(context.Background(), "u1", true)
	if err != nil || !u.IsActive {
		t.Fatalf("expected user active, got %v, err=%v", u, err)
	}

	_, err = svc.SetUserActive(context.Background(), "uX", true)
	if err != service.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCreatePR(t *testing.T) {
	mockR := &mockRepo{}
	svc := newTestService(mockR)

	pr := models.PullRequest{
		PullRequestID:   "pr1",
		PullRequestName: "Test PR",
		AuthorID:        "u1",
	}

	call := 0
	mockR.GetPRFunc = func(ctx context.Context, prID string) (models.PullRequest, error) {
		if call == 0 {
			call++
			return models.PullRequest{}, errors.New("not found")
		}
		return pr, nil
	}

	mockR.GetUserTeamFunc = func(ctx context.Context, userID string) (string, error) {
		return "teamA", nil
	}
	mockR.GetActiveTeamMembersExceptFunc = func(ctx context.Context, team, exclude string) ([]string, error) {
		return []string{"u2"}, nil
	}
	mockR.GetUserFunc = func(ctx context.Context, userID string) (models.User, error) {
		return models.User{UserID: userID, IsActive: true}, nil
	}
	mockR.CreatePRFunc = func(ctx context.Context, pr models.PullRequest) error { return nil }

	created, err := svc.CreatePR(context.Background(), pr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.PullRequestID != pr.PullRequestID {
		t.Fatalf("expected PRID '%s', got '%s'", pr.PullRequestID, created.PullRequestID)
	}
}

func TestMergePR(t *testing.T) {
	mockR := &mockRepo{}
	svc := newTestService(mockR)

	mockR.GetPRFunc = func(ctx context.Context, prID string) (models.PullRequest, error) {
		return models.PullRequest{PullRequestID: prID, Status: "OPEN"}, nil
	}
	mockR.MergePRFunc = func(ctx context.Context, prID string, t time.Time) (models.PullRequest, error) {
		return models.PullRequest{PullRequestID: prID, Status: "MERGED"}, nil
	}

	pr, err := svc.MergePR(context.Background(), "pr1")
	if err != nil || pr.Status != "MERGED" {
		t.Fatalf("expected merged PR, got %v, err=%v", pr, err)
	}
}

func TestReassign(t *testing.T) {
	mockR := &mockRepo{}
	svc := newTestService(mockR)

	pr := models.PullRequest{
		PullRequestID: "pr1",
		Assigned:      []models.PRReviewer{{UserID: "u1", IsActive: true}},
		Status:        "OPEN",
	}

	mockR.GetPRFunc = func(ctx context.Context, prID string) (models.PullRequest, error) {
		return pr, nil
	}
	mockR.GetUserFunc = func(ctx context.Context, uid string) (models.User, error) {
		return models.User{UserID: uid, IsActive: true}, nil
	}
	mockR.GetUserTeamFunc = func(ctx context.Context, uid string) (string, error) {
		return "teamA", nil
	}
	mockR.GetActiveTeamMembersExceptFunc = func(ctx context.Context, team, exclude string) ([]string, error) {
		return []string{"u2"}, nil
	}
	mockR.ReplaceReviewerFunc = func(ctx context.Context, prID, oldUser, newUser string) (models.PullRequest, error) {
		pr.Assigned = []models.PRReviewer{{UserID: newUser, IsActive: true}}
		return pr, nil
	}

	newPR, newUID, err := svc.Reassign(context.Background(), "pr1", "u1")
	if err != nil || newUID != "u2" || newPR.Assigned[0].UserID != "u2" {
		t.Fatalf("expected reassigned to u2, got %v, newUID=%s, err=%v", newPR, newUID, err)
	}
}

func TestGetStats(t *testing.T) {
	mockR := &mockRepo{}
	svc := newTestService(mockR)

	mockR.GetReviewerStatsFunc = func(ctx context.Context) (map[string]int, error) {
		return map[string]int{"u1": 10}, nil
	}

	stats, err := svc.GetStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats["u1"] != 10 {
		t.Fatalf("expected 10, got %d", stats["u1"])
	}
}

func TestEnqueueJob_Success(t *testing.T) {
	mockR := &mockRepo{}
	svc := newTestService(mockR)

	done := make(chan bool)
	job := service.Job{
		Type: "get_team",
		Payload: map[string]interface{}{
			"team": "alpha",
		},
		RespCh: make(chan service.JobResult, 1),
	}

	mockR.GetTeamFunc = func(ctx context.Context, name string) (models.Team, error) {
		return models.Team{TeamName: name}, nil
	}

	svc.EnqueueJob(job)

	res := <-job.RespCh
	if res.Error != nil {
		t.Fatalf("expected no error, got %v", res.Error)
	}
	if team, ok := res.Data.(models.Team); !ok || team.TeamName != "alpha" {
		t.Fatalf("expected team 'alpha', got %v", res.Data)
	}
	close(done)
	<-done
}

func TestEnqueueJob_Stopped(t *testing.T) {
	mockR := &mockRepo{}
	svc := newTestService(mockR)

	respCh := make(chan service.JobResult, 1)
	job := service.Job{
		Type:    "get_team",
		Payload: map[string]interface{}{"team": "alpha"},
		RespCh:  respCh,
	}

	svc.StopWorkers()
	svc.EnqueueJob(job)

	select {
	case res := <-respCh:
		if res.Error != context.Canceled {
			t.Fatalf("expected context.Canceled, got %v", res.Error)
		}
	default:
		t.Fatal("expected job result to be returned immediately after stop")
	}
}

func TestFullQueue(t *testing.T) {
	mockR := &mockRepo{}
	svc := newTestService(mockR)

	block := make(chan struct{})

	mockR.GetTeamFunc = func(ctx context.Context, name string) (models.Team, error) {
		<-block
		return models.Team{TeamName: name}, nil
	}

	const maxAttempts = 5000
	found := false

	for i := 0; i < maxAttempts; i++ {
		j := service.Job{
			Type: "get_team",
			Payload: map[string]interface{}{
				"team": "team-" + strconv.Itoa(i),
			},
			RespCh: make(chan service.JobResult, 1),
		}
		svc.EnqueueJob(j)

		select {
		case res := <-j.RespCh:
			if res.Error == service.ErrJobQueueFull {
				found = true
				close(block)
				goto DONE
			}
		default:
		}
	}

DONE:
	if !found {
		close(block)
		t.Fatalf("expected at least one task to be dropped with ErrJobQueueFull (tried %d enqueues)", maxAttempts)
	}
}
