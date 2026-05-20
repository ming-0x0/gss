package http

import (
	"gss/configs"
	"gss/internal/errcode"
	"gss/internal/infrastructure/logger"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SuccessResponse[T any] struct {
	Code    errcode.ErrorCode `json:"code"`
	Message string            `json:"message"`
	Data    T                 `json:"data,omitempty"`
}

type ErrorResponse struct {
	Code    errcode.ErrorCode `json:"code"`
	Message string            `json:"message"`
	Error   *ErrorDetails     `json:"error,omitempty"`
}

type ErrorDetails struct {
	Details string `json:"details,omitempty"`
}

func isDebugMode() bool {
	return logger.Debug.GTE(logger.GetLevelFromString(configs.Get().Logger.Level))
}

func OK[T any](c *gin.Context, msg string, data T) {
	c.JSON(http.StatusOK, SuccessResponse[T]{
		Code:    errcode.OK,
		Message: msg,
		Data:    data,
	})
}

func Error(c *gin.Context, err error, customMsg ...string) {
	code, msg, details := errcode.FromError(err)

	if !code.IsServerError() && len(customMsg) > 0 && customMsg[0] != "" {
		msg = customMsg[0]
	}

	resp := ErrorResponse{
		Code:    code,
		Message: msg,
	}

	if isDebugMode() && details != "" {
		resp.Error = &ErrorDetails{Details: details}
	}

	c.AbortWithStatusJSON(http.StatusOK, resp)
}

func InternalServerError(c *gin.Context, err error) {
	Error(c, errcode.WithCause(errcode.Internal, err))
}

func BadRequest(c *gin.Context, err error) {
	Error(c, errcode.WithCause(errcode.InvalidArgument, err))
}
