package repo

import (
	"database/sql"
	"fmt"
	"time"

	"PR-reviewer/internal/models"
)

type PostgresRepo struct {
	db *sql.DB
}

func NewPostgresRepo(db *sql.DB) *PostgresRepo {
	return &PostgresRepo{db: db}
}

func (r *PostgresRepo) InsertTeam(team models.Team) error {
	if _, err := r.db.Exec(`INSERT INTO teams(team_name) VALUES ($1) ON CONFLICT (team_name) DO NOTHING`, team.TeamName); err != nil {
		return fmt.Errorf("insert team: %w", err)
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.Prepare(`INSERT INTO users(user_id, username, team_name, is_active)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (user_id) DO UPDATE SET username=EXCLUDED.username, team_name=EXCLUDED.team_name, is_active=EXCLUDED.is_active`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, m := range team.Members {
		if _, err := stmt.Exec(m.UserID, m.Username, team.TeamName, m.IsActive); err != nil {
			return fmt.Errorf("exec upsert user: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (r *PostgresRepo) GetTeam(teamName string) (models.Team, error) {
	var res models.Team
	rows, err := r.db.Query(`SELECT user_id, username, is_active FROM users WHERE team_name = $1 ORDER BY user_id`, teamName)
	if err != nil {
		return res, fmt.Errorf("query team members: %w", err)
	}
	defer rows.Close()

	members := make([]models.TeamMember, 0)
	for rows.Next() {
		var m models.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return res, fmt.Errorf("scan member: %w", err)
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("rows err: %w", err)
	}

	if len(members) == 0 {
		return res, fmt.Errorf("not found")
	}

	res.TeamName = teamName
	res.Members = members
	return res, nil
}

func (r *PostgresRepo) UpdateUserActive(userID string, isActive bool) (models.User, error) {
	var u models.User

	res, err := r.db.Exec(`UPDATE users SET is_active = $1 WHERE user_id = $2`, isActive, userID)
	if err != nil {
		return u, fmt.Errorf("update user active: %w", err)
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return u, fmt.Errorf("not found")
	}

	row := r.db.QueryRow(`SELECT user_id, username, team_name, is_active FROM users WHERE user_id = $1`, userID)
	if err := row.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
		if err == sql.ErrNoRows {
			return u, fmt.Errorf("not found")
		}
		return u, fmt.Errorf("select updated user: %w", err)
	}
	return u, nil
}

func (r *PostgresRepo) CreatePR(pr models.PullRequest) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(`INSERT INTO pull_requests(pull_request_id, pull_request_name, author_id, status, created_at) VALUES ($1,$2,$3,$4,$5)`,
		pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, pr.CreatedAt); err != nil {
		return fmt.Errorf("insert pr: %w", err)
	}

	if len(pr.Assigned) > 0 {
		stmt, err := tx.Prepare(`INSERT INTO pr_reviewers(pull_request_id, user_id) VALUES ($1,$2)`)
		if err != nil {
			return fmt.Errorf("prepare insert reviewers: %w", err)
		}
		defer stmt.Close()
		for _, uid := range pr.Assigned {
			if _, err := stmt.Exec(pr.PullRequestID, uid); err != nil {
				return fmt.Errorf("insert reviewer: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (r *PostgresRepo) GetPR(prID string) (models.PullRequest, error) {
	var pr models.PullRequest
	var mergedAt sql.NullTime

	row := r.db.QueryRow(`SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at FROM pull_requests WHERE pull_request_id = $1`, prID)
	if err := row.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status, &pr.CreatedAt, &mergedAt); err != nil {
		if err == sql.ErrNoRows {
			return pr, fmt.Errorf("not found")
		}
		return pr, fmt.Errorf("select pr: %w", err)
	}
	if mergedAt.Valid {
		t := mergedAt.Time
		pr.MergedAt = &t
	}

	rows, err := r.db.Query(`SELECT user_id FROM pr_reviewers WHERE pull_request_id = $1 ORDER BY user_id`, prID)
	if err != nil {
		return pr, fmt.Errorf("query reviewers: %w", err)
	}
	defer rows.Close()
	revs := make([]string, 0)
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return pr, fmt.Errorf("scan reviewer: %w", err)
		}
		revs = append(revs, uid)
	}
	if err := rows.Err(); err != nil {
		return pr, fmt.Errorf("rows err: %w", err)
	}
	pr.Assigned = revs
	return pr, nil
}

func (r *PostgresRepo) MergePR(prID string, t time.Time) (models.PullRequest, error) {
	if _, err := r.db.Exec(`UPDATE pull_requests SET status='MERGED', merged_at=$1 WHERE pull_request_id=$2`, t, prID); err != nil {
		return models.PullRequest{}, fmt.Errorf("update merge: %w", err)
	}
	return r.GetPR(prID)
}

func (r *PostgresRepo) ReplaceReviewer(prID, oldUID, newUID string) (models.PullRequest, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return models.PullRequest{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(`DELETE FROM pr_reviewers WHERE pull_request_id=$1 AND user_id=$2`, prID, oldUID); err != nil {
		return models.PullRequest{}, fmt.Errorf("delete old reviewer: %w", err)
	}

	if _, err := tx.Exec(`INSERT INTO pr_reviewers(pull_request_id, user_id) VALUES ($1,$2)`, prID, newUID); err != nil {
		return models.PullRequest{}, fmt.Errorf("insert new reviewer: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return models.PullRequest{}, fmt.Errorf("commit: %w", err)
	}

	return r.GetPR(prID)
}

func (r *PostgresRepo) GetActiveTeamMembersExcept(teamName, exceptUser string) ([]string, error) {
	if exceptUser == "" {
		rows, err := r.db.Query(`SELECT user_id FROM users WHERE team_name=$1 AND is_active=true ORDER BY user_id`, teamName)
		if err != nil {
			return nil, fmt.Errorf("query active members: %w", err)
		}
		defer rows.Close()
		res := make([]string, 0)
		for rows.Next() {
			var uid string
			if err := rows.Scan(&uid); err != nil {
				return nil, fmt.Errorf("scan uid: %w", err)
			}
			res = append(res, uid)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("rows err: %w", err)
		}
		return res, nil
	}

	rows, err := r.db.Query(`SELECT user_id FROM users WHERE team_name=$1 AND is_active=true AND user_id<>$2 ORDER BY user_id`, teamName, exceptUser)
	if err != nil {
		return nil, fmt.Errorf("query active members except: %w", err)
	}
	defer rows.Close()
	res := make([]string, 0)
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("scan uid: %w", err)
		}
		res = append(res, uid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	return res, nil
}

func (r *PostgresRepo) GetUserTeam(userID string) (string, error) {
	var team string
	row := r.db.QueryRow(`SELECT team_name FROM users WHERE user_id=$1`, userID)
	if err := row.Scan(&team); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("not found")
		}
		return "", fmt.Errorf("select user team: %w", err)
	}
	return team, nil
}

func (r *PostgresRepo) GetPRsByReviewer(userID string) ([]models.PullRequestShort, error) {
	rows, err := r.db.Query(`
		SELECT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status
		FROM pull_requests pr
		JOIN pr_reviewers rr ON pr.pull_request_id = rr.pull_request_id
		WHERE rr.user_id = $1
		ORDER BY pr.created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query prs by reviewer: %w", err)
	}
	defer rows.Close()

	res := make([]models.PullRequestShort, 0)
	for rows.Next() {
		var p models.PullRequestShort
		if err := rows.Scan(&p.PullRequestID, &p.PullRequestName, &p.AuthorID, &p.Status); err != nil {
			return nil, fmt.Errorf("scan pr short: %w", err)
		}
		res = append(res, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}
	return res, nil
}
