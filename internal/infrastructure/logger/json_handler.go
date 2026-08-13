package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
)

// jsonHandler is a pretty-printing slog.Handler that formats JSON output
// with indentation for development readability.
type jsonHandler struct {
	inner  slog.Handler
	buffer *bytes.Buffer
	mutex  *sync.Mutex
	writer io.Writer
}

func (h *jsonHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *jsonHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &jsonHandler{inner: h.inner.WithAttrs(attrs), buffer: h.buffer, mutex: h.mutex, writer: h.writer}
}

func (h *jsonHandler) WithGroup(name string) slog.Handler {
	return &jsonHandler{inner: h.inner.WithGroup(name), buffer: h.buffer, mutex: h.mutex, writer: h.writer}
}

func (h *jsonHandler) Handle(ctx context.Context, r slog.Record) error {
	h.mutex.Lock()
	defer func() {
		h.buffer.Reset()
		h.mutex.Unlock()
	}()

	if err := h.inner.Handle(ctx, r); err != nil {
		return err
	}

	var attrs map[string]any
	if err := json.Unmarshal(h.buffer.Bytes(), &attrs); err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(attrs, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, byte('\n'))

	if _, err := h.writer.Write(bytes); err != nil {
		return err
	}

	return nil
}

func newJSONHandler(writer io.Writer, opts *slog.HandlerOptions) *jsonHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	buffer := &bytes.Buffer{}
	return &jsonHandler{
		buffer: buffer,
		inner: slog.NewJSONHandler(buffer, &slog.HandlerOptions{
			Level:       opts.Level,
			ReplaceAttr: opts.ReplaceAttr,
		}),
		mutex:  &sync.Mutex{},
		writer: writer,
	}
}
