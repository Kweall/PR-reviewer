package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"

	"PR-reviewer/internal/handlers"
	"PR-reviewer/internal/logger"
	"PR-reviewer/internal/repo"
	"PR-reviewer/internal/service"
)

var testDB *sql.DB
var server *httptest.Server

func TestMain(m *testing.M) {
	dsn := "postgres://pruser:prpass@localhost:5432/prdb_test?sslmode=disable"

	var err error
	testDB, err = sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}

	for i := 0; i < 10; i++ {
		if err := testDB.Ping(); err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	createTables(testDB)

	clearDB(testDB)

	appLog := logger.NewStdLogger(os.Stdout, "debug")
	r := chi.NewRouter()
	svc := service.NewService(repo.NewPostgresRepo(testDB), appLog)
	h := handlers.NewHandler(svc, appLog)

	r.Post("/team/add", h.AddTeam)
	r.Get("/team/get", h.GetTeam)
	r.Post("/users/setIsActive", h.SetIsActive)
	r.Post("/pullRequest/create", h.CreatePR)
	r.Post("/pullRequest/merge", h.MergePR)
	r.Post("/pullRequest/reassign", h.Reassign)
	r.Get("/users/getReview", h.GetUserReviews)
	r.Get("/stats", h.GetStats)
	r.Post("/team/deactivate", h.DeactivateTeam)

	server = httptest.NewServer(r)
	defer server.Close()

	code := m.Run()

	clearDB(testDB)
	testDB.Close()
	os.Exit(code)
}

func createTables(db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS teams (
			team_name TEXT PRIMARY KEY
		);`,
		`CREATE TABLE IF NOT EXISTS users (
			user_id TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			team_name TEXT NOT NULL REFERENCES teams(team_name) ON DELETE CASCADE,
			is_active BOOLEAN NOT NULL DEFAULT TRUE
		);`,
		`CREATE TABLE IF NOT EXISTS pull_requests (
			pull_request_id TEXT PRIMARY KEY,
			pull_request_name TEXT NOT NULL,
			author_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			merged_at TIMESTAMP NULL
		);`,
		`CREATE TABLE IF NOT EXISTS pr_reviewers (
			pull_request_id TEXT NOT NULL REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			PRIMARY KEY (pull_request_id, user_id)
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			panic(fmt.Sprintf("failed to create table: %v", err))
		}
	}
}

func clearDB(db *sql.DB) {
	tables := []string{"pr_reviewers", "pull_requests", "users", "teams"}
	for _, t := range tables {
		db.Exec("TRUNCATE TABLE " + t + " CASCADE;")
	}
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	data, _ := json.Marshal(body)
	resp, err := http.Post(server.URL+url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	return resp
}

func getJSON(t *testing.T, url string) *http.Response {
	resp, err := http.Get(server.URL + url)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	return resp
}

func decodeJSONBody(t *testing.T, resp *http.Response, out any) {
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
}

func TestE2E(t *testing.T) {
	addTeamResp := postJSON(t, "/team/add", map[string]interface{}{
		"team_name": "backend",
		"members": []map[string]interface{}{
			{"user_id": "u1", "username": "Alice", "is_active": true},
			{"user_id": "u2", "username": "Bob", "is_active": true},
			{"user_id": "u3", "username": "Charlie", "is_active": false},
		},
	})
	if addTeamResp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", addTeamResp.StatusCode)
	}

	var count int
	err := testDB.QueryRow("SELECT COUNT(*) FROM teams WHERE team_name=$1", "backend").Scan(&count)
	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 team in DB, got %d", count)
	}

	createPRResp := postJSON(t, "/pullRequest/create", map[string]string{"pull_request_id": "pr-10000", "pull_request_name": "Test PR", "author_id": "u1"})
	if createPRResp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", createPRResp.StatusCode)
	}

	var prID string
	err = testDB.QueryRow("SELECT pull_request_id FROM pull_requests WHERE pull_request_name=$1", "Test PR").Scan(&prID)
	if err != nil {
		t.Fatalf("PR not found in DB: %v", err)
	}

	type teamResp struct {
		TeamName string `json:"team_name"`
		Members  []struct {
			UserID   string `json:"user_id"`
			Username string `json:"username"`
			IsActive bool   `json:"is_active"`
		} `json:"members"`
	}
	var team teamResp
	getResp := getJSON(t, "/team/get?team_name=backend")
	decodeJSONBody(t, getResp, &team)

	if team.TeamName != "backend" {
		t.Fatalf("expected backend team in response")
	}

	if len(team.Members) == 0 {
		t.Fatalf("expected members in backend team")
	}

	reassignResp := postJSON(t, "/pullRequest/reassign", map[string]string{"pull_request_id": prID, "old_user_id": "u2"})
	if reassignResp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", reassignResp.StatusCode)
	}

	getReviewsResp := getJSON(t, "/users/getReview?user_id=u2")
	if getReviewsResp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", getReviewsResp.StatusCode)
	}

	deactivateResp := postJSON(t, "/team/deactivate", map[string]string{"team_name": "backend"})
	if deactivateResp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", deactivateResp.StatusCode)
	}

	var active bool
	testDB.QueryRow("SELECT is_active FROM teams WHERE team_name=$1", "backend").Scan(&active)
	if active {
		t.Fatalf("expected team to be inactive")
	}
}
