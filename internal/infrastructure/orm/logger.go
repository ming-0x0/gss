package orm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

const (
	Info  = slog.Level(1)
	Warn  = slog.Level(2)
	Error = slog.Level(3)
)

type Logger struct {
	handler                   slog.Handler
	level                     gormlogger.LogLevel
	slowThreshold             time.Duration
	ignoreRecordNotFoundError bool
}

type Option func(*Logger)

func WithHandler(handler slog.Handler) Option {
	return func(l *Logger) { l.handler = handler }
}

func WithSlowThreshold(d time.Duration) Option {
	return func(l *Logger) { l.slowThreshold = d }
}

func WithIgnoreRecordNotFoundError(ignore bool) Option {
	return func(l *Logger) { l.ignoreRecordNotFoundError = ignore }
}

func WithLevel(level string) Option {
	return func(l *Logger) {
		switch level {
		case "silent":
			l.level = gormlogger.Silent
		case "error":
			l.level = gormlogger.Error
		case "warn":
			l.level = gormlogger.Warn
		case "info":
			l.level = gormlogger.Info
		}
	}
}

func New(opts ...Option) *Logger {
	l := &Logger{
		level:                     gormlogger.Info,
		slowThreshold:             200 * time.Millisecond,
		ignoreRecordNotFoundError: true,
	}
	for _, opt := range opts {
		opt(l)
	}
	if l.handler == nil {
		l.handler = slog.Default().Handler()
	}

	return l
}

func (l *Logger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	clone := *l
	clone.level = level
	return &clone
}

func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Info {
		l.log(ctx, Info, msg, args...)
	}
}

func (l *Logger) Warn(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Warn {
		l.log(ctx, Warn, msg, args...)
	}
}

func (l *Logger) Error(ctx context.Context, msg string, args ...any) {
	if l.level >= gormlogger.Error {
		l.log(ctx, Error, msg, args...)
	}
}

func (l *Logger) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if !l.handler.Enabled(ctx, level) {
		return
	}
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])
	r := slog.NewRecord(time.Now(), level, fmt.Sprintf(msg, args...), pcs[0])
	_ = l.handler.Handle(ctx, r)
}

func (l *Logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)

	switch {
	case err != nil && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.ignoreRecordNotFoundError):
		if l.level < gormlogger.Error {
			return
		}
		sql, rows := fc()
		l.trace(ctx, Error, "SQL Query failed", elapsed, sql, rows, err)

	case l.slowThreshold != 0 && elapsed > l.slowThreshold:
		if l.level < gormlogger.Warn {
			return
		}
		sql, rows := fc()
		l.trace(ctx, Warn, "Performed SLOW SQL Query", elapsed, sql, rows, nil)

	default:
		if l.level < gormlogger.Info {
			return
		}
		sql, rows := fc()
		l.trace(ctx, Info, "Performed SQL Query", elapsed, sql, rows, nil)
	}
}

func (l *Logger) trace(ctx context.Context, level slog.Level, msg string, elapsed time.Duration, sql string, rows int64, err error) {
	if !l.handler.Enabled(ctx, level) {
		return
	}

	var pcs [1]uintptr
	runtime.Callers(3, pcs[:])

	attrs := []slog.Attr{
		slog.Duration("duration", elapsed),
		slog.String("sql", sql),
		slog.String("service", "database"),
		slog.String("file", utils.FileWithLineNum()),
	}
	if rows >= 0 {
		attrs = append(attrs, slog.Int64("rows", rows))
	} else {
		attrs = append(attrs, slog.String("rows", "-"))
	}

	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}

	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.AddAttrs(attrs...)
	_ = l.handler.Handle(ctx, r)
}
