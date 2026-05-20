package auth

import (
	"gss/internal/delivery/http"
	"gss/internal/delivery/http/dto"
	"gss/internal/domain"
	"gss/internal/infrastructure/logger"

	"github.com/oaswrap/spec/adapter/ginopenapi"
	"github.com/oaswrap/spec/option"
)

type AuthHandler struct {
	userRepo domain.UserRepositoryInterface
	logger   *logger.Logger
}

func NewAuthHandler(
	userRepo domain.UserRepositoryInterface,
	logger *logger.Logger,
) *AuthHandler {
	return &AuthHandler{
		userRepo: userRepo,
		logger:   logger,
	}
}

func (h *AuthHandler) RegisterRoutes(r ginopenapi.Router) {
	v1 := r.Group("/v1")

	auth := v1.Group("/auth")

	auth.POST("/login", h.Login).With(
		option.Tags("auth"),
		option.Summary("Login"),
		option.Request(new(dto.LoginRequest)),
		option.Response(200, new(http.SuccessResponse[dto.LoginResponse])),
	)
}
