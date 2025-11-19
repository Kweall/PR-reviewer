package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"PR-reviewer/internal/logger"
	"PR-reviewer/internal/mocks"
	"PR-reviewer/internal/models"
	"PR-reviewer/internal/service"
)

type dummyLogger struct{}

func (m *dummyLogger) Info(msg string, kv ...any)               {}
func (m *dummyLogger) Success(msg string, kv ...any)            {}
func (m *dummyLogger) Warn(msg string, kv ...any)               {}
func (m *dummyLogger) Error(msg string, kv ...any)              {}
func (m *dummyLogger) WithWorker(workerID string) logger.Logger { return m }

func newTestHandler(t *testing.T, svc *mocks.ServiceMock) *Handler {
	t.Helper()
	return NewHandler(svc, &dummyLogger{})
}

func TestAddTeam(t *testing.T) {
	teamJSON := `{"team_name": "alpha", "members": []}`
	expectedTeam := models.Team{TeamName: "alpha"}

	svcMock := mocks.NewServiceMock(t)
	svcMock.AddTeamMock.Set(func(ctx context.Context, team models.Team) error {
		if team.TeamName != expectedTeam.TeamName {
			t.Errorf("expected team name %q, got %q", expectedTeam.TeamName, team.TeamName)
		}
		return nil
	})

	handler := newTestHandler(t, svcMock)

	req := httptest.NewRequest(http.MethodPost, "/team", strings.NewReader(teamJSON))
	rr := httptest.NewRecorder()

	handler.AddTeam(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"team_name":"alpha"`) {
		t.Fatalf("expected body to contain team_name, got %s", rr.Body.String())
	}
}

func TestSetIsActive(t *testing.T) {
	tests := []struct {
		name           string
		inputJSON      string
		mockJobResult  service.JobResult
		expectedStatus int
		expectedBody   string
		contextTimeout time.Duration
	}{
		{
			name:      "Success",
			inputJSON: `{"user_id": "u1", "is_active": false}`,
			mockJobResult: service.JobResult{
				Data:  models.User{UserID: "u1", IsActive: false},
				Error: nil,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"user_id":"u1"`,
		},
		{
			name:           "Validation error",
			inputJSON:      `{"is_active": true}`,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `user_id required`,
		},
		{
			name:      "Service error",
			inputJSON: `{"user_id": "not-found", "is_active": true}`,
			mockJobResult: service.JobResult{
				Error: service.ErrNotFound,
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `user not found`,
		},
		{
			name:           "Timeout",
			inputJSON:      `{"user_id": "u1", "is_active": true}`,
			mockJobResult:  service.JobResult{},
			expectedStatus: http.StatusGatewayTimeout,
			expectedBody:   `request canceled`,
			contextTimeout: 5 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcMock := mocks.NewServiceMock(t)

			if tt.name != "Validation error" {
				svcMock.EnqueueJobMock.Set(func(job service.Job) {
					if tt.name != "Timeout" {
						job.RespCh <- tt.mockJobResult
					}
				})
			}

			handler := newTestHandler(t, svcMock)
			req := httptest.NewRequest(http.MethodPost, "/user/active", strings.NewReader(tt.inputJSON))
			rr := httptest.NewRecorder()

			if tt.contextTimeout > 0 {
				ctx, cancel := context.WithTimeout(req.Context(), tt.contextTimeout)
				defer cancel()
				req = req.WithContext(ctx)
			}

			handler.SetIsActive(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tt.expectedBody) {
				t.Errorf("expected body to contain '%s', got '%s'", tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestCreatePR(t *testing.T) {
	inputJSON := `{"pull_request_id":"pr-1","pull_request_name":"My PR","author_id":"u1"}`
	mockResult := service.JobResult{
		Data: models.PullRequest{
			PullRequestID:   "pr-1",
			PullRequestName: "My PR",
			AuthorID:        "u1",
			Status:          "OPEN",
		},
	}

	svcMock := mocks.NewServiceMock(t)
	svcMock.EnqueueJobMock.Set(func(job service.Job) {
		job.RespCh <- mockResult
	})

	handler := newTestHandler(t, svcMock)
	req := httptest.NewRequest(http.MethodPost, "/pullRequest/create", strings.NewReader(inputJSON))
	rr := httptest.NewRecorder()
	handler.CreatePR(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"pr":{"pull_request_id":"pr-1"`) {
		t.Errorf("body does not contain PR id")
	}
	if !strings.Contains(rr.Body.String(), `"pull_request_name":"My PR"`) {
		t.Errorf("body does not contain PR name")
	}
	if !strings.Contains(rr.Body.String(), `"author_id":"u1"`) {
		t.Errorf("body does not contain author_id")
	}
}

func TestMergePR(t *testing.T) {
	inputJSON := `{"pull_request_id":"pr-1"}`
	mockResult := service.JobResult{Data: models.PullRequest{PullRequestID: "pr-1"}}

	svcMock := mocks.NewServiceMock(t)
	svcMock.EnqueueJobMock.Set(func(job service.Job) {
		job.RespCh <- mockResult
	})

	handler := newTestHandler(t, svcMock)
	req := httptest.NewRequest(http.MethodPost, "/pullRequest/merge", strings.NewReader(inputJSON))
	rr := httptest.NewRecorder()
	handler.MergePR(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"pr":{"pull_request_id":"pr-1"`) {
		t.Errorf("body does not contain PR id")
	}
}

func TestReassign(t *testing.T) {
	testCases := []struct {
		name           string
		inputJSON      string
		mockJobResult  service.JobResult
		expectedStatus int
		expectedBody   string
	}{
		{
			name:      "Успешное переназначение",
			inputJSON: `{"pull_request_id": "pr1", "old_user_id": "u1"}`,
			mockJobResult: service.JobResult{
				Data: map[string]interface{}{
					"pr":       models.PullRequest{PullRequestID: "pr1"},
					"new_user": "u2",
				},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"new_user":"u2"`,
		},
		{
			name:      "PR смержен",
			inputJSON: `{"pull_request_id": "pr1", "old_user_id": "u1"}`,
			mockJobResult: service.JobResult{
				Error: service.ErrPRMerged,
			},
			expectedStatus: http.StatusConflict,
			expectedBody:   `cannot reassign on merged PR`,
		},
		{
			name:      "Нет кандидата",
			inputJSON: `{"pull_request_id": "pr1", "old_user_id": "u1"}`,
			mockJobResult: service.JobResult{
				Error: service.ErrNoCandidate,
			},
			expectedStatus: http.StatusConflict,
			expectedBody:   `no active replacement candidate in team`,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			svcMock := mocks.NewServiceMock(t)
			svcMock.EnqueueJobMock.Set(func(job service.Job) {
				job.RespCh <- tt.mockJobResult
			})

			handler := newTestHandler(t, svcMock)

			req := httptest.NewRequest(http.MethodPost, "/pr/reassign", strings.NewReader(tt.inputJSON))
			rr := httptest.NewRecorder()

			handler.Reassign(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tt.expectedBody) {
				t.Errorf("expected body to contain '%s', got '%s'", tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestGetTeam(t *testing.T) {
	tests := []struct {
		name           string
		targetURL      string
		mockJobResult  service.JobResult
		expectedStatus int
		expectedBody   string
	}{
		{
			name:      "Успешное получение команды",
			targetURL: "/team?team_name=alpha",
			mockJobResult: service.JobResult{
				Data: models.Team{TeamName: "alpha", Members: []models.TeamMember{}},
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"team_name":"alpha"`,
		},
		{
			name:           "Ошибка валидации",
			targetURL:      "/team?team_name=",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `team_name required`,
		},
		{
			name:      "Команда не найдена",
			targetURL: "/team?team_name=beta",
			mockJobResult: service.JobResult{
				Error: service.ErrNotFound,
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   `team not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcMock := mocks.NewServiceMock(t)
			if tt.mockJobResult.Data != nil || tt.mockJobResult.Error != nil {
				svcMock.EnqueueJobMock.Set(func(job service.Job) {
					job.RespCh <- tt.mockJobResult
				})
			}

			handler := newTestHandler(t, svcMock)
			req := httptest.NewRequest(http.MethodGet, tt.targetURL, nil)
			rr := httptest.NewRecorder()

			handler.GetTeam(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d. body: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tt.expectedBody) {
				t.Errorf("expected body to contain '%s', got '%s'", tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestGetUserReviews(t *testing.T) {
	url := "/reviews?user_id=u1"
	mockResult := service.JobResult{Data: []models.PullRequestShort{{PullRequestID: "pr1"}}}

	svcMock := mocks.NewServiceMock(t)
	svcMock.EnqueueJobMock.Set(func(job service.Job) {
		job.RespCh <- mockResult
	})

	handler := newTestHandler(t, svcMock)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	handler.GetUserReviews(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestGetStats(t *testing.T) {
	t.Run("Успешное получение статистики", func(t *testing.T) {
		svcMock := mocks.NewServiceMock(t)
		svcMock.GetStatsMock.Set(func(ctx context.Context) (map[string]int, error) {
			return map[string]int{"u1": 10, "u2": 5}, nil
		})

		handler := newTestHandler(t, svcMock)
		req := httptest.NewRequest(http.MethodGet, "/stats", nil)
		rr := httptest.NewRecorder()

		handler.GetStats(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `"u1":10`) {
			t.Errorf("body does not contain expected data: %s", rr.Body.String())
		}
	})

	t.Run("Ошибка сервиса", func(t *testing.T) {
		svcMock := mocks.NewServiceMock(t)
		svcMock.GetStatsMock.Set(func(ctx context.Context) (map[string]int, error) {
			return nil, errors.New("stats db error")
		})

		handler := newTestHandler(t, svcMock)
		req := httptest.NewRequest(http.MethodGet, "/stats", nil)
		rr := httptest.NewRecorder()

		handler.GetStats(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `stats db error`) {
			t.Errorf("body does not contain expected error: %s", rr.Body.String())
		}
	})
}

func TestDeactivateTeam(t *testing.T) {
	inputJSON := `{"team_name":"alpha"}`
	mockResult := service.JobResult{Data: nil}

	svcMock := mocks.NewServiceMock(t)
	svcMock.EnqueueJobMock.Set(func(job service.Job) {
		job.RespCh <- mockResult
	})

	handler := newTestHandler(t, svcMock)
	req := httptest.NewRequest(http.MethodPost, "/team/deactivate", strings.NewReader(inputJSON))
	rr := httptest.NewRecorder()
	handler.DeactivateTeam(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"status":"success"`) {
		t.Errorf("body does not contain status")
	}
}
