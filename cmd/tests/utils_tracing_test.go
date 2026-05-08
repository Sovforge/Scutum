package tests

import (
	"context"
	"log/slog"
	"testing"
	"scutum/cmd/internal/utils"
)

func TestTracingUtils(t *testing.T) {
	ctx := context.Background()
	
	// Test WithTrace and GetTrace
	ctx = utils.WithTrace(ctx, "root")
	trace, ok := utils.GetTrace(ctx)
	if !ok {
		t.Fatal("expected trace info in context")
	}
	if trace.Name != "root" {
		t.Errorf("expected name root, got %s", trace.Name)
	}

	logger := utils.InitLogger(slog.LevelDebug, false)

	t.Run("Spans", func(t *testing.T) {
		span := utils.StartSpan(ctx, logger, "child-span")
		span.SetAttribute("key", "val")
		span.End(nil)
	})

	t.Run("Errors", func(t *testing.T) {
		err := utils.NewError("CODE123", "test error", nil)
		if err.Message != "test error" {
			t.Errorf("error message mismatch: %s", err.Message)
		}
		
		wrapped := utils.WrapError(err, "WRAP", "wrapped msg")
		if wrapped.Cause != err {
			t.Error("expected wrapped error to contain original cause")
		}
	})

	t.Run("ErrorHandler", func(t *testing.T) {
		h := utils.NewErrorHandler(logger)
		h.Handle(nil)
		h.Handle(utils.ErrInternal)
	})
}

