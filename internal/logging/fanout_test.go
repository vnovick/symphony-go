package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	charmlog "github.com/charmbracelet/log"
)

func TestFanoutHandler_WritesToAllHandlers(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelInfo})
	h2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelInfo})

	logger := slog.New(NewFanoutHandler(h1, h2))
	logger.Info("hello", "key", "value")

	if !strings.Contains(buf1.String(), "hello") {
		t.Errorf("handler 1 did not receive message, got: %q", buf1.String())
	}
	if !strings.Contains(buf2.String(), "hello") {
		t.Errorf("handler 2 did not receive message, got: %q", buf2.String())
	}
	if !strings.Contains(buf1.String(), "key=value") {
		t.Errorf("handler 1 missing attribute, got: %q", buf1.String())
	}
}

func TestFanoutHandler_RespectsLevelPerHandler(t *testing.T) {
	var debugBuf, infoBuf bytes.Buffer
	debugH := slog.NewTextHandler(&debugBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	infoH := slog.NewTextHandler(&infoBuf, &slog.HandlerOptions{Level: slog.LevelInfo})

	logger := slog.New(NewFanoutHandler(debugH, infoH))
	logger.Debug("debug-only")
	logger.Info("both-get-this")

	if !strings.Contains(debugBuf.String(), "debug-only") {
		t.Error("debug handler should have received debug message")
	}
	if strings.Contains(infoBuf.String(), "debug-only") {
		t.Error("info handler should NOT have received debug message")
	}
	if !strings.Contains(infoBuf.String(), "both-get-this") {
		t.Error("info handler should have received info message")
	}
}

func TestFanoutHandler_Enabled_TrueIfAnyEnabled(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	debugH := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelDebug})
	warnH := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelWarn})

	fan := NewFanoutHandler(debugH, warnH)

	if !fan.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("should be enabled for debug (debug handler accepts it)")
	}
	if !fan.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("should be enabled for info (debug handler accepts it)")
	}
	if !fan.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("should be enabled for warn (both accept it)")
	}
}

func TestFanoutHandler_WithAttrs_PropagatesAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})

	fan := NewFanoutHandler(h)
	withAttrs := fan.WithAttrs([]slog.Attr{slog.String("service", "test")})

	logger := slog.New(withAttrs)
	logger.Info("message")

	if !strings.Contains(buf.String(), "service=test") {
		t.Errorf("expected attr propagation, got: %q", buf.String())
	}
}

func TestFanoutHandler_WithGroup_PropagatesGroup(t *testing.T) {
	var buf bytes.Buffer
	h := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})

	fan := NewFanoutHandler(h)
	grouped := fan.WithGroup("grp")

	logger := slog.New(grouped)
	logger.Info("message", "k", "v")

	if !strings.Contains(buf.String(), "grp.k=v") {
		t.Errorf("expected group propagation, got: %q", buf.String())
	}
}

func TestFanoutHandler_CharmlogAndTextHandler(t *testing.T) {
	// Integration test: verify charmbracelet/log handler and TextHandler
	// both receive records when combined via FanoutHandler.
	var charmBuf, textBuf bytes.Buffer

	charmHandler := charmlog.NewWithOptions(&charmBuf, charmlog.Options{
		ReportTimestamp: false,
		Level:           charmlog.InfoLevel,
	})
	textHandler := slog.NewTextHandler(&textBuf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(NewFanoutHandler(charmHandler, textHandler))
	logger.Info("starting", "version", "1.0.0")
	logger.Warn("slow query", "elapsed_ms", 500)

	// charmbracelet/log output should contain the messages
	charmOut := charmBuf.String()
	if !strings.Contains(charmOut, "starting") {
		t.Errorf("charm handler missing 'starting', got: %q", charmOut)
	}
	if !strings.Contains(charmOut, "1.0.0") {
		t.Errorf("charm handler missing version attr, got: %q", charmOut)
	}
	if !strings.Contains(charmOut, "slow query") {
		t.Errorf("charm handler missing 'slow query', got: %q", charmOut)
	}

	// TextHandler output should have standard slog format
	textOut := textBuf.String()
	if !strings.Contains(textOut, "msg=starting") {
		t.Errorf("text handler missing 'starting', got: %q", textOut)
	}
	if !strings.Contains(textOut, "version=1.0.0") {
		t.Errorf("text handler missing version attr, got: %q", textOut)
	}
	if !strings.Contains(textOut, `msg="slow query"`) {
		t.Errorf("text handler missing 'slow query', got: %q", textOut)
	}
}

func TestFanoutHandler_CharmlogLevelMapping(t *testing.T) {
	// Verify that charmbracelet/log level filtering works correctly
	// when used through the slog interface via FanoutHandler.
	var charmBuf bytes.Buffer

	charmHandler := charmlog.NewWithOptions(&charmBuf, charmlog.Options{
		ReportTimestamp: false,
		Level:           charmlog.WarnLevel,
	})

	logger := slog.New(NewFanoutHandler(charmHandler))
	logger.Info("should-be-filtered")
	logger.Warn("should-appear")
	logger.Error("also-appears")

	out := charmBuf.String()
	if strings.Contains(out, "should-be-filtered") {
		t.Errorf("info message should be filtered by warn-level handler, got: %q", out)
	}
	if !strings.Contains(out, "should-appear") {
		t.Errorf("warn message should appear, got: %q", out)
	}
	if !strings.Contains(out, "also-appears") {
		t.Errorf("error message should appear, got: %q", out)
	}
}

func TestFanoutHandler_CharmlogTimestampFormat(t *testing.T) {
	// Verify the time format we use in production (time.TimeOnly = "15:04:05").
	var buf bytes.Buffer

	handler := charmlog.NewWithOptions(&buf, charmlog.Options{
		ReportTimestamp: true,
		TimeFormat:      time.TimeOnly,
		Level:           charmlog.InfoLevel,
	})

	logger := slog.New(NewFanoutHandler(handler))
	logger.Info("timestamp-test")

	out := buf.String()
	// Should contain a time like "15:04:05" (HH:MM:SS format)
	now := time.Now().Format("15:04")
	if !strings.Contains(out, now) {
		t.Errorf("expected timestamp containing %q in output: %q", now, out)
	}
}
