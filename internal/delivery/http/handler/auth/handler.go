package auth

import (
	"gss/internal/delivery/http/handler"

	"gss/internal/delivery/http/dto"
	"gss/internal/domain"
	"gss/internal/infrastructure/logger"
	"net/http"
	"time"

	"github.com/oaswrap/spec/adapter/ginopenapi"
	"github.com/oaswrap/spec/option"
)

type Handler struct {
	*handler.Handler
	userRepo       domain.UserRepositoryInterface
	contextTimeout time.Duration
	logger         *logger.Logger
}

func NewHandler(
	base *handler.Handler,
	userRepo domain.UserRepositoryInterface,
	contextTimeout time.Duration,
	logger *logger.Logger,
) *Handler {
	return &Handler{
		Handler:        base,
		userRepo:       userRepo,
		contextTimeout: contextTimeout,
		logger:         logger,
	}
}

func (h *Handler) RegisterRoutes(r ginopenapi.Router) {
	auth := r.Group("/auth")
	v1 := auth.Group("/v1")

	v1.POST("/login", h.Login).With(
		option.Tags("auth"),
		option.Summary("Login"),
		option.Request(new(dto.LoginRequest)),
		option.Response(http.StatusOK, new(dto.LoginResponse)),
	)
}

