// Package logging provides slog handler utilities.
package logging

import (
	"context"
	"log/slog"
)

// FanoutHandler dispatches each slog record to multiple handlers.
// All handlers are called even if an earlier one returns an error;
// the first error (if any) is returned.
type FanoutHandler struct {
	handlers []slog.Handler
}

// NewFanoutHandler creates a handler that writes to all given handlers.
func NewFanoutHandler(handlers ...slog.Handler) *FanoutHandler {
	return &FanoutHandler{handlers: handlers}
}

func (f *FanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *FanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	var firstErr error
	for _, h := range f.handlers {
		if h.Enabled(ctx, record.Level) {
			if err := h.Handle(ctx, record); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (f *FanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cloned := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		cloned[i] = h.WithAttrs(attrs)
	}
	return &FanoutHandler{handlers: cloned}
}

func (f *FanoutHandler) WithGroup(name string) slog.Handler {
	cloned := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		cloned[i] = h.WithGroup(name)
	}
	return &FanoutHandler{handlers: cloned}
}
