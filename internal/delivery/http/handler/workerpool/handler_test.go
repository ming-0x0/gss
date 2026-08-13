package workerpool_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gss/internal/delivery/http/handler"
	wpHandler "gss/internal/delivery/http/handler/workerpool"
	"gss/internal/infrastructure/logger"
	"gss/pkg/workerpool"

	"github.com/gin-gonic/gin"
	"github.com/oaswrap/spec/adapter/ginopenapi"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *workerpool.Pool) {
	gin.SetMode(gin.TestMode)

	wp, err := workerpool.New(
		workerpool.WithWorkers(2),
		workerpool.WithQueueSize(5),
	)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	l := logger.New()
	baseHandler := handler.NewHandler("debug")
	h := wpHandler.NewHandler(baseHandler, wp, l)

	engine := gin.New()
	router := ginopenapi.NewRouter(engine)
	h.RegisterRoutes(router)

	return engine, wp
}

func TestWorkerPoolEndpoints(t *testing.T) {
	engine, wp := setupTestRouter(t)
	defer wp.Stop()

	t.Run("POST /api/v1/workerpool/submit", func(t *testing.T) {
		body := map[string]any{
			"task_name": "test_async_job",
			"delay_ms":  10,
		}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/workerpool/submit", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /api/v1/workerpool/submit-result", func(t *testing.T) {
		body := map[string]any{
			"number": 9,
		}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/workerpool/submit-result", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]any)
		if data["square"].(float64) != 81 {
			t.Errorf("expected square 81, got %v", data["square"])
		}
	})

	t.Run("POST /api/v1/workerpool/panic-demo (safe-wrapper)", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/workerpool/panic-demo?type=safe-wrapper", nil)
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}

		time.Sleep(20 * time.Millisecond)
	})

	t.Run("POST /api/v1/workerpool/panic-demo (result-panic)", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/workerpool/panic-demo?type=result-panic", nil)
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 InternalServerError, got %d", w.Code)
		}
	})

	t.Run("POST /api/v1/workerpool/panic-demo (standard)", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/workerpool/panic-demo?type=standard", nil)
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}

		time.Sleep(20 * time.Millisecond)
	})

	t.Run("GET /api/v1/workerpool/stats", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/workerpool/stats", nil)
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]any)

		if data["workers"].(float64) != 2 {
			t.Errorf("expected 2 workers, got %v", data["workers"])
		}
	})
}
