package http

import (
	"net/http"

	"gss/configs"
	"gss/internal/errcode"
	"gss/internal/infrastructure/logger"

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
	return logger.Debug.GTE(
		logger.GetLevelFromString(configs.Get().Logger.Level),
	)
}

func OK[T any](c *gin.Context, message string, data T) {
	c.JSON(http.StatusOK, SuccessResponse[T]{
		Code:    errcode.OK,
		Message: message,
		Data:    data,
	})
}

func abort(c *gin.Context, overrideMessage string, err error) {
	code, message, details := errcode.FromError(err)

	if !code.IsServerError() && overrideMessage != "" {
		message = overrideMessage
	}

	resp := ErrorResponse{
		Code:    code,
		Message: message,
	}

	if isDebugMode() && details != "" {
		resp.Error = &ErrorDetails{
			Details: details,
		}
	}

	c.AbortWithStatusJSON(code.HTTPStatusCode(), resp)
}

func Error(c *gin.Context, message string, err error) {
	abort(c, message, err)
}

func InternalServerError(c *gin.Context, err error) {
	abort(c, "", errcode.WithCause(errcode.Internal, err))
}

func BadRequest(c *gin.Context, err error) {
	abort(c, "", errcode.WithCause(errcode.InvalidArgument, err))
}
