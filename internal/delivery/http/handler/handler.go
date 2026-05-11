package handler

import (
	"errors"
	"net/http"

	"gss/internal/errcode"
	"gss/internal/infrastructure/logger"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type Response struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    any            `json:"data,omitempty"`
	Error   *ErrorResponse `json:"error,omitempty"`
}

type ErrorResponse struct {
	Details any `json:"details,omitempty"`
}

type Handler struct {
	level logger.Level
}

func NewHandler(level string) *Handler {
	return &Handler{
		level: logger.GetLevelFromString(level),
	}
}

func (h *Handler) isDebugMode() bool {
	return logger.Debug.GTE(h.level)
}

// OK sends a successful response.
func (h *Handler) OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Code:    errcode.OK.Int(),
		Message: "success",
		Data:    data,
	})
}

// Created sends a 201 response.
func (h *Handler) Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{
		Code:    errcode.OK.Int(),
		Message: "created",
		Data:    data,
	})
}

// HandleError maps domain errors to HTTP responses.
func (h *Handler) HandleError(c *gin.Context, err error) {
	var domainErr *errcode.Error
	if !errors.As(err, &domainErr) {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    errcode.Internal.Int(),
			Message: "Internal Server Error",
		})
		return
	}

	status, msg := mapErrorCodeToHTTP(domainErr.Code())

	resp := Response{
		Code:    domainErr.Code().Int(),
		Message: msg,
	}

	if h.isDebugMode() {
		resp.Error = &ErrorResponse{Details: domainErr.Error()}
	}

	c.JSON(status, resp)
}

// BindAndValidate binds the request body and returns validation errors if any.
func (h *Handler) BindAndValidate(c *gin.Context, req any) bool {
	if err := c.ShouldBindJSON(req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			details := make([]map[string]string, 0, len(ve))
			for _, fe := range ve {
				details = append(details, map[string]string{
					"field":   fe.Field(),
					"message": fe.Error(),
				})
			}
			c.JSON(http.StatusBadRequest, Response{
				Code:    errcode.InvalidArgument.Int(),
				Message: "Validation failed",
				Error:   &ErrorResponse{Details: details},
			})
		} else {
			c.JSON(http.StatusBadRequest, Response{
				Code:    errcode.InvalidArgument.Int(),
				Message: "Invalid request body",
			})
		}
		return false
	}
	return true
}

func mapErrorCodeToHTTP(code errcode.ErrorCode) (int, string) {
	switch code {
	case errcode.InvalidArgument:
		return http.StatusBadRequest, "Bad Request"
	case errcode.Unauthenticated:
		return http.StatusUnauthorized, "Unauthorized"
	case errcode.PermissionDenied:
		return http.StatusForbidden, "Forbidden"
	case errcode.NotFound:
		return http.StatusNotFound, "Not Found"
	case errcode.AlreadyExists:
		return http.StatusConflict, "Conflict"
	case errcode.ResourceExhausted:
		return http.StatusTooManyRequests, "Too Many Requests"
	default:
		return http.StatusInternalServerError, "Internal Server Error"
	}
}
