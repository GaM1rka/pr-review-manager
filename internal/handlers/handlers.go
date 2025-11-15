package handlers

import (
	"encoding/json"
	"net/http"

	"log/slog"
	"pr-review-manager/internal/models"
	"pr-review-manager/internal/service"
)

type Handler struct {
	service *service.Service
	logger  *slog.Logger
}

func NewHandler(s *service.Service, logger *slog.Logger) *Handler {
	return &Handler{service: s, logger: logger}
}

// writeJSON отправляет JSON ответ с заданным HTTP статусом
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError отправляет ошибку в формате JSON с кодом и сообщением
func writeError(w http.ResponseWriter, status int, code, msg string) {
	errResp := &models.ErrorResponse{
		ErrDetail: models.ErrorDetail{
			Code:    code,
			Message: msg,
		},
	}
	writeJSON(w, status, errResp)
}

// getStatusByCode преобразует код ошибки в HTTP статус
func getStatusByCode(code string) int {
	switch code {
	case models.ErrorCodeNotFound:
		return http.StatusNotFound
	case models.ErrorCodeTeamExists, models.ErrorCodePRExists:
		return http.StatusConflict
	case models.ErrorCodePRMerged, models.ErrorCodeNotAssigned, models.ErrorCodeNoCandidate:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// AddHandler создаёт новую команду (POST /team/add)
func (h *Handler) AddHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("AddHandler called", slog.String("remote", r.RemoteAddr))

	var team models.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		h.logger.Error("invalid request body in AddHandler", slog.Any("err", err))
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	teamResp, err := h.service.AddTeam(&team)
	if err != nil {
		h.logger.Error("AddTeam failed", slog.Any("err", err))
		code := service.ParseCodeFromError(err)
		status := getStatusByCode(code)
		if er, ok := err.(*models.ErrorResponse); ok {
			writeJSON(w, status, er)
			return
		}
		writeError(w, status, "INTERNAL_ERROR", err.Error())
		return
	}

	h.logger.Info("team created", slog.Any("team", teamResp))
	writeJSON(w, http.StatusCreated, teamResp)
}

// GetHandler получает команду по названию (GET /team/get?team_name=...)
func (h *Handler) GetHandler(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		h.logger.Warn("GetHandler missing team_name", slog.String("remote", r.RemoteAddr))
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "team_name is required")
		return
	}

	h.logger.Info("GetHandler called", slog.String("team_name", teamName))
	teamResp, err := h.service.GetTeam(teamName)
	if err != nil {
		h.logger.Error("GetTeam failed", slog.Any("err", err))
		code := service.ParseCodeFromError(err)
		status := getStatusByCode(code)
		if er, ok := err.(*models.ErrorResponse); ok {
			writeJSON(w, status, er)
			return
		}
		writeError(w, status, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, teamResp)
}

// SetIsActiveHandler изменяет статус активности пользователя (POST /users/setIsActive)
func (h *Handler) SetIsActiveHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("SetIsActiveHandler called", slog.String("remote", r.RemoteAddr))

	var req models.SetUserActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("invalid request body in SetIsActiveHandler", slog.Any("err", err))
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	userResp, err := h.service.SetUserActive(req.UserID, req.IsActive)
	if err != nil {
		h.logger.Error("SetUserActive failed", slog.Any("err", err), slog.String("user_id", req.UserID))
		code := service.ParseCodeFromError(err)
		status := getStatusByCode(code)
		if er, ok := err.(*models.ErrorResponse); ok {
			writeJSON(w, status, er)
			return
		}
		writeError(w, status, "INTERNAL_ERROR", err.Error())
		return
	}

	h.logger.Info("user active state changed", slog.String("user_id", req.UserID), slog.Bool("is_active", req.IsActive))
	writeJSON(w, http.StatusOK, userResp)
}

// GetReviewHandler получает список PR'ов для ревью пользователя (GET /users/getReview?user_id=...)
func (h *Handler) GetReviewHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		h.logger.Warn("GetReviewHandler missing user_id", slog.String("remote", r.RemoteAddr))
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "user_id is required")
		return
	}

	h.logger.Info("GetReviewHandler called", slog.String("user_id", userID))
	resp, err := h.service.GetReviewPRs(userID)
	if err != nil {
		h.logger.Error("GetReviewPRs failed", slog.Any("err", err))
		code := service.ParseCodeFromError(err)
		status := getStatusByCode(code)
		if er, ok := err.(*models.ErrorResponse); ok {
			writeJSON(w, status, er)
			return
		}
		writeError(w, status, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateHandler создаёт новый pull request (POST /pullRequest/create)
func (h *Handler) CreateHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("CreateHandler called", slog.String("remote", r.RemoteAddr))

	var req models.CreatePullRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("invalid request body in CreateHandler", slog.Any("err", err))
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	prResp, err := h.service.CreatePullRequest(&req)
	if err != nil {
		h.logger.Error("CreatePullRequest failed", slog.Any("err", err))
		code := service.ParseCodeFromError(err)
		status := getStatusByCode(code)
		if er, ok := err.(*models.ErrorResponse); ok {
			writeJSON(w, status, er)
			return
		}
		writeError(w, status, "INTERNAL_ERROR", err.Error())
		return
	}

	h.logger.Info("pull request created", slog.Any("pr", prResp))
	writeJSON(w, http.StatusCreated, prResp)
}

// MergeHandler объединяет pull request (POST /pullRequest/merge)
func (h *Handler) MergeHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("MergeHandler called", slog.String("remote", r.RemoteAddr))

	var req models.MergePullRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("invalid request body in MergeHandler", slog.Any("err", err))
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	prResp, err := h.service.MergePullRequest(req.PullRequestID)
	if err != nil {
		h.logger.Error("MergePullRequest failed", slog.Any("err", err), slog.String("pr_id", req.PullRequestID))
		code := service.ParseCodeFromError(err)
		status := getStatusByCode(code)
		if er, ok := err.(*models.ErrorResponse); ok {
			writeJSON(w, status, er)
			return
		}
		writeError(w, status, "INTERNAL_ERROR", err.Error())
		return
	}

	h.logger.Info("pull request merged", slog.String("pr_id", req.PullRequestID))
	writeJSON(w, http.StatusOK, prResp)
}

// ReassignHandler переназначает рецензента для pull request (POST /pullRequest/reassign)
func (h *Handler) ReassignHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("ReassignHandler called", slog.String("remote", r.RemoteAddr))

	var req models.ReassignPullRequestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("invalid request body in ReassignHandler", slog.Any("err", err))
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	prResp, replacedBy, err := h.service.ReassignReviewer(req.PullRequestID, req.OldUserID)
	if err != nil {
		h.logger.Error("ReassignReviewer failed", slog.Any("err", err))
		code := service.ParseCodeFromError(err)
		status := getStatusByCode(code)
		if er, ok := err.(*models.ErrorResponse); ok {
			writeJSON(w, status, er)
			return
		}
		writeError(w, status, "INTERNAL_ERROR", err.Error())
		return
	}

	resp := models.ReassignPullRequestResponse{
		PR:         *prResp,
		ReplacedBy: replacedBy,
	}
	h.logger.Info("pull request reassigned", slog.String("pr_id", req.PullRequestID), slog.String("replaced_by", replacedBy))
	writeJSON(w, http.StatusOK, resp)
}

// StatsUsersHandler handles GET /stats/users
func (h *Handler) StatsUsersHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("StatsUsersHandler called", slog.String("remote", r.RemoteAddr))

	resp, err := h.service.GetUserStats()
	if err != nil {
		h.logger.Error("AddTeam failed", slog.Any("err", err))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(models.ErrorResponse{
			ErrDetail: models.ErrorDetail{
				Code:    "INTERNAL_ERROR",
				Message: err.Error(),
			},
		})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
