package service

import (
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"pr-review-manager/internal/models"
	"pr-review-manager/internal/repository"
)

type Service struct {
	storage *repository.Storage
	rnd     *rand.Rand
	logger  *slog.Logger
}

func NewService(stor *repository.Storage, logger *slog.Logger) *Service {
	return &Service{
		storage: stor,
		rnd:     rand.New(rand.NewSource(time.Now().UnixNano())),
		logger:  logger,
	}
}

// errWithCode создаёт ошибку с кодом ошибки
func errWithCode(code, msg string) error {
	return &models.ErrorResponse{
		ErrDetail: models.ErrorDetail{
			Code:    code,
			Message: msg,
		},
	}
}

// ParseCodeFromError извлекает код ошибки из ошибки сервиса
func ParseCodeFromError(err error) string {
	if err == nil {
		return ""
	}
	parts := strings.SplitN(err.Error(), ": ", 2)
	if len(parts) >= 2 {
		return parts[0]
	}
	return ""
}

// AddTeam создаёт новую команду и добавляет пользователей
func (s *Service) AddTeam(team *models.Team) (*models.TeamResponse, error) {
	if s.logger != nil {
		s.logger.Info("AddTeam вызван", slog.String("team_name", team.TeamName))
	}

	// пытаемся создать команду
	if err := s.storage.CreateTeam(*team); err != nil {
		if strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "23505") {
			if s.logger != nil {
				s.logger.Warn("команда уже существует", slog.String("team_name", team.TeamName))
			}
			return nil, errWithCode(models.ErrorCodeTeamExists, "team_name already exists")
		}
		if s.logger != nil {
			s.logger.Error("не удалось создать команду", slog.Any("err", err))
		}
		return nil, fmt.Errorf("failed create team: %w", err)
	}

	// добавляем пользователей команды
	for _, m := range team.Members {
		u := models.User{
			UserID:   m.UserID,
			Username: m.Username,
			TeamName: team.TeamName,
			IsActive: m.IsActive,
		}
		if err := s.storage.UpsertUser(u); err != nil {
			if s.logger != nil {
				s.logger.Error("не удалось добавить пользователя", slog.String("user_id", u.UserID), slog.Any("err", err))
			}
			_ = s.storage.DeleteTeam(team.TeamName)
			return nil, fmt.Errorf("failed upsert user %s: %w", u.UserID, err)
		}
		if s.logger != nil {
			s.logger.Debug("пользователь добавлен", slog.String("user_id", u.UserID))
		}
	}

	if s.logger != nil {
		s.logger.Info("команда создана", slog.String("team_name", team.TeamName))
	}
	return &models.TeamResponse{Team: *team}, nil
}

// GetTeam получает информацию о команде по названию
func (s *Service) GetTeam(teamName string) (*models.TeamResponse, error) {
	if s.logger != nil {
		s.logger.Info("GetTeam вызван", slog.String("team_name", teamName))
	}
	team, err := s.storage.GetTeam(teamName)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("команда не найдена", slog.String("team_name", teamName), slog.Any("err", err))
		}
		return nil, errWithCode(models.ErrorCodeNotFound, "team not found")
	}
	return &models.TeamResponse{Team: team}, nil
}

// SetUserActive изменяет статус активности пользователя
func (s *Service) SetUserActive(userID string, isActive bool) (*models.UserResponse, error) {
	if s.logger != nil {
		s.logger.Info("SetUserActive вызван", slog.String("user_id", userID), slog.Bool("is_active", isActive))
	}
	u, err := s.storage.GetUser(userID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("пользователь не найден", slog.String("user_id", userID), slog.Any("err", err))
		}
		return nil, errWithCode(models.ErrorCodeNotFound, "user not found")
	}

	u.IsActive = isActive
	if err := s.storage.UpdateUser(u); err != nil {
		if s.logger != nil {
			s.logger.Error("не удалось обновить пользователя", slog.String("user_id", userID), slog.Any("err", err))
		}
		return nil, fmt.Errorf("failed update user: %w", err)
	}

	if s.logger != nil {
		s.logger.Info("пользователь обновлён", slog.String("user_id", userID), slog.Bool("is_active", isActive))
	}
	return &models.UserResponse{User: u}, nil
}

// CreatePullRequest создаёт новый pull request и назначает рецензентов
func (s *Service) CreatePullRequest(req *models.CreatePullRequestRequest) (*models.PullRequestResponse, error) {
	if s.logger != nil {
		s.logger.Info("CreatePullRequest вызван", slog.String("pr_id", req.PullRequestID), slog.String("author", req.AuthorID))
	}

	// проверяем что PR не существует
	if _, err := s.storage.GetPullRequest(req.PullRequestID); err == nil {
		if s.logger != nil {
			s.logger.Warn("PR уже существует", slog.String("pr_id", req.PullRequestID))
		}
		return nil, errWithCode(models.ErrorCodePRExists, "PR id already exists")
	}

	author, err := s.storage.GetUser(req.AuthorID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("автор не найден", slog.String("author", req.AuthorID), slog.Any("err", err))
		}
		return nil, errWithCode(models.ErrorCodeNotFound, "author not found")
	}

	team, err := s.storage.GetTeam(author.TeamName)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("команда автора не найдена", slog.String("team", author.TeamName), slog.Any("err", err))
		}
		return nil, errWithCode(models.ErrorCodeNotFound, "team not found")
	}

	// собираем активных кандидатов (исключая автора)
	candidates := []models.TeamMember{}
	for _, m := range team.Members {
		if !m.IsActive {
			continue
		}
		if m.UserID == author.UserID {
			continue
		}
		candidates = append(candidates, m)
	}

	if s.logger != nil {
		s.logger.Debug("кандидаты собраны", slog.Int("count", len(candidates)))
	}

	assigned := []string{}
	// перемешиваем и выбираем до 2 человек
	if len(candidates) > 0 {
		// Fisher–Yates shuffle
		for i := len(candidates) - 1; i > 0; i-- {
			j := s.rnd.Intn(i + 1)
			candidates[i], candidates[j] = candidates[j], candidates[i]
		}
		limit := 2
		if len(candidates) < 2 {
			limit = len(candidates)
		}
		for i := 0; i < limit; i++ {
			assigned = append(assigned, candidates[i].UserID)
		}
	}

	if s.logger != nil {
		s.logger.Info("рецензенты назначены", slog.String("pr_id", req.PullRequestID), slog.Any("assigned", assigned))
	}

	now := time.Now().UTC()
	pr := models.PullRequest{
		PullRequestID:     req.PullRequestID,
		PullRequestName:   req.PullRequestName,
		AuthorID:          req.AuthorID,
		Status:            models.PRStatusOpen,
		AssignedReviewers: assigned,
		CreatedAt:         now,
	}

	// создаём запись PR
	if err := s.storage.CreatePullRequest(pr); err != nil {
		if s.logger != nil {
			s.logger.Error("не удалось создать PR", slog.String("pr_id", pr.PullRequestID), slog.Any("err", err))
		}
		return nil, fmt.Errorf("failed create pr: %w", err)
	}

	// добавляем рецензентов
	for _, reviewerID := range assigned {
		if err := s.storage.AssignReviewer(pr.PullRequestID, reviewerID); err != nil {
			if s.logger != nil {
				s.logger.Error("не удалось назначить рецензента", slog.String("pr_id", pr.PullRequestID), slog.String("reviewer", reviewerID), slog.Any("err", err))
			}
			return nil, fmt.Errorf("failed assign reviewer %s: %w", reviewerID, err)
		}
	}

	if s.logger != nil {
		s.logger.Info("PR создан", slog.String("pr_id", pr.PullRequestID))
	}
	return &models.PullRequestResponse{PR: pr}, nil
}

// MergePullRequest объединяет pull request (меняет статус на MERGED)
func (s *Service) MergePullRequest(prID string) (*models.PullRequestResponse, error) {
	if s.logger != nil {
		s.logger.Info("MergePullRequest вызван", slog.String("pr_id", prID))
	}

	pr, err := s.storage.GetPullRequest(prID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("не удалось получить PR", slog.String("pr_id", prID), slog.Any("err", err))
		}
		return nil, errWithCode(models.ErrorCodeNotFound, "pr not found")
	}

	// идемпотентность: если уже объединён, возвращаем текущее состояние
	if pr.Status == models.PRStatusMerged {
		if s.logger != nil {
			s.logger.Debug("PR уже объединён", slog.String("pr_id", prID))
		}
		return &models.PullRequestResponse{PR: pr}, nil
	}

	pr.Status = models.PRStatusMerged
	pr.MergedAt = time.Now().UTC()
	if err := s.storage.UpdatePullRequest(pr); err != nil {
		if s.logger != nil {
			s.logger.Error("не удалось обновить PR", slog.String("pr_id", prID), slog.Any("err", err))
		}
		return nil, fmt.Errorf("failed update pr: %w", err)
	}

	if s.logger != nil {
		s.logger.Info("PR объединён", slog.String("pr_id", prID))
	}
	return &models.PullRequestResponse{PR: pr}, nil
}

// ReassignReviewer заменяет одного рецензента на случайного активного из его команды
func (s *Service) ReassignReviewer(prID, oldUserID string) (*models.PullRequest, string, error) {
	if s.logger != nil {
		s.logger.Info("ReassignReviewer вызван", slog.String("pr_id", prID), slog.String("old_reviewer", oldUserID))
	}

	pr, err := s.storage.GetPullRequest(prID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("не удалось получить PR", slog.String("pr_id", prID), slog.Any("err", err))
		}
		return nil, "", errWithCode(models.ErrorCodeNotFound, "pr not found")
	}

	if pr.Status == models.PRStatusMerged {
		if s.logger != nil {
			s.logger.Warn("не можно переназначить уже объединённый PR", slog.String("pr_id", prID))
		}
		return nil, "", errWithCode(models.ErrorCodePRMerged, "cannot reassign on merged PR")
	}

	// проверяем что oldUserID назначен рецензентом
	found := -1
	for i, id := range pr.AssignedReviewers {
		if id == oldUserID {
			found = i
			break
		}
	}
	if found == -1 {
		if s.logger != nil {
			s.logger.Warn("рецензент не назначен на PR", slog.String("pr_id", prID), slog.String("reviewer", oldUserID))
		}
		return nil, "", errWithCode(models.ErrorCodeNotAssigned, "reviewer is not assigned to this PR")
	}

	// находим команду рецензента
	oldUser, err := s.storage.GetUser(oldUserID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("пользователь не найден", slog.String("user_id", oldUserID), slog.Any("err", err))
		}
		return nil, "", errWithCode(models.ErrorCodeNotFound, "user not found")
	}

	team, err := s.storage.GetTeam(oldUser.TeamName)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("команда не найдена", slog.String("team", oldUser.TeamName), slog.Any("err", err))
		}
		return nil, "", errWithCode(models.ErrorCodeNotFound, "team not found")
	}

	// кандидаты: активные члены команды кроме текущих рецензентов и автора
	candidates := []models.TeamMember{}
	exclude := map[string]struct{}{oldUserID: {}}
	for _, rid := range pr.AssignedReviewers {
		exclude[rid] = struct{}{}
	}
	exclude[pr.AuthorID] = struct{}{}

	for _, m := range team.Members {
		if !m.IsActive {
			continue
		}
		if _, ex := exclude[m.UserID]; ex {
			continue
		}
		candidates = append(candidates, m)
	}

	if len(candidates) == 0 {
		if s.logger != nil {
			s.logger.Warn("нет подходящего замены для рецензента", slog.String("pr_id", prID))
		}
		return nil, "", errWithCode(models.ErrorCodeNoCandidate, "no active replacement candidate in team")
	}

	// выбираем случайного кандидата
	idx := s.rnd.Intn(len(candidates))
	newReviewer := candidates[idx].UserID

	// заменяем в памяти
	pr.AssignedReviewers[found] = newReviewer

	// сохраняем в БД
	if err := s.storage.UpdatePullRequest(pr); err != nil {
		if s.logger != nil {
			s.logger.Error("не удалось обновить PR при переназначении", slog.String("pr_id", prID), slog.Any("err", err))
		}
		return nil, "", fmt.Errorf("failed update pr: %w", err)
	}

	if s.logger != nil {
		s.logger.Info("рецензент переназначен", slog.String("pr_id", prID), slog.String("new_reviewer", newReviewer))
	}
	return &pr, newReviewer, nil
}

// GetReviewPRs получает список PR'ов, на которых пользователь назначен рецензентом
func (s *Service) GetReviewPRs(userID string) (*models.UserReviewResponse, error) {
	if s.logger != nil {
		s.logger.Info("GetReviewPRs вызван", slog.String("user_id", userID))
	}
	prs, err := s.storage.ListPRsByReviewer(userID)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("не удалось получить PR'ы рецензента", slog.String("user_id", userID), slog.Any("err", err))
		}
		return nil, errWithCode(models.ErrorCodeNotFound, "user or prs not found")
	}
	return &models.UserReviewResponse{
		UserID:       userID,
		PullRequests: prs,
	}, nil
}
