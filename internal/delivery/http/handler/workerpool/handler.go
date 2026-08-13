package workerpool

import (
	"context"
	"errors"
	"net/http"
	"time"

	"gss/internal/delivery/http/handler"
	"gss/internal/infrastructure/logger"
	"gss/pkg/workerpool"

	"github.com/gin-gonic/gin"
	"github.com/oaswrap/spec/adapter/ginopenapi"
)

type Handler struct {
	*handler.Handler
	wp     *workerpool.Pool
	logger *logger.Logger
}

func NewHandler(baseHandler *handler.Handler, wp *workerpool.Pool, logger *logger.Logger) *Handler {
	return &Handler{
		Handler: baseHandler,
		wp:      wp,
		logger:  logger,
	}
}

func (h *Handler) RegisterRoutes(r ginopenapi.Router) {
	group := r.Group("/api/v1/workerpool")
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

// SubmitTask queues an async task to the worker pool.
func (h *Handler) SubmitTask(c *gin.Context) {
	var req SubmitTaskRequest
	if !h.BindAndValidate(c, &req) {
		return
	}

	delay := time.Duration(req.DelayMs) * time.Millisecond
	err := h.wp.Submit(c.Request.Context(), func(ctx context.Context) error {
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

	err := h.wp.TrySubmit(c.Request.Context(), func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	if errors.Is(err, workerpool.ErrQueueFull) {
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

	resCh, err := workerpool.SubmitWithResult(h.wp, c.Request.Context(), func(ctx context.Context) (int, error) {
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

// PanicDemoTask submits a task that panics to showcase worker panic recovery.
func (h *Handler) PanicDemoTask(c *gin.Context) {
	err := h.wp.Submit(c.Request.Context(), func(ctx context.Context) error {
		panic("Simulated worker panic demo")
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.OK(c, gin.H{
		"message": "Panic task queued; worker pool panic recovery will catch it safely without crashing",
	})
}

// GetStats returns current metrics for the worker pool.
func (h *Handler) GetStats(c *gin.Context) {
	h.OK(c, gin.H{
		"workers":        h.wp.Workers(),
		"active_workers": h.wp.ActiveWorkers(),
		"queue_len":      h.wp.QueueLen(),
		"submitted":      h.wp.Submitted(),
		"completed":      h.wp.Completed(),
		"failed":         h.wp.Failed(),
	})
}
