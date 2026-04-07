// Catwalk golden-file tests for Itervox's statusui Model.
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

	"github.com/vnovick/itervox/internal/logbuffer"
	"github.com/vnovick/itervox/internal/server"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// newCatwalkModel builds a Model pre-seeded with one running session and a
// representative log-buffer, so View() renders meaningful content immediately
// without waiting for a real tickCmd to fire.
func newCatwalkModel() Model {
	buf := logbuffer.New()
	buf.Add("PROJ-1", logLine("INFO", "claude: action", map[string]string{"session_id": "s1", "tool": "Bash", "description": "ls -la"}))
	buf.Add("PROJ-1", logLine("INFO", "claude: action", map[string]string{"session_id": "s1", "tool": "Bash", "description": "cat go.mod"}))
	buf.Add("PROJ-1", logLine("INFO", "claude: action", map[string]string{"session_id": "s1", "tool": "Read", "description": "README.md"}))
	buf.Add("PROJ-1", logLine("INFO", "claude: text", map[string]string{"session_id": "s1", "text": "Analysing the codebase."}))

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

// catwalkUpdater handles custom test-file commands that inject Itervox-specific
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
	case "set-profiles":
		// Inject available profiles into the snapshot so 'a' key works.
		mod := m.(Model)
		origSnap := mod.snap
		mod.snap = func() server.StateSnapshot {
			s := origSnap()
			s.AvailableProfiles = []string{"frontend", "backend", "infra"}
			s.ProfileDefs = map[string]server.ProfileDef{
				"frontend": {Command: "claude", Backend: "claude"},
				"backend":  {Command: "claude", Backend: "claude"},
				"infra":    {Command: "codex", Backend: "codex"},
			}
			return s
		}
		mod.cfg.SetIssueProfile = func(id, profile string) {
			mod.profileOverrides[id] = profile
		}
		mod.cfg.IssueProfiles = func() map[string]string {
			return mod.profileOverrides
		}
		// Trigger a tick to pick up new snap.
		newM, c := mod.Update(tickMsg(time.Now()))
		return true, newM, c, nil
	}
	return false, m, nil, nil
}

// ---------------------------------------------------------------------------
// Session details via split pane
// ---------------------------------------------------------------------------

// TestCatwalk_SessionDetails verifies that pressing 's' opens the split
// pane showing session details alongside logs.
func TestCatwalk_SessionDetails(t *testing.T) {
	m := newCatwalkModel()
	m.profileOverrides = make(map[string]string)
	catwalk.RunModel(t, "testdata/catwalk_details", m,
		catwalk.WithWindowSize(160, 30), // wide enough for split
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

// ---------------------------------------------------------------------------
// Split pane toggle
// ---------------------------------------------------------------------------

// TestCatwalk_SplitPane verifies the split pane toggle:
// 's' opens split details, 's' again closes it.
func TestCatwalk_SplitPane(t *testing.T) {
	m := newCatwalkModel()
	m.profileOverrides = make(map[string]string)
	catwalk.RunModel(t, "testdata/catwalk_split", m,
		catwalk.WithWindowSize(160, 30), // wide enough for split (>=120)
		catwalk.WithUpdater(catwalkUpdater),
	)
}

// ---------------------------------------------------------------------------
// Profile picker flow
// ---------------------------------------------------------------------------

// TestCatwalk_ProfilePicker verifies the profile assignment picker flow:
// set-profiles → 'a' opens picker → navigate → enter confirms.
func TestCatwalk_ProfilePicker(t *testing.T) {
	m := newCatwalkModel()
	m.profileOverrides = make(map[string]string)
	catwalk.RunModel(t, "testdata/catwalk_profile_picker", m,
		catwalk.WithWindowSize(100, 30),
		catwalk.WithUpdater(catwalkUpdater),
	)
}
