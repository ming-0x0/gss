package dto

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" required:"true"`
	Password string `json:"password" binding:"required" required:"true"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token" required:"true"`
	RefreshToken string `json:"refresh_token" required:"true"`
}
