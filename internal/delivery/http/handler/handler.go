package handler

import (
	"gss/internal/infrastructure/logger"
)

type BaseResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    any            `json:"data,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

type ErrorResponse struct {
	Details any `json:"details"`
}

type Handler struct {
	level logger.Level
}

func NewHandler(level string) *Handler {
	return &Handler{
		level: logger.GetLevelFromString(level),
	}
}

func (h *Handler) isDebugMode() bool {
	return logger.Debug.GTE(h.level)
}
