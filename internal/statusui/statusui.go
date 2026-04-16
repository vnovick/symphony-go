package statusui

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vnovick/itervox/internal/logbuffer"
	"github.com/vnovick/itervox/internal/server"
)

// Run starts the bubbletea TUI, writing to stderr. It blocks until ctx is
// cancelled or the user presses q. buf may be nil (log pane disabled).
// cancelFn is called when the user presses x to kill the selected session; nil disables it.
func Run(ctx context.Context, snap func() server.StateSnapshot, buf *logbuffer.Buffer, cfg Config, cancelFn func(string) bool) {
	// Ignore SIGTTOU/SIGTTIN so the TUI can safely take the terminal even
	// when the shell's process-group state is slightly dirty (e.g. after a
	// previous process opened a browser or exited uncleanly).
	signal.Ignore(syscall.SIGTTOU, syscall.SIGTTIN)

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
