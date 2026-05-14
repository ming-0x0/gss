package auth

import (
	"gss/internal/delivery/http/dto"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if !h.BindAndValidate(c, &req) {
		return
	}

	h.OK(c, dto.LoginResponse{
		AccessToken:  "access-token-placeholder",
		RefreshToken: "refresh-token-placeholder",
	})
}
