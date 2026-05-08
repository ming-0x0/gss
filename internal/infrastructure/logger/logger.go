package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"time"

	"gss/internal/infrastructure/logger/handler"
)

type Level string

const (
	Fatal Level = "fatal"
	Error Level = "error"
	Warn  Level = "warn"
	Info  Level = "info"
	Debug Level = "debug"
)

func (l Level) GTE(other Level) bool {
	return l.ToSlogLevel() >= other.ToSlogLevel()
}

const (
	slogFatal = slog.Level(4)
	slogError = slog.Level(3)
	slogWarn  = slog.Level(2)
	slogInfo  = slog.Level(1)
	slogDebug = slog.Level(0)
)

var background = context.Background()

func GetLevelFromString(level string) Level {
	switch level {
	case "fatal":
		return Fatal
	case "error":
		return Error
	case "warn":
		return Warn
	case "info":
		return Info
	case "debug":
		return Debug
	default:
		return Info
	}
}

type config struct {
	level  Level
	writer io.Writer
}

type Option func(*config)

func WithLevel(level string) Option {
	return func(c *config) {
		c.level = GetLevelFromString(level)
	}
}

func WithWriter(writer io.Writer) Option {
	return func(c *config) {
		c.writer = writer
	}
}

func (l Level) ToSlogLevel() slog.Level {
	switch l {
	case Fatal:
		return slogFatal
	case Error:
		return slogError
	case Warn:
		return slogWarn
	case Info:
		return slogInfo
	case Debug:
		return slogDebug
	default:
		return slogInfo
	}
}

func slogLevelName(level slog.Level) string {
	switch level {
	case slogFatal:
		return "fatal"
	case slogError:
		return "error"
	case slogWarn:
		return "warn"
	case slogInfo:
		return "info"
	case slogDebug:
		return "debug"
	default:
		return level.String()
	}
}

type Logger struct {
	*slog.Logger
}

func New(opts ...Option) *Logger {
	cfg := &config{
		level:  Info,
		writer: os.Stdout,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	handlerOptions := &slog.HandlerOptions{
		Level: cfg.level.ToSlogLevel(),
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				return slog.String(slog.TimeKey, a.Value.Time().Format(time.RFC3339))
			case slog.LevelKey:
				level := a.Value.Any().(slog.Level)
				return slog.String(slog.LevelKey, slogLevelName(level))
			default:
				return a
			}
		},
	}

	return &Logger{
		Logger: slog.New(handler.NewJSONHandler(cfg.writer, handlerOptions)),
	}
}

func (l *Logger) log(ctx context.Context, level Level, msg string, keyVals ...any) {
	slogLevel := level.ToSlogLevel()

	pc, file, line, ok := runtime.Caller(2)

	attrs := toAttrs(keyVals...)
	if ok {
		attrs = append(attrs,
			slog.String("file", fmt.Sprintf("%s:%d", file, line)),
		)
		if fn := runtime.FuncForPC(pc); fn != nil {
			attrs = append(attrs, slog.String("func", fn.Name()))
		}
	}

	l.Log(ctx, slogLevel, msg, attrs...)
}

func (l *Logger) Fatal(msg string, attrs ...any) {
	l.log(background, Fatal, msg, attrs...)
	os.Exit(1)
}

func (l *Logger) FatalContext(ctx context.Context, msg string, attrs ...any) {
	l.log(ctx, Fatal, msg, attrs...)
	os.Exit(1)
}

func (l *Logger) Error(msg string, attrs ...any) {
	l.log(background, Error, msg, attrs...)
}

func (l *Logger) ErrorContext(ctx context.Context, msg string, attrs ...any) {
	l.log(ctx, Error, msg, attrs...)
}

func (l *Logger) Warn(msg string, attrs ...any) {
	l.log(background, Warn, msg, attrs...)
}

func (l *Logger) WarnContext(ctx context.Context, msg string, attrs ...any) {
	l.log(ctx, Warn, msg, attrs...)
}

func (l *Logger) Info(msg string, attrs ...any) {
	l.log(background, Info, msg, attrs...)
}

func (l *Logger) InfoContext(ctx context.Context, msg string, attrs ...any) {
	l.log(ctx, Info, msg, attrs...)
}

func (l *Logger) Debug(msg string, attrs ...any) {
	l.log(background, Debug, msg, attrs...)
}

func (l *Logger) DebugContext(ctx context.Context, msg string, attrs ...any) {
	l.log(ctx, Debug, msg, attrs...)
}

func (l *Logger) With(keyVals ...any) *Logger {
	return &Logger{Logger: l.Logger.With(keyVals...)}
}

func toAttrs(keyVals ...any) []any {
	n := len(keyVals)
	attrs := make([]any, 0, n/2+2)

	for i := 0; i+1 < n; i += 2 {
		key, ok := keyVals[i].(string)
		if !ok {
			key = fmt.Sprintf("key_%d", i)
		}
		attrs = append(attrs, slog.Attr{
			Key:   key,
			Value: slog.AnyValue(keyVals[i+1]),
		})
	}

	return attrs
}
