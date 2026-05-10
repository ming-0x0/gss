package orm

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gss/internal/infrastructure/logger"

	"github.com/uptrace/bun"
)

type Logger struct {
	logger            *logger.Logger
	level             logger.Level
	slowThreshold     time.Duration
	ignoreNoRowsError bool
}

type Option func(*Logger)

func WithLogger(logger *logger.Logger) Option {
	return func(l *Logger) {
		l.logger = logger
	}
}

func WithSlowThreshold(slowThreshold time.Duration) Option {
	return func(l *Logger) {
		l.slowThreshold = slowThreshold
	}
}

func WithIgnoreNoRowsError() Option {
	return func(l *Logger) {
		l.ignoreNoRowsError = true
	}
}

func WithLevel(level string) Option {
	return func(l *Logger) {
		l.level = logger.GetLevelFromString(level)
	}
}

func New(opts ...Option) *Logger {
	l := &Logger{
		level:             logger.Info,
		slowThreshold:     200 * time.Millisecond,
		ignoreNoRowsError: false,
	}
	for _, opt := range opts {
		opt(l)
	}

	if l.logger == nil {
		l.logger = logger.New()
	}

	return l
}

var _ bun.QueryHook = (*Logger)(nil)

func (l *Logger) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	return ctx
}

func (l *Logger) AfterQuery(ctx context.Context, event *bun.QueryEvent) {
	elapsed := time.Since(event.StartTime)

	keyVals := []any{
		"durations", elapsed.String(),
		"sql", event.Query,
	}

	if event.Result != nil {
		rows, err := event.Result.RowsAffected()
		if err != nil {
			keyVals = append(keyVals, "rows", "-")
		} else {
			keyVals = append(keyVals, "rows", rows)
		}
	}

	if event.Err != nil {
		keyVals = append(keyVals, "error", event.Err.Error())
	}

	keyVals = append(keyVals, "service", "database")

	switch {
	case event.Err != nil && (!errors.Is(event.Err, sql.ErrNoRows) || !l.ignoreNoRowsError):
		if logger.Error.GTE(l.level) {
			l.logger.ErrorContext(ctx, "SQL Query failed", keyVals...)
		}
	case l.slowThreshold != 0 && elapsed > l.slowThreshold:
		if logger.Warn.GTE(l.level) {
			l.logger.WarnContext(ctx, "Performed SLOW SQL Query", keyVals...)
		}
	default:
		if logger.Info.GTE(l.level) {
			l.logger.InfoContext(ctx, "Performed SQL Query", keyVals...)
		}
	}
}
