package background

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"gss/internal/delivery/http/handler"
	"gss/internal/infrastructure/logger"
	"gss/pkg/background"

	"github.com/gin-gonic/gin"
	"github.com/oaswrap/spec/adapter/ginopenapi"
)

type Handler struct {
	*handler.Handler
	bg     *background.Runner
	logger *logger.Logger
}

func NewHandler(baseHandler *handler.Handler, bg *background.Runner, logger *logger.Logger) *Handler {
	return &Handler{
		Handler: baseHandler,
		bg:      bg,
		logger:  logger,
	}
}

func (h *Handler) RegisterRoutes(r ginopenapi.Router) {
	group := r.Group("/api/v1/background")
	{
		group.POST("/submit", h.SubmitTask)
		group.POST("/try-submit", h.TrySubmitTask)
		group.POST("/submit-result", h.SubmitWithResultTask)
		group.POST("/panic-demo", h.PanicDemoTask)
		group.GET("/stats", h.GetStats)
	}
}

type SubmitTaskRequest struct {
	TaskName string `json:"task_name" binding:"required"`
	DelayMs  int    `json:"delay_ms"`
}

type SubmitWithResultRequest struct {
	Number int `json:"number" binding:"required"`
}

// SubmitTask queues an async task to the background runner.
func (h *Handler) SubmitTask(c *gin.Context) {
	var req SubmitTaskRequest
	if !h.BindAndValidate(c, &req) {
		return
	}

	delay := time.Duration(req.DelayMs) * time.Millisecond
	err := h.bg.Submit(c.Request.Context(), func(ctx context.Context) error {
		h.logger.Info("Starting async background task", "name", req.TaskName)
		if delay > 0 {
			time.Sleep(delay)
		}
		h.logger.Info("Completed async background task", "name", req.TaskName)
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.OK(c, gin.H{
		"message":   "Task queued successfully",
		"task_name": req.TaskName,
	})
}

// TrySubmitTask attempts to queue a task without blocking.
func (h *Handler) TrySubmitTask(c *gin.Context) {
	var req SubmitTaskRequest
	if !h.BindAndValidate(c, &req) {
		return
	}

	err := h.bg.TrySubmit(c.Request.Context(), func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	if errors.Is(err, background.ErrQueueFull) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "Task queue is full, please try again later",
		})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.OK(c, gin.H{
		"message":   "Task accepted via TrySubmit",
		"task_name": req.TaskName,
	})
}

// SubmitWithResultTask executes a task using Generics and waits for the typed result.
func (h *Handler) SubmitWithResultTask(c *gin.Context) {
	var req SubmitWithResultRequest
	if !h.BindAndValidate(c, &req) {
		return
	}

	resCh, err := background.SubmitWithResult(h.bg, c.Request.Context(), func(ctx context.Context) (int, error) {
		// Demo computation: calculate square of input number
		result := req.Number * req.Number
		return result, nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	res := <-resCh
	if res.Err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": res.Err.Error()})
		return
	}

	h.OK(c, gin.H{
		"input":  req.Number,
		"square": res.Value,
	})
}

// PanicDemoTask demonstrates different panic recovery strategies directly within the HTTP handler:
// Query parameter ?type=safe-wrapper|result-panic|standard (default: safe-wrapper)
// 1. safe-wrapper: Uses SafeSubmit wrapper to catch panic, log stack trace with request context, and convert to error.
// 2. result-panic: Catches panic inside SubmitWithResult task, logging details and returning typed error to caller channel.
// 3. standard: Delegates panic handling directly to runner level recovery.
func (h *Handler) PanicDemoTask(c *gin.Context) {
	panicType := c.DefaultQuery("type", "safe-wrapper")

	switch panicType {
	case "result-panic":
		// Pattern 1: Catching panic inside SubmitWithResult & returning panic error back over result channel
		resCh, err := background.SubmitWithResult(h.bg, c.Request.Context(), func(ctx context.Context) (res int, err error) {
			defer func() {
				if r := recover(); r != nil {
					h.logger.Error("Handler caught task panic inside SubmitWithResult",
						"panic", r,
						"stack", string(debug.Stack()),
					)
					err = fmt.Errorf("task panicked: %v", r)
				}
			}()
			// Simulate calculation panic
			panic("math computation divide by zero panic")
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		res := <-resCh
		if res.Err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Task failed due to panic",
				"details": res.Err.Error(),
			})
			return
		}
		h.OK(c, gin.H{"result": res.Value})

	case "safe-wrapper":
		// Pattern 2: Using Handler-level SafeSubmit wrapper for contextual panic recovery & stack trace logging
		err := h.SafeSubmit(c.Request.Context(), "DemoPanicTask", func(ctx context.Context) error {
			// Simulate nil pointer dereference
			var ptr *string
			_ = *ptr
			return nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Task execution failed",
				"details": err.Error(),
			})
			return
		}
		h.OK(c, gin.H{"message": "Task completed successfully"})

	default:
		// Pattern 3: Standard submission, relying on runner level panic handler
		err := h.bg.Submit(c.Request.Context(), func(ctx context.Context) error {
			panic("Standard worker panic demo string")
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		h.OK(c, gin.H{
			"message": "Panic task queued; recovered by background panic handler",
		})
	}
}

// SafeSubmit wraps a background task with handler-level panic recovery, stack trace capture, and error mapping.
func (h *Handler) SafeSubmit(ctx context.Context, taskName string, task background.Task) error {
	safeTask := func(taskCtx context.Context) (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				h.logger.Error("Handler caught panic in SafeSubmit",
					"task_name", taskName,
					"panic", r,
					"stack", string(stack),
				)
				err = fmt.Errorf("task %s panicked: %v", taskName, r)
			}
		}()
		return task(taskCtx)
	}
	return h.bg.Submit(ctx, safeTask)
}

// GetStats returns current metrics for the background runner.
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.bg.Stats()
	h.OK(c, gin.H{
		"workers":        h.bg.Size(),
		"active_workers": stats.Active,
		"queue_len":      stats.QueueLen,
		"submitted":      stats.Submitted,
		"completed":      stats.Completed,
		"failed":         stats.Failed,
	})
}
