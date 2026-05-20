package auth

import (
	"gss/internal/delivery/http"
	"gss/internal/delivery/http/dto"

	"github.com/gin-gonic/gin"
)

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		http.BadRequest(c, err)
		return
	}

	http.OK(c, "Login success", dto.LoginResponse{
		AccessToken:  "access_token",
		RefreshToken: "refresh_token",
	})
}
