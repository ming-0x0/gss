package handler

import (
	"gss/internal/infrastructure/logger"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Details any `json:"details,omitempty"`
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
