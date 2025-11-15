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
