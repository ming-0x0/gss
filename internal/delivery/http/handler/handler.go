package handler

import (
	"errors"
	"net/http"

	"gss/configs"
	"gss/internal/errcode"
	"gss/internal/infrastructure/logger"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Details any `json:"details,omitempty"`
}

type BaseHandler struct{}

func NewBaseHandler() *BaseHandler {
	return &BaseHandler{}
}

func (h *BaseHandler) isDebugMode() bool {
	return logger.Debug.GTE(logger.GetLevelFromString(configs.Get().Logger.Level))
}

func (h *BaseHandler) OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func (h *BaseHandler) Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

func (h *BaseHandler) Err(c *gin.Context, err error) {
	status, resp := h.buildErrorResponse(err)
	c.JSON(status, resp)
}

func (h *BaseHandler) buildErrorResponse(err error) (int, Response) {
	var e *errcode.Error
	if !errors.As(err, &e) {
		return http.StatusInternalServerError, Response{
			Code:    errcode.Internal.Int(),
			Message: "internal error",
		}
	}

	status := mapCodeToHTTPStatus(e.Code())
	resp := Response{
		Code:    e.Code().Int(),
		Message: e.Message(),
	}

	if h.isDebugMode() && e.Unwrap() != nil {
		resp.Error = &Error{Details: e.Unwrap().Error()}
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
