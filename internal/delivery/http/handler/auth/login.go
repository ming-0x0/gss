package auth

import (
	"context"

	"gss/internal/delivery/http/dto"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if !h.BindAndValidate(c, &req) {
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), h.contextTimeout)
	defer cancel()

	user, err := h.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		h.HandleError(c, err)
		return
	}

	// TODO: verify password, generate tokens
	_ = user

	h.OK(c, dto.LoginResponse{
		AccessToken:  "access-token-placeholder",
		RefreshToken: "refresh-token-placeholder",
	})
}
