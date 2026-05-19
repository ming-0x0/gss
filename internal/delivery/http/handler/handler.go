package handler

import (
	"errors"
	"net/http"

	"gss/configs"
	"gss/internal/errcode"
	"gss/internal/infrastructure/logger"

	"github.com/gin-gonic/gin"
)

type SuccessResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data,omitempty"`
}

type ErrorResponse struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Error   *ErrorDetail `json:"error,omitempty"`
}

type ErrorDetail struct {
	Details string `json:"details,omitempty"`
}

type BaseHandler struct{}

func NewBaseHandler() *BaseHandler {
	return &BaseHandler{}
}

func (h *BaseHandler) isDebugMode() bool {
	return logger.Debug.GTE(logger.GetLevelFromString(configs.Get().Logger.Level))
}

func OK[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, SuccessResponse[T]{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func Created[T any](c *gin.Context, data T) {
	c.JSON(http.StatusCreated, SuccessResponse[T]{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

func (h *BaseHandler) Err(c *gin.Context, err error) {
	status, resp := h.buildErrorResponse(err)
	c.JSON(status, resp)
}

func (h *BaseHandler) buildErrorResponse(err error) (int, ErrorResponse) {
	var e *errcode.Error
	if !errors.As(err, &e) {
		return http.StatusInternalServerError, ErrorResponse{
			Code:    errcode.Internal.Int(),
			Message: "internal error",
		}
	}

	status := mapCodeToHTTPStatus(e.Code())
	resp := ErrorResponse{
		Code:    e.Code().Int(),
		Message: e.Message(),
	}

	if h.isDebugMode() && e.Unwrap() != nil {
		resp.Error = &ErrorDetail{Details: e.Unwrap().Error()}
	}

	return status, resp
}

func mapCodeToHTTPStatus(code errcode.ErrorCode) int {
	switch code {
	case errcode.InvalidArgument:
		return http.StatusBadRequest
	case errcode.Unauthenticated:
		return http.StatusUnauthorized
	case errcode.PermissionDenied:
		return http.StatusForbidden
	case errcode.NotFound:
		return http.StatusNotFound
	case errcode.AlreadyExists:
		return http.StatusConflict
	case errcode.ResourceExhausted:
		return http.StatusTooManyRequests
	case errcode.DeadlineExceeded:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
