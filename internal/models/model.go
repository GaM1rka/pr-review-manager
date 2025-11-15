package models

import (
	"time"
)

func (e *ErrorResponse) Error() string {
	return e.ErrDetail.Message
}

// ErrorDetail представляет детали ошибки
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse представляет структуру ответа об ошибке
type ErrorResponse struct {
	ErrDetail ErrorDetail `json:"error"`
}

// Team представляет команду с участниками
type Team struct {
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

// TeamMember представляет участника команды
type TeamMember struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	IsActive bool   `json:"is_active"`
}

// User представляет пользователя
type User struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	TeamName string `json:"team_name"`
	IsActive bool   `json:"is_active"`
}

// PullRequest представляет полную информацию о pull request
type PullRequest struct {
	PullRequestID     string    `json:"pull_request_id"`
	PullRequestName   string    `json:"pull_request_name"`
	AuthorID          string    `json:"author_id"`
	Status            PRStatus  `json:"status"`
	AssignedReviewers []string  `json:"assigned_reviewers"`
	CreatedAt         time.Time `json:"createdAt,omitempty"`
	MergedAt          time.Time `json:"mergedAt,omitempty"`
}

// PullRequestShort представляет сокращенную информацию о pull request
type PullRequestShort struct {
	PullRequestID   string   `json:"pull_request_id"`
	PullRequestName string   `json:"pull_request_name"`
	AuthorID        string   `json:"author_id"`
	Status          PRStatus `json:"status"`
}

// PRStatus представляет статус pull request
type PRStatus string

const (
	PRStatusOpen   PRStatus = "OPEN"
	PRStatusMerged PRStatus = "MERGED"
)

// SetUserActiveRequest представляет запрос на установку флага активности пользователя
type SetUserActiveRequest struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
}

// CreatePullRequestRequest представляет запрос на создание PR
type CreatePullRequestRequest struct {
	PullRequestID   string `json:"pull_request_id"`
	PullRequestName string `json:"pull_request_name"`
	AuthorID        string `json:"author_id"`
}

// MergePullRequestRequest представляет запрос на мерж PR
type MergePullRequestRequest struct {
	PullRequestID string `json:"pull_request_id"`
}

// ReassignPullRequestRequest представляет запрос на переназначение ревьювера
type ReassignPullRequestRequest struct {
	PullRequestID string `json:"pull_request_id"`
	OldUserID     string `json:"old_user_id"`
}

// ReassignPullRequestResponse представляет ответ на переназначение ревьювера
type ReassignPullRequestResponse struct {
	PR         PullRequest `json:"pr"`
	ReplacedBy string      `json:"replaced_by"`
}

// UserReviewResponse представляет ответ с PR пользователя для ревью
type UserReviewResponse struct {
	UserID       string             `json:"user_id"`
	PullRequests []PullRequestShort `json:"pull_requests"`
}

// TeamResponse представляет ответ с информацией о команде
type TeamResponse struct {
	Team Team `json:"team"`
}

// UserResponse представляет ответ с информацией о пользователе
type UserResponse struct {
	User User `json:"user"`
}

// PullRequestResponse представляет ответ с информацией о PR
type PullRequestResponse struct {
	PR PullRequest `json:"pr"`
}

// Error codes
const (
	ErrorCodeTeamExists  = "TEAM_EXISTS"
	ErrorCodePRExists    = "PR_EXISTS"
	ErrorCodePRMerged    = "PR_MERGED"
	ErrorCodeNotAssigned = "NOT_ASSIGNED"
	ErrorCodeNoCandidate = "NO_CANDIDATE"
	ErrorCodeNotFound    = "NOT_FOUND"
)
