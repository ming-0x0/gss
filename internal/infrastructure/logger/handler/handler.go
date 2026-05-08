package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
)

type Handler struct {
	handler slog.Handler
	buffer  *bytes.Buffer
	mutex   *sync.Mutex
	writer  io.Writer
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{handler: h.handler.WithAttrs(attrs), buffer: h.buffer, mutex: h.mutex, writer: h.writer}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{handler: h.handler.WithGroup(name), buffer: h.buffer, mutex: h.mutex, writer: h.writer}
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	h.mutex.Lock()
	defer func() {
		h.buffer.Reset()
		h.mutex.Unlock()
	}()

	if err := h.handler.Handle(ctx, r); err != nil {
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

func NewJSONHandler(writer io.Writer, opts *slog.HandlerOptions) *Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	buffer := &bytes.Buffer{}
	handler := &Handler{
		buffer: buffer,
		handler: slog.NewJSONHandler(buffer, &slog.HandlerOptions{
			Level:       opts.Level,
			ReplaceAttr: opts.ReplaceAttr,
		}),
		mutex:  &sync.Mutex{},
		writer: writer,
	}

	return handler
}
