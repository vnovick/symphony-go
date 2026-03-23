// Catwalk golden-file tests for Symphony's statusui Model.
//
// Catwalk drives the full Update→View pipeline by sending tea.Msg objects and
// capturing View() output into testdata/ golden files. On first run (or after a
// deliberate UI change) regenerate the golden files with:
//
//	go test ./internal/statusui/... -args -rewrite
//
// Then review and commit the generated testdata/* files.
package statusui

import (
	"io"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/knz/catwalk"

	"github.com/vnovick/symphony-go/internal/logbuffer"
	"github.com/vnovick/symphony-go/internal/server"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// newCatwalkModel builds a Model pre-seeded with one running session and a
// representative log-buffer, so View() renders meaningful content immediately
// without waiting for a real tickCmd to fire.
func newCatwalkModel() Model {
	buf := logbuffer.New()
	buf.Add("PROJ-1", `INFO claude: action session_id=s1 tool=Bash description="ls -la"`)
	buf.Add("PROJ-1", `INFO claude: action session_id=s1 tool=Bash description="cat go.mod"`)
	buf.Add("PROJ-1", `INFO claude: action session_id=s1 tool=Read description="README.md"`)
	buf.Add("PROJ-1", `INFO claude: text session_id=s1 text="Analysing the codebase."`)

	snap := newTestSnap(server.StateSnapshot{
		Running: []server.RunningRow{{Identifier: "PROJ-1", State: "Running"}},
	})
	m := New(snap, buf, Config{MaxAgents: 5}, func(id string) bool { return true })
	// Pre-populate sessions and navItems so the first View() call renders
	// session content without requiring a tick round-trip.
	m.sessions = []server.RunningRow{{Identifier: "PROJ-1", State: "Running"}}
	m.navItems = buildNavItems(m.sessions, m.subagents, m.collapsed)
	return m
}

// catwalkUpdater handles custom test-file commands that inject Symphony-specific
// tea.Msg objects which have no catwalk built-in equivalent.
//
// Supported commands (used in testdata/* files):
//
//	tick              — fires a tickMsg, syncing snap() into the model
//	picker-loaded     — opens the project picker with two test projects
func catwalkUpdater(m tea.Model, cmd string, args ...string) (bool, tea.Model, tea.Cmd, error) {
	switch cmd {
	case "tick":
		newM, c := m.Update(tickMsg(time.Now()))
		return true, newM, c, nil
	case "picker-loaded":
		newM, c := m.Update(pickerLoadedMsg{
			projects: []ProjectItem{
				{ID: "p1", Name: "Alpha", Slug: "alpha"},
				{ID: "p2", Name: "Beta", Slug: "beta"},
			},
		})
		return true, newM, c, nil
	}
	return false, m, nil, nil
}

// ---------------------------------------------------------------------------
// renderToolsView  (was 0% coverage)
// ---------------------------------------------------------------------------

// TestCatwalk_ToolsView verifies that pressing 't' switches the right pane to
// the per-tool call statistics table (renderToolsView).
func TestCatwalk_ToolsView(t *testing.T) {
	m := newCatwalkModel()
	catwalk.RunModel(t, "testdata/catwalk_tools", m,
		catwalk.WithWindowSize(100, 30),
		catwalk.WithUpdater(catwalkUpdater),
	)
}

// ---------------------------------------------------------------------------
// renderSessionDetails  (was 0% coverage)
// ---------------------------------------------------------------------------

// TestCatwalk_SessionDetails verifies that pressing 'd' switches the right
// pane to the session-details view (renderSessionDetails).
func TestCatwalk_SessionDetails(t *testing.T) {
	m := newCatwalkModel()
	catwalk.RunModel(t, "testdata/catwalk_details", m,
		catwalk.WithWindowSize(100, 30),
		catwalk.WithUpdater(catwalkUpdater),
	)
}

// ---------------------------------------------------------------------------
// renderToolDetailView  (was 0% coverage)
// ---------------------------------------------------------------------------

// TestCatwalk_ToolDetailDrilldown verifies that the drill-down sequence
// (t → tab → enter) opens the per-call history for the selected tool
// (renderToolDetailView).
func TestCatwalk_ToolDetailDrilldown(t *testing.T) {
	m := newCatwalkModel()
	catwalk.RunModel(t, "testdata/catwalk_tool_detail", m,
		catwalk.WithWindowSize(100, 30),
		catwalk.WithUpdater(catwalkUpdater),
	)
}

// ---------------------------------------------------------------------------
// pickerSlugAt + applyPickerFilter  (were 0% coverage)
// ---------------------------------------------------------------------------

// TestCatwalk_ProjectPicker verifies the picker open → toggle → apply flow
// which exercises pickerSlugAt (on space) and applyPickerFilter (on enter).
func TestCatwalk_ProjectPicker(t *testing.T) {
	applied := []string(nil)
	m := newCatwalkModel()
	m.cfg.SetProjectFilter = func(slugs []string) { applied = slugs }
	catwalk.RunModel(t, "testdata/catwalk_picker", m,
		catwalk.WithWindowSize(100, 30),
		catwalk.WithUpdater(catwalkUpdater),
		// Custom observer so golden file captures whether the filter was applied.
		catwalk.WithObserver("filter", catwalk.Observer(func(w io.Writer, _ tea.Model) error {
			if applied == nil {
				_, err := w.Write([]byte("(not applied)"))
				return err
			}
			if len(applied) == 0 {
				_, err := w.Write([]byte("all"))
				return err
			}
			_, err := w.Write([]byte(applied[0]))
			return err
		})),
	)
}
