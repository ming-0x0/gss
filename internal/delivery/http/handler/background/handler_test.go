package background_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gss/internal/delivery/http/handler"
	bgHandler "gss/internal/delivery/http/handler/background"
	"gss/internal/infrastructure/logger"
	"gss/pkg/background"

	"github.com/gin-gonic/gin"
	"github.com/oaswrap/spec/adapter/ginopenapi"
)

func setupTestRouter(t *testing.T) (*gin.Engine, *background.Runner) {
	gin.SetMode(gin.TestMode)

	bg, err := background.New(
		background.WithWorkers(2),
		background.WithQueueSize(5),
	)
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	l := logger.New()
	baseHandler := handler.NewHandler("debug")
	h := bgHandler.NewHandler(baseHandler, bg, l)

	engine := gin.New()
	router := ginopenapi.NewRouter(engine)
	h.RegisterRoutes(router)

	return engine, bg
}

func TestBackgroundEndpoints(t *testing.T) {
	engine, bg := setupTestRouter(t)
	defer bg.Stop()

	t.Run("POST /api/v1/background/submit", func(t *testing.T) {
		body := map[string]any{
			"task_name": "test_async_job",
			"delay_ms":  10,
		}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/background/submit", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /api/v1/background/submit-result", func(t *testing.T) {
		body := map[string]any{
			"number": 9,
		}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/background/submit-result", bytes.NewBuffer(jsonBytes))
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

	t.Run("POST /api/v1/background/panic-demo (safe-wrapper)", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/background/panic-demo?type=safe-wrapper", nil)
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}

		time.Sleep(20 * time.Millisecond)
	})

	t.Run("POST /api/v1/background/panic-demo (result-panic)", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/background/panic-demo?type=result-panic", nil)
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 InternalServerError, got %d", w.Code)
		}
	})

	t.Run("POST /api/v1/background/panic-demo (standard)", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/background/panic-demo?type=standard", nil)
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", w.Code)
		}

		time.Sleep(20 * time.Millisecond)
	})

	t.Run("GET /api/v1/background/stats", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/background/stats", nil)
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
