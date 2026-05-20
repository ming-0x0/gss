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
	errCode, defaultMsg, details := errcode.FromError(err)

	msg := defaultMsg
	if !errCode.IsServerError() && len(customMsg) > 0 && customMsg[0] != "" {
		msg = customMsg[0]
	}

	var errDetails *ErrorDetails
	if isDebugMode() && details != "" {
		errDetails = &ErrorDetails{
			Details: details,
		}
	}

	c.JSON(http.StatusOK, ErrorResponse{
		Code:    errCode,
		Message: msg,
		Error:   errDetails,
	})
}

func InternalServerError(c *gin.Context, err error) {
	Error(c, errcode.WithMessage(errcode.Internal, "Internal Server Error", err))
}

func BadRequest(c *gin.Context, err error) {
	Error(c, errcode.WithCause(errcode.InvalidArgument, err))
}
