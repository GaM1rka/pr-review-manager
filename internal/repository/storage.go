package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"pr-review-manager/internal/models"

	_ "github.com/lib/pq"
)

type Storage struct {
	db *sql.DB
}

func NewStorage(databaseURL string) (*Storage, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &Storage{db: db}, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

// CreateTables создаёт таблицы в БД для хранения команд, пользователей и pull request'ов
func (s *Storage) CreateTables(logger *slog.Logger) error {
	logger.Info("Creating database tables")

	tx, err := s.db.Begin()
	if err != nil {
		logger.Error("begin tx failed", "err", err)
		return err
	}
	defer tx.Rollback()

	// teams: таблица команд с уникальным названием
	_, err = tx.Exec(`
        CREATE TABLE IF NOT EXISTS teams (
            team_name TEXT PRIMARY KEY
        )
    `)
	if err != nil {
		logger.Error("create teams table failed", "err", err)
		return err
	}

	// users: таблица пользователей с ссылкой на команду
	_, err = tx.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            user_id   TEXT PRIMARY KEY,
            username  TEXT NOT NULL,
            is_active BOOLEAN NOT NULL DEFAULT TRUE,
            team_name TEXT NOT NULL REFERENCES teams(team_name) ON DELETE CASCADE
        )
    `)
	if err != nil {
		logger.Error("create users table failed", "err", err)
		return err
	}

	// pull_requests: таблица pull request'ов со статусом и датами
	_, err = tx.Exec(`
        CREATE TABLE IF NOT EXISTS pull_requests (
            pull_request_id   TEXT PRIMARY KEY,
            pull_request_name TEXT NOT NULL,
            author_id         TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
            status            TEXT NOT NULL CHECK (status IN ('OPEN','MERGED')),
            created_at        TIMESTAMPTZ,
            merged_at         TIMESTAMPTZ
        )
    `)
	if err != nil {
		logger.Error("create pull_requests table failed", "err", err)
		return err
	}

	// reviewers: таблица для связи PR и рецензентов (макс 2 на PR)
	_, err = tx.Exec(`
        CREATE TABLE IF NOT EXISTS reviewers (
            pull_request_id TEXT NOT NULL REFERENCES pull_requests(pull_request_id) ON DELETE CASCADE,
            user_id         TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
            PRIMARY KEY (pull_request_id, user_id)
        )
    `)
	if err != nil {
		logger.Error("create reviewers table failed", "err", err)
		return err
	}

	if err := tx.Commit(); err != nil {
		logger.Error("commit create tables failed", "err", err)
		return err
	}
	logger.Info("DB tables ready")
	return nil
}

// CreateTeam создаёт новую команду в БД
func (s *Storage) CreateTeam(team models.Team) error {
	_, err := s.db.Exec(`INSERT INTO teams (team_name) VALUES ($1)`, team.TeamName)
	if err != nil {
		return fmt.Errorf("create team: %w", err)
	}
	return nil
}

// DeleteTeam удаляет команду по названию
func (s *Storage) DeleteTeam(teamName string) error {
	_, err := s.db.Exec(`DELETE FROM teams WHERE team_name=$1`, teamName)
	return err
}

// GetTeam получает команду со всеми её участниками
func (s *Storage) GetTeam(teamName string) (models.Team, error) {
	var t models.Team
	t.TeamName = teamName

	rows, err := s.db.Query(`SELECT user_id, username, is_active FROM users WHERE team_name=$1`, teamName)
	if err != nil {
		return t, fmt.Errorf("get team users: %w", err)
	}
	defer rows.Close()

	var count int
	err = s.db.QueryRow(`SELECT COUNT(*) FROM teams WHERE team_name=$1`, teamName).Scan(&count)
	if err != nil || count == 0 {
		return models.Team{}, fmt.Errorf("team not found")
	}

	members := []models.TeamMember{}
	for rows.Next() {
		var m models.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return t, fmt.Errorf("scan team member: %w", err)
		}
		members = append(members, m)
	}
	t.Members = members
	return t, nil
}

// UpsertUser вставляет или обновляет пользователя в БД
func (s *Storage) UpsertUser(u models.User) error {
	_, err := s.db.Exec(`
        INSERT INTO users (user_id, username, is_active, team_name)
        VALUES ($1,$2,$3,$4)
        ON CONFLICT (user_id) DO UPDATE
        SET username = EXCLUDED.username,
            is_active = EXCLUDED.is_active,
            team_name = EXCLUDED.team_name
    `, u.UserID, u.Username, u.IsActive, u.TeamName)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

// GetUser получает информацию о пользователе по ID
func (s *Storage) GetUser(userID string) (models.User, error) {
	var u models.User
	row := s.db.QueryRow(`SELECT user_id, username, team_name, is_active FROM users WHERE user_id=$1`, userID)
	if err := row.Scan(&u.UserID, &u.Username, &u.TeamName, &u.IsActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return u, fmt.Errorf("user not found: %w", err)
		}
		return u, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}

// UpdateUser обновляет информацию о пользователе
func (s *Storage) UpdateUser(u models.User) error {
	res, err := s.db.Exec(`UPDATE users SET username=$1, is_active=$2, team_name=$3 WHERE user_id=$4`,
		u.Username, u.IsActive, u.TeamName, u.UserID)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("update user: not found")
	}
	return nil
}

// CreatePullRequest создаёт новый pull request в БД
func (s *Storage) CreatePullRequest(pr models.PullRequest) error {
	_, err := s.db.Exec(`
        INSERT INTO pull_requests
          (pull_request_id, pull_request_name, author_id, status, created_at)
        VALUES ($1,$2,$3,$4,$5)
    `, pr.PullRequestID, pr.PullRequestName, pr.AuthorID, string(pr.Status), pr.CreatedAt)
	if err != nil {
		return fmt.Errorf("create pr: %w", err)
	}
	return nil
}

// GetPullRequest получает pull request со всеми его рецензентами
func (s *Storage) GetPullRequest(prID string) (models.PullRequest, error) {
	var pr models.PullRequest
	var createdAt sql.NullTime
	var mergedAt sql.NullTime
	row := s.db.QueryRow(`
        SELECT pull_request_id, pull_request_name, author_id, status, created_at, merged_at
        FROM pull_requests WHERE pull_request_id=$1
    `, prID)
	var status string
	if err := row.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &status, &createdAt, &mergedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pr, fmt.Errorf("pr not found: %w", err)
		}
		return pr, fmt.Errorf("scan pr: %w", err)
	}
	pr.Status = models.PRStatus(status)
	if createdAt.Valid {
		pr.CreatedAt = createdAt.Time
	}
	if mergedAt.Valid {
		pr.MergedAt = mergedAt.Time
	}
	// загружаем назначенных рецензентов
	reviewers, err := s.ListReviewersByPR(prID)
	if err != nil {
		return pr, fmt.Errorf("list reviewers: %w", err)
	}
	pr.AssignedReviewers = reviewers
	return pr, nil
}

// UpdatePullRequest обновляет pull request и его список рецензентов
func (s *Storage) UpdatePullRequest(pr models.PullRequest) error {
	_, err := s.db.Exec(`
        UPDATE pull_requests SET pull_request_name=$1, author_id=$2, status=$3, created_at=$4, merged_at=$5
        WHERE pull_request_id=$6
    `, pr.PullRequestName, pr.AuthorID, string(pr.Status), pr.CreatedAt, sqlNullTime(pr.MergedAt), pr.PullRequestID)
	if err != nil {
		return fmt.Errorf("update pr: %w", err)
	}
	// удаляем всех старых рецензентов и вставляем новых
	_, err = s.db.Exec(`DELETE FROM reviewers WHERE pull_request_id=$1`, pr.PullRequestID)
	if err != nil {
		return fmt.Errorf("delete reviewers on update: %w", err)
	}
	for _, uid := range pr.AssignedReviewers {
		_, err := s.db.Exec(`INSERT INTO reviewers (pull_request_id, user_id) VALUES ($1,$2)`, pr.PullRequestID, uid)
		if err != nil {
			return fmt.Errorf("insert reviewer on update: %w", err)
		}
	}
	return nil
}

// AssignReviewer назначает рецензента для pull request'а
func (s *Storage) AssignReviewer(prID, userID string) error {
	_, err := s.db.Exec(`
        INSERT INTO reviewers (pull_request_id, user_id) VALUES ($1,$2)
    `, prID, userID)
	if err != nil {
		return fmt.Errorf("assign reviewer: %w", err)
	}
	return nil
}

// ListReviewersByPR получает список ID всех рецензентов для pull request'а
func (s *Storage) ListReviewersByPR(prID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT user_id FROM reviewers WHERE pull_request_id=$1 ORDER BY user_id`, prID)
	if err != nil {
		return nil, fmt.Errorf("list reviewers by pr: %w", err)
	}
	defer rows.Close()
	var list []string
	for rows.Next() {
		var uid string
		if err := rows.Scan(&uid); err != nil {
			return nil, fmt.Errorf("scan reviewer uid: %w", err)
		}
		list = append(list, uid)
	}
	return list, nil
}

// ListActiveMembers получает всех активных участников команды
func (s *Storage) ListActiveMembers(teamName string) ([]models.TeamMember, error) {
	rows, err := s.db.Query(`SELECT user_id, username, is_active FROM users WHERE team_name=$1 AND is_active = TRUE`, teamName)
	if err != nil {
		return nil, fmt.Errorf("list active members: %w", err)
	}
	defer rows.Close()
	var members []models.TeamMember
	for rows.Next() {
		var m models.TeamMember
		if err := rows.Scan(&m.UserID, &m.Username, &m.IsActive); err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		members = append(members, m)
	}
	return members, nil
}

// ListPRsByReviewer получает список pull request'ов, для которых пользователь назначен рецензентом
func (s *Storage) ListPRsByReviewer(userID string) ([]models.PullRequestShort, error) {
	rows, err := s.db.Query(`
        SELECT p.pull_request_id, p.pull_request_name, p.author_id, p.status
        FROM pull_requests p
        JOIN reviewers r ON r.pull_request_id = p.pull_request_id
        WHERE r.user_id = $1
    `, userID)
	if err != nil {
		return nil, fmt.Errorf("list prs by reviewer: %w", err)
	}
	defer rows.Close()

	var result []models.PullRequestShort
	for rows.Next() {
		var pr models.PullRequestShort
		var status string
		if err := rows.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &status); err != nil {
			return nil, fmt.Errorf("scan pr short: %w", err)
		}
		pr.Status = models.PRStatus(status)
		result = append(result, pr)
	}
	return result, nil
}

// sqlNullTime преобразует время в sql.NullTime (NULL если время нулевое)
func sqlNullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}
