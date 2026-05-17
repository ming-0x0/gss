package orm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"time"

	"gss/internal/infrastructure/logger"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// internalPrefixes chứa các tiền tố package cần bỏ qua khi tìm caller.
// Là array tĩnh (không phải slice) → nằm trên stack, không heap-alloc,
// và được sắp xếp theo tần suất xuất hiện để thoát sớm nhất có thể.
var internalPrefixes = [...]string{
	"gorm.io/",
	"runtime.",
	"database/sql.",
	"gss/internal/infrastructure/orm.",
	"gss/internal/infrastructure/orm/",
}

type Logger struct {
	handler                   slog.Handler
	slowMsg                   string // pre-computed "SLOW SQL >= Xms", tránh fmt.Sprintf trong hot path
	level                     gormlogger.LogLevel
	slowThreshold             time.Duration
	ignoreRecordNotFoundError bool
}

type Option func(*Logger)

func WithHandler(handler slog.Handler) Option {
	return func(l *Logger) { l.handler = handler }
}

func WithSlowThreshold(d time.Duration) Option {
	return func(l *Logger) {
		l.slowThreshold = d
		l.slowMsg = "SLOW SQL >= " + d.String()
	}
}

func WithIgnoreRecordNotFoundError(ignore bool) Option {
	return func(l *Logger) { l.ignoreRecordNotFoundError = ignore }
}

func New(opts ...Option) *Logger {
	const defaultSlow = 200 * time.Millisecond
	l := &Logger{
		level:                     gormlogger.Warn,
		slowThreshold:             defaultSlow,
		slowMsg:                   "SLOW SQL >= " + defaultSlow.String(),
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

// isInternal kiểm tra fn có thuộc package nội bộ/runtime không.
// Fast-path O(1): lọc qua ký tự đầu tiên trước khi compare prefix đầy đủ.
// Chỉ 'g', 'r', 'd' mới có thể là internal → bỏ qua toàn bộ vòng lặp cho user frames.
func isInternal(fn string) bool {
	if len(fn) == 0 {
		return false
	}
	switch fn[0] {
	case 'g', 'r', 'd':
	default:
		return false
	}
	for _, p := range internalPrefixes {
		if len(fn) >= len(p) && fn[:len(p)] == p {
			return true
		}
	}
	return false
}

// fileAttrValue ghép "file:line" dùng strconv.Itoa thay vì fmt.Sprintf,
// tránh reflection và giảm allocation trong hot path.
func fileAttrValue(file string, line int) string {
	return file + ":" + strconv.Itoa(line)
}

// getCaller trả về frame user-land đầu tiên (bỏ qua internal frames).
// Không phải method để tránh pointer receiver overhead khi inlining.
func getCaller(skip int) (file string, line int, fn string, pc uintptr) {
	var pcs [16]uintptr
	n := runtime.Callers(skip, pcs[:])
	if n == 0 {
		return
	}
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !isInternal(frame.Function) {
			return frame.File, frame.Line, frame.Function, frame.PC
		}
		if !more {
			break
		}
	}
	return
}

// emit ghi record — caller phải đảm bảo handler.Enabled đã được check trước.
func (l *Logger) emit(ctx context.Context, level slog.Level, msg string, pc uintptr, attrs []slog.Attr) {
	r := slog.NewRecord(time.Now(), level, msg, pc)
	if len(attrs) > 0 {
		r.AddAttrs(attrs...)
	}
	_ = l.handler.Handle(ctx, r)
}

// log dành cho Info/Warn/Error.
// Dùng [2]slog.Attr trên stack thay vì make() → zero heap allocation.
func (l *Logger) log(ctx context.Context, level slog.Level, format string, args ...any) {
	if !l.handler.Enabled(ctx, level) {
		return
	}
	file, line, fn, pc := getCaller(4)
	msg := fmt.Sprintf(format, args...)
	if file == "" {
		l.emit(ctx, level, msg, pc, nil)
		return
	}
	var buf [2]slog.Attr
	buf[0] = slog.String("file", fileAttrValue(file, line))
	buf[1] = slog.String("func", fn)
	l.emit(ctx, level, msg, pc, buf[:])
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
	// Guard 1: level check rẻ nhất, không alloc gì.
	if l.level <= gormlogger.Silent {
		return
	}

	// Guard 2: phân loại error TRƯỚC khi đọc clock (time.Since).
	// Đọc clock là syscall, tránh khi sẽ return ngay ở error path.
	isRealError := err != nil && !(errors.Is(err, gorm.ErrRecordNotFound) && l.ignoreRecordNotFoundError)

	if isRealError {
		if l.level < gormlogger.Error {
			return
		}
		slogLevel := logger.Error.ToSlogLevel()
		if !l.handler.Enabled(ctx, slogLevel) {
			return
		}
		// Error path: đọc clock và build attrs chỉ khi thực sự cần ghi log.
		elapsed := time.Since(begin)
		sql, rows := fc()
		file, line, fn, pc := getCaller(3)

		// [7]slog.Attr trên stack: duration + rows? + sql + service + file? + func? + error
		var buf [7]slog.Attr
		n := l.fillCommonAttrs(buf[:], elapsed, rows, sql, file, line, fn)
		buf[n] = slog.String("error", err.Error())
		n++

		l.emit(ctx, slogLevel, "database error", pc, buf[:n])
		return
	}

	// Non-error path: đọc clock để kiểm tra slow query.
	elapsed := time.Since(begin)

	var slogLevel slog.Level
	var traceMsg string

	if l.slowThreshold != 0 && elapsed > l.slowThreshold {
		if l.level < gormlogger.Warn {
			return
		}
		slogLevel = logger.Warn.ToSlogLevel()
		traceMsg = l.slowMsg // pre-computed, zero alloc
	} else {
		if l.level < gormlogger.Info {
			return
		}
		slogLevel = logger.Info.ToSlogLevel()
		traceMsg = "database query"
	}

	// Guard 3: handler enabled — tránh getCaller + fc() khi handler tắt.
	if !l.handler.Enabled(ctx, slogLevel) {
		return
	}

	sql, rows := fc()
	file, line, fn, pc := getCaller(3)

	// [6]slog.Attr trên stack: duration + rows? + sql + service + file? + func?
	var buf [6]slog.Attr
	n := l.fillCommonAttrs(buf[:], elapsed, rows, sql, file, line, fn)

	l.emit(ctx, slogLevel, traceMsg, pc, buf[:n])
}

// fillCommonAttrs điền các attr chung (duration, rows, sql, service, file, func)
// vào dst và trả về số lượng attr đã điền.
// Tách ra để tránh duplicate code giữa error path và non-error path trong Trace.
func (l *Logger) fillCommonAttrs(dst []slog.Attr, elapsed time.Duration, rows int64, sql, file string, line int, fn string) int {
	n := 0
	dst[n] = slog.Duration("duration", elapsed)
	n++
	if rows >= 0 {
		dst[n] = slog.Int64("rows", rows)
		n++
	}
	dst[n] = slog.String("sql", sql)
	n++
	dst[n] = slog.String("service", "database")
	n++
	if file != "" {
		dst[n] = slog.String("file", fileAttrValue(file, line))
		n++
	}
	if fn != "" {
		dst[n] = slog.String("func", fn)
		n++
	}
	return n
}