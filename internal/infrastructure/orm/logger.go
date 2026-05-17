package orm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"gss/internal/infrastructure/logger"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var internalPrefixes = [...]string{
	"gorm.io/",
	"gss/internal/infrastructure/orm.",
	"gss/internal/infrastructure/orm/",
	"database/sql.",
	"runtime.",
}

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

func New(opts ...Option) *Logger {
	l := &Logger{
		level:                     gormlogger.Warn,
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

func (l *Logger) getCaller(skip int) (file string, line int, fn string, pc uintptr) {
	var pcs [16]uintptr
	n := runtime.Callers(skip, pcs[:])
	if n == 0 {
		return
	}
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !l.isInternal(frame.Function) {
			return frame.File, frame.Line, frame.Function, frame.PC
		}
		if !more {
			break
		}
	}
	return
}

func (l *Logger) isInternal(fn string) bool {
	for _, prefix := range internalPrefixes {
		if len(fn) >= len(prefix) && fn[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func (l *Logger) emit(ctx context.Context, level slog.Level, msg string, pc uintptr, attrs []slog.Attr) {
	if !l.handler.Enabled(ctx, level) {
		return
	}
	r := slog.NewRecord(time.Now(), level, msg, pc)
	if len(attrs) > 0 {
		r.AddAttrs(attrs...)
	}
	_ = l.handler.Handle(ctx, r)
}

func (l *Logger) log(ctx context.Context, level slog.Level, format string, args ...any) {
	if !l.handler.Enabled(ctx, level) {
		return
	}
	file, line, fn, pc := l.getCaller(4)
	msg := fmt.Sprintf(format, args...)

	attrs := make([]slog.Attr, 0, 2)
	if file != "" {
		attrs = append(attrs,
			slog.String("file", fmt.Sprintf("%s:%d", file, line)),
			slog.String("func", fn),
		)
	}
	l.emit(ctx, level, msg, pc, attrs)
}

func (l *Logger) Info(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= gormlogger.Info {
		l.log(ctx, logger.Info.ToSlogLevel(), msg, args...)
	}
}

func (l *Logger) Warn(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= gormlogger.Warn {
		l.log(ctx, logger.Warn.ToSlogLevel(), msg, args...)
	}
}

func (l *Logger) Error(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= gormlogger.Error {
		l.log(ctx, logger.Error.ToSlogLevel(), msg, args...)
	}
}

func (l *Logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)

	isRecordNotFound := err != nil && errors.Is(err, gorm.ErrRecordNotFound)
	isRealError := err != nil && !(isRecordNotFound && l.ignoreRecordNotFoundError)
	isSlow := !isRealError && l.slowThreshold != 0 && elapsed > l.slowThreshold

	var slogLevel slog.Level
	var traceMsg string

	switch {
	case isRealError:
		if l.level < gormlogger.Error {
			return
		}
		slogLevel = logger.Error.ToSlogLevel()
		traceMsg = "database error"
	case isSlow:
		if l.level < gormlogger.Warn {
			return
		}
		slogLevel = logger.Warn.ToSlogLevel()
		traceMsg = fmt.Sprintf("SLOW SQL >= %v", l.slowThreshold)
	default:
		if l.level < gormlogger.Info {
			return
		}
		slogLevel = logger.Info.ToSlogLevel()
		traceMsg = "database query"
	}

	if !l.handler.Enabled(ctx, slogLevel) {
		return
	}

	sql, rows := fc()
	file, line, fn, pc := l.getCaller(3)

	capacity := 4
	if rows >= 0 {
		capacity++
	}
	if file != "" {
		capacity++
	}
	if fn != "" {
		capacity++
	}
	if isRealError {
		capacity++
	}

	attrs := make([]slog.Attr, 0, capacity)
	attrs = append(attrs,
		slog.Duration("duration", elapsed),
	)
	if rows >= 0 {
		attrs = append(attrs, slog.Int64("rows", rows))
	}
	attrs = append(attrs,
		slog.String("sql", sql),
		slog.String("service", "database"),
	)
	if file != "" {
		attrs = append(attrs, slog.String("file", fmt.Sprintf("%s:%d", file, line)))
	}
	if fn != "" {
		attrs = append(attrs, slog.String("func", fn))
	}
	if isRealError {
		attrs = append(attrs, slog.String("error", err.Error()))
	}

	l.emit(ctx, slogLevel, traceMsg, pc, attrs)
}
