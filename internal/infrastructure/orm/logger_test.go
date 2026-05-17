package orm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"gss/internal/infrastructure/logger"

	"github.com/stretchr/testify/assert"
	gormlogger "gorm.io/gorm/logger"
)

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	
	appLogger := logger.New(
		logger.WithLevel("debug"),
		logger.WithWriter(&buf),
	)

	l := New(
		WithHandler(appLogger.Handler()),
		WithSlowThreshold(50*time.Millisecond),
	)

	ctx := context.Background()

	t.Run("Info log", func(t *testing.T) {
		buf.Reset()
		l.LogMode(gormlogger.Info).Info(ctx, "hello %s", "world")

		var logEntry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &logEntry)
		assert.NoError(t, err)

		assert.Equal(t, "hello world", logEntry["msg"])
		assert.Equal(t, "info", logEntry["level"])
		assert.NotEmpty(t, logEntry["file"])
		assert.NotEmpty(t, logEntry["func"])
	})

	t.Run("Trace normal query", func(t *testing.T) {
		buf.Reset()
		l.LogMode(gormlogger.Info).Trace(ctx, time.Now(), func() (string, int64) {
			return "SELECT * FROM users", 5
		}, nil)

		var logEntry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &logEntry)
		assert.NoError(t, err)

		assert.Equal(t, "database query", logEntry["msg"])
		assert.Equal(t, "info", logEntry["level"])
		assert.Equal(t, "SELECT * FROM users", logEntry["sql"])
		assert.Equal(t, float64(5), logEntry["rows"])
		assert.Equal(t, "database", logEntry["service"])
		assert.NotEmpty(t, logEntry["duration"])
		assert.NotEmpty(t, logEntry["file"])
		assert.NotEmpty(t, logEntry["func"])
	})

	t.Run("Trace slow query", func(t *testing.T) {
		buf.Reset()
		l.LogMode(gormlogger.Warn).Trace(ctx, time.Now().Add(-100*time.Millisecond), func() (string, int64) {
			return "SELECT * FROM large_table", 1000
		}, nil)

		var logEntry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &logEntry)
		assert.NoError(t, err)

		assert.True(t, strings.HasPrefix(logEntry["msg"].(string), "SLOW SQL"))
		assert.Equal(t, "warn", logEntry["level"])
		assert.Equal(t, "SELECT * FROM large_table", logEntry["sql"])
		assert.Equal(t, "database", logEntry["service"])
		assert.NotEmpty(t, logEntry["file"])
		assert.NotEmpty(t, logEntry["func"])
	})

	t.Run("Trace query error", func(t *testing.T) {
		buf.Reset()
		dbErr := errors.New("connection reset")
		l.LogMode(gormlogger.Error).Trace(ctx, time.Now(), func() (string, int64) {
			return "SELECT * FROM missing_table", -1
		}, dbErr)

		var logEntry map[string]interface{}
		err := json.Unmarshal(buf.Bytes(), &logEntry)
		assert.NoError(t, err)

		assert.Equal(t, "database error", logEntry["msg"])
		assert.Equal(t, "error", logEntry["level"])
		assert.Equal(t, "SELECT * FROM missing_table", logEntry["sql"])
		assert.Equal(t, "connection reset", logEntry["error"])
		assert.Equal(t, "database", logEntry["service"])
		assert.NotEmpty(t, logEntry["file"])
		assert.NotEmpty(t, logEntry["func"])
	})
}

func BenchmarkLoggerTrace(b *testing.B) {
	// 1. Benchmark when logging is disabled (Silent mode)
	b.Run("Disabled", func(b *testing.B) {
		appLogger := logger.New(logger.WithLevel("error"))
		l := New(
			WithHandler(appLogger.Handler()),
		).LogMode(gormlogger.Silent)

		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Trace(ctx, time.Now(), func() (string, int64) {
				return "SELECT * FROM users", 5
			}, nil)
		}
	})

	// 2. Benchmark when logging is enabled but level is filtered out (Info query when level is Warn)
	b.Run("Filtered", func(b *testing.B) {
		appLogger := logger.New(logger.WithLevel("warn"))
		l := New(
			WithHandler(appLogger.Handler()),
		).LogMode(gormlogger.Warn)

		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Trace(ctx, time.Now(), func() (string, int64) {
				return "SELECT * FROM users", 5
			}, nil)
		}
	})

	// 3. Benchmark when logging is enabled and active
	b.Run("Enabled", func(b *testing.B) {
		appLogger := logger.New(logger.WithLevel("debug"), logger.WithWriter(io.Discard))
		l := New(
			WithHandler(appLogger.Handler()),
		).LogMode(gormlogger.Info)

		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Trace(ctx, time.Now(), func() (string, int64) {
				return "SELECT * FROM users", 5
			}, nil)
		}
	})
}
