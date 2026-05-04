package statusui

import (
	"context"
	"errors"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vnovick/itervox/internal/logbuffer"
	"github.com/vnovick/itervox/internal/server"
)

// Run starts the bubbletea TUI, writing to stderr. It blocks until ctx is
// cancelled or the user presses q. buf may be nil (log pane disabled).
// cancelFn is called when the user presses x to kill the selected session; nil disables it.
func Run(ctx context.Context, snap func() server.StateSnapshot, buf *logbuffer.Buffer, cfg Config, cancelFn func(string) bool) {
	if err := waitForForegroundTTY(ctx); err != nil {
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			return
		case errors.Is(err, errNoControllingTTY):
			slog.Info("statusui: no controlling tty; terminal UI disabled", "error", err)
			return
		default:
			slog.Warn("statusui: foreground tty check failed; terminal UI disabled", "error", err)
			return
		}
	}

	m := New(snap, buf, cfg, cancelFn)
	p := tea.NewProgram(m,
		tea.WithOutput(os.Stderr),
		tea.WithAltScreen(),
	)

	// Stop the program when the context is cancelled.
	go func() {
		<-ctx.Done()
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		slog.Warn("statusui: bubbletea exited with error", "error", err)
	}
}
