package auth

import (
	"gss/internal/delivery/http"
	"gss/internal/delivery/http/dto"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		http.BadRequest(c, err)
		return
	}

	_, err = h.userRepo.FindByEmail(c.Request.Context(), req.Email)
	if err != nil {
		http.Error(c, err, "Email not found")
		return
	}

	http.OK(c, "Login success", dto.LoginResponse{
		AccessToken:  "access_token",
		RefreshToken: "refresh_token",
	})
}


