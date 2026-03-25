package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/symphony-go/internal/server"
)

func baseSnap() server.StateSnapshot {
	return server.StateSnapshot{
		GeneratedAt: time.Now(),
		Running:     []server.RunningRow{},
		Retrying:    []server.RetryRow{},
	}
}

func makeTestConfig(snap server.StateSnapshot) server.Config {
	return server.Config{
		Snapshot:    func() server.StateSnapshot { return snap },
		RefreshChan: make(chan struct{}, 1),
	}
}

func testServer(t *testing.T) *server.Server {
	t.Helper()
	return server.New(makeTestConfig(baseSnap()))
}

func TestStateEndpointReturnsJSON(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/state", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "generatedAt")
}

func TestUnknownRouteReturns404(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nonexistent-route", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRefreshReturns202(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/refresh", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Contains(t, w.Body.String(), "queued")
}

func TestWrongMethodReturns405(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/state", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestDashboardReturns200HTML(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, strings.HasPrefix(w.Header().Get("Content-Type"), "text/html"))
}

func TestStateEndpointShape(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/state", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "generatedAt")
	assert.Contains(t, body, "running")
	assert.Contains(t, body, "retrying")
	assert.Contains(t, body, "counts")
}

// postJSON is a helper that sends a POST with a JSON body and returns the recorder.
func postJSON(t *testing.T, srv *server.Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// patchJSON is a helper that sends a PATCH with a JSON body and returns the recorder.
func patchJSON(t *testing.T, srv *server.Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestSetWorkers(t *testing.T) {
	tests := []struct {
		name           string
		currentWorkers int
		body           string
		wantStatus     int
		wantWorkers    int
	}{
		{
			name:           "absolute value",
			currentWorkers: 1,
			body:           `{"workers":5}`,
			wantStatus:     http.StatusOK,
			wantWorkers:    5,
		},
		{
			name:           "delta positive",
			currentWorkers: 3,
			body:           `{"delta":2}`,
			wantStatus:     http.StatusOK,
			wantWorkers:    5,
		},
		{
			name:           "delta clamps to 1",
			currentWorkers: 2,
			body:           `{"delta":-100}`,
			wantStatus:     http.StatusOK,
			wantWorkers:    1,
		},
		{
			name:           "absolute clamps to 50",
			currentWorkers: 1,
			body:           `{"workers":100}`,
			wantStatus:     http.StatusOK,
			wantWorkers:    50,
		},
		{
			name:           "invalid JSON returns 400",
			currentWorkers: 1,
			body:           `not-json`,
			wantStatus:     http.StatusBadRequest,
			wantWorkers:    0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var called int
			snap := baseSnap()
			snap.MaxConcurrentAgents = tc.currentWorkers
			cfg := makeTestConfig(snap)
			cfg.Client = &server.FuncClient{
				SetWorkersFn: func(n int) { called = n },
				BumpWorkersFn: func(delta int) int {
					next := snap.MaxConcurrentAgents + delta
					if next < 1 {
						next = 1
					}
					if next > 50 {
						next = 50
					}
					called = next
					return next
				},
			}
			srv := server.New(cfg)

			w := postJSON(t, srv, "/api/v1/settings/workers", tc.body)
			assert.Equal(t, tc.wantStatus, w.Code)

			if tc.wantStatus == http.StatusOK {
				var resp map[string]any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, float64(tc.wantWorkers), resp["workers"])
				assert.Equal(t, tc.wantWorkers, called)
			} else {
				assert.Contains(t, w.Body.String(), "error")
			}
		})
	}
}

func TestSetIssueProfile(t *testing.T) {
	tests := []struct {
		name           string
		identifier     string
		body           string
		wantStatus     int
		wantProfile    string
		wantIdentifier string
	}{
		{
			name:           "set profile",
			identifier:     "ENG-1",
			body:           `{"profile":"fast"}`,
			wantStatus:     http.StatusOK,
			wantProfile:    "fast",
			wantIdentifier: "ENG-1",
		},
		{
			name:           "clear profile",
			identifier:     "ENG-1",
			body:           `{"profile":""}`,
			wantStatus:     http.StatusOK,
			wantProfile:    "",
			wantIdentifier: "ENG-1",
		},
		{
			name:       "invalid JSON returns 400",
			identifier: "ENG-1",
			body:       `not-json`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotIdentifier, gotProfile string
			cfg := makeTestConfig(baseSnap())
			cfg.Client = &server.FuncClient{
				SetIssueProfileFn: func(identifier, profile string) {
					gotIdentifier = identifier
					gotProfile = profile
				},
			}
			srv := server.New(cfg)

			path := "/api/v1/issues/" + tc.identifier + "/profile"
			w := postJSON(t, srv, path, tc.body)
			assert.Equal(t, tc.wantStatus, w.Code)

			if tc.wantStatus == http.StatusOK {
				var resp map[string]any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, true, resp["ok"])
				assert.Equal(t, tc.wantIdentifier, resp["identifier"])
				assert.Equal(t, tc.wantProfile, resp["profile"])
				assert.Equal(t, tc.wantIdentifier, gotIdentifier)
				assert.Equal(t, tc.wantProfile, gotProfile)
			} else {
				assert.Contains(t, w.Body.String(), "error")
			}
		})
	}
}

func TestUpsertProfileIncludesBackend(t *testing.T) {
	var gotName string
	var gotDef server.ProfileDef
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{
		UpsertProfileFn: func(name string, def server.ProfileDef) error {
			gotName = name
			gotDef = def
			return nil
		},
	}
	srv := server.New(cfg)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings/profiles/codex-fast", bytes.NewBufferString(`{"command":"run-codex-wrapper","prompt":"fast path","backend":"codex"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "codex-fast", gotName)
	assert.Equal(t, "run-codex-wrapper", gotDef.Command)
	assert.Equal(t, "fast path", gotDef.Prompt)
	assert.Equal(t, "codex", gotDef.Backend)
}

func TestUpdateIssueState(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		body       string
		updaterErr error
		wantStatus int
	}{
		{
			name:       "success",
			identifier: "ENG-1",
			body:       `{"state":"In Progress"}`,
			updaterErr: nil,
			wantStatus: http.StatusOK,
		},
		{
			name:       "updater returns error gives 500",
			identifier: "ENG-1",
			body:       `{"state":"Done"}`,
			updaterErr: errors.New("tracker unavailable"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "invalid JSON returns 400",
			identifier: "ENG-1",
			body:       `not-json`,
			updaterErr: nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing state field returns 400",
			identifier: "ENG-1",
			body:       `{}`,
			updaterErr: nil,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := makeTestConfig(baseSnap())
			cfg.Client = &server.FuncClient{
				UpdateIssueStateFn: func(ctx context.Context, identifier, stateName string) error {
					return tc.updaterErr
				},
			}
			srv := server.New(cfg)

			path := "/api/v1/issues/" + tc.identifier + "/state"
			w := patchJSON(t, srv, path, tc.body)
			assert.Equal(t, tc.wantStatus, w.Code)

			if tc.wantStatus == http.StatusOK {
				var resp map[string]any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				assert.Equal(t, true, resp["ok"])
			} else {
				assert.Contains(t, w.Body.String(), "error")
			}
		})
	}
}

// ─── handleIssues ─────────────────────────────────────────────────────────────

func testServerWithFetchIssues(t *testing.T, fn func(ctx context.Context) ([]server.TrackerIssue, error)) *server.Server {
	t.Helper()
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{FetchIssuesFn: fn}
	return server.New(cfg)
}

func TestHandleIssues_ReturnsJSONArray(t *testing.T) {
	issues := []server.TrackerIssue{
		{Identifier: "ENG-1", Title: "Fix bug", State: "In Progress"},
		{Identifier: "ENG-2", Title: "Add feature", State: "Todo"},
	}
	srv := testServerWithFetchIssues(t, func(_ context.Context) ([]server.TrackerIssue, error) {
		return issues, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var got []server.TrackerIssue
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)
	assert.Equal(t, "ENG-1", got[0].Identifier)
	assert.Equal(t, "ENG-2", got[1].Identifier)
}

func TestHandleIssues_FetchError_Returns500(t *testing.T) {
	srv := testServerWithFetchIssues(t, func(_ context.Context) ([]server.TrackerIssue, error) {
		return nil, errors.New("tracker down")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

// ─── handleIssueDetail ────────────────────────────────────────────────────────

func TestHandleIssueDetail_Found_Returns200(t *testing.T) {
	issues := []server.TrackerIssue{
		{Identifier: "ENG-10", Title: "My issue", State: "Todo"},
	}
	srv := testServerWithFetchIssues(t, func(_ context.Context) ([]server.TrackerIssue, error) {
		return issues, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues/ENG-10", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var got server.TrackerIssue
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "ENG-10", got.Identifier)
	assert.Equal(t, "My issue", got.Title)
}

func TestHandleIssueDetail_NotFound_Returns404(t *testing.T) {
	srv := testServerWithFetchIssues(t, func(_ context.Context) ([]server.TrackerIssue, error) {
		return []server.TrackerIssue{}, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues/ENG-999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_found")
}

func TestHandleIssueDetail_FetchError_Returns500(t *testing.T) {
	srv := testServerWithFetchIssues(t, func(_ context.Context) ([]server.TrackerIssue, error) {
		return nil, errors.New("db error")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues/ENG-1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ─── handleCancelIssue ────────────────────────────────────────────────────────

func testServerWithCancel(t *testing.T, fn func(string) bool) *server.Server {
	t.Helper()
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{CancelIssueFn: fn}
	return server.New(cfg)
}

func TestHandleCancelIssue_Found_Returns200(t *testing.T) {
	srv := testServerWithCancel(t, func(id string) bool { return id == "ENG-1" })
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/issues/ENG-1", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["cancelled"])
	assert.Equal(t, "ENG-1", resp["identifier"])
}

func TestHandleCancelIssue_NotFound_Returns404(t *testing.T) {
	srv := testServerWithCancel(t, func(id string) bool { return false })
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/issues/ENG-999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_running")
}

// ─── handleResumeIssue ────────────────────────────────────────────────────────

func testServerWithResume(t *testing.T, fn func(string) bool) *server.Server {
	t.Helper()
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{ResumeIssueFn: fn}
	return server.New(cfg)
}

func TestHandleResumeIssue_Found_Returns200(t *testing.T) {
	srv := testServerWithResume(t, func(id string) bool { return id == "ENG-5" })
	w := postJSON(t, srv, "/api/v1/issues/ENG-5/resume", "")

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["resumed"])
	assert.Equal(t, "ENG-5", resp["identifier"])
}

func TestHandleResumeIssue_NotPaused_Returns404(t *testing.T) {
	srv := testServerWithResume(t, func(id string) bool { return false })
	w := postJSON(t, srv, "/api/v1/issues/ENG-5/resume", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_paused")
}

// ─── handleSetAgentMode ───────────────────────────────────────────────────────

func testServerWithAgentMode(t *testing.T, fn func(string) error) *server.Server {
	t.Helper()
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{SetAgentModeFn: fn}
	return server.New(cfg)
}

func TestHandleSetAgentMode_ValidMode_Returns200(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{"empty mode (off)", ""},
		{"subagents mode", "subagents"},
		{"teams mode", "teams"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotMode string
			srv := testServerWithAgentMode(t, func(mode string) error {
				gotMode = mode
				return nil
			})

			body := `{"mode":"` + tc.mode + `"}`
			w := postJSON(t, srv, "/api/v1/settings/agent-mode", body)

			assert.Equal(t, http.StatusOK, w.Code)
			var resp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, true, resp["ok"])
			assert.Equal(t, tc.mode, resp["agentMode"])
			assert.Equal(t, tc.mode, gotMode)
		})
	}
}

func TestHandleSetAgentMode_InvalidJSON_Returns400(t *testing.T) {
	srv := testServerWithAgentMode(t, func(mode string) error { return nil })
	w := postJSON(t, srv, "/api/v1/settings/agent-mode", `not-json`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

func TestHandleSetAgentMode_InvalidMode_Returns400(t *testing.T) {
	srv := testServerWithAgentMode(t, func(mode string) error { return nil })
	w := postJSON(t, srv, "/api/v1/settings/agent-mode", `{"mode":"invalid-mode"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_mode")
}

// ─── handleTerminateIssue ─────────────────────────────────────────────────────

func testServerWithTerminate(t *testing.T, fn func(string) bool) *server.Server {
	t.Helper()
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{TerminateIssueFn: fn}
	return server.New(cfg)
}


func putJSON(t *testing.T, srv *server.Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestHandleTerminateIssue_Success(t *testing.T) {
	var got string
	srv := testServerWithTerminate(t, func(id string) bool { got = id; return true })
	w := postJSON(t, srv, "/api/v1/issues/ENG-5/terminate", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ENG-5", got)
}

func TestHandleTerminateIssue_NotFound(t *testing.T) {
	srv := testServerWithTerminate(t, func(string) bool { return false })
	w := postJSON(t, srv, "/api/v1/issues/ENG-X/terminate", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ─── handleReanalyzeIssue ─────────────────────────────────────────────────────

func testServerWithReanalyze(t *testing.T, fn func(string) bool) *server.Server {
	t.Helper()
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{ReanalyzeIssueFn: fn}
	return server.New(cfg)
}

func TestHandleReanalyzeIssue_Success(t *testing.T) {
	var got string
	srv := testServerWithReanalyze(t, func(id string) bool { got = id; return true })
	w := postJSON(t, srv, "/api/v1/issues/ENG-7/reanalyze", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ENG-7", got)
	assert.Contains(t, w.Body.String(), "queued")
}

func TestHandleReanalyzeIssue_NotPaused(t *testing.T) {
	srv := testServerWithReanalyze(t, func(string) bool { return false })
	w := postJSON(t, srv, "/api/v1/issues/ENG-7/reanalyze", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not_paused")
}

// ─── handleListProfiles / handleDeleteProfile ─────────────────────────────────

func testServerWithProfiles(t *testing.T) (*server.Server, *map[string]server.ProfileDef) {
	t.Helper()
	defs := map[string]server.ProfileDef{"fast": {Command: "codex", Backend: "codex"}}
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{
		ProfileDefsFn:   func() map[string]server.ProfileDef { return defs },
		DeleteProfileFn: func(name string) error { delete(defs, name); return nil },
	}
	return server.New(cfg), &defs
}

func TestHandleListProfiles_ReturnsProfiles(t *testing.T) {
	srv, _ := testServerWithProfiles(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/profiles", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "fast")
}

func TestHandleDeleteProfile_Success(t *testing.T) {
	srv, defs := testServerWithProfiles(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/settings/profiles/fast", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, *defs, "fast")
}

// ─── handleListProjects / handleGetProjectFilter / handleSetProjectFilter ─────

type fakeProjectManager struct {
	projects []server.Project
	filter   []string
}

func (f *fakeProjectManager) FetchProjects(_ context.Context) ([]server.Project, error) {
	return f.projects, nil
}
func (f *fakeProjectManager) GetProjectFilter() []string     { return f.filter }
func (f *fakeProjectManager) SetProjectFilter(s []string)    { f.filter = s }

func testServerWithProjects(t *testing.T) (*server.Server, *fakeProjectManager) {
	t.Helper()
	pm := &fakeProjectManager{projects: []server.Project{{Name: "Alpha"}}}
	cfg := makeTestConfig(baseSnap())
	cfg.ProjectManager = pm
	return server.New(cfg), pm
}

func TestHandleListProjects_NotConfigured(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestHandleListProjects_ReturnsProjects(t *testing.T) {
	srv, _ := testServerWithProjects(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Alpha")
}

func TestHandleGetProjectFilter_ReturnsFilter(t *testing.T) {
	srv, pm := testServerWithProjects(t)
	pm.filter = []string{"proj-1"}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/filter", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "proj-1")
}

func TestHandleSetProjectFilter_SetsSlugs(t *testing.T) {
	srv, pm := testServerWithProjects(t)
	w := putJSON(t, srv, "/api/v1/projects/filter", `{"slugs":["s1","s2"]}`)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"s1", "s2"}, pm.filter)
}

func TestHandleSetProjectFilter_NullSlugsResetsFilter(t *testing.T) {
	srv, pm := testServerWithProjects(t)
	pm.filter = []string{"old"}
	w := putJSON(t, srv, "/api/v1/projects/filter", `{}`)
	assert.Equal(t, http.StatusOK, w.Code)
	// nil slugs = reset to WORKFLOW.md default (nil)
	assert.Nil(t, pm.filter)
}

// ─── handleUpdateTrackerStates ────────────────────────────────────────────────

func TestHandleUpdateTrackerStates_Success(t *testing.T) {
	var gotActive, gotTerminal []string
	var gotCompletion string
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{
		UpdateTrackerStatesFn: func(active, terminal []string, completion string) error {
			gotActive = active
			gotTerminal = terminal
			gotCompletion = completion
			return nil
		},
	}
	srv := server.New(cfg)
	w := putJSON(t, srv, "/api/v1/settings/tracker/states",
		`{"activeStates":["Todo","In Progress"],"terminalStates":["Done"],"completionState":"Done"}`)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []string{"Todo", "In Progress"}, gotActive)
	assert.Equal(t, []string{"Done"}, gotTerminal)
	assert.Equal(t, "Done", gotCompletion)
}

func TestHandleUpdateTrackerStates_InvalidJSON(t *testing.T) {
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{UpdateTrackerStatesFn: func(_, _ []string, _ string) error { return nil }}
	srv := server.New(cfg)
	w := putJSON(t, srv, "/api/v1/settings/tracker/states", `{bad json`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ─── Notify / broadcaster ─────────────────────────────────────────────────────

func TestNotifyDoesNotPanicWithNoSubscribers(t *testing.T) {
	srv := testServer(t)
	// Must not panic; no-op when broadcaster has no subscribers.
	assert.NotPanics(t, func() {
		srv.Notify()
	})
}

// ─── handleIssueLogs / handleClearIssueLogs ───────────────────────────────────

func testServerWithIssueLogs(t *testing.T, fetchLogs func(string) []string) *server.Server {
	t.Helper()
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{FetchLogsFn: fetchLogs}
	return server.New(cfg)
}

func TestHandleIssueLogs_ReturnsEntries(t *testing.T) {
	srv := testServerWithIssueLogs(t, func(id string) []string {
		return []string{`{"level":"INFO","msg":"claude: text","time":"10:00:00","text":"something happened"}`}
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues/ENG-1/logs", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var entries []any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entries))
	// Non-skipped lines get parsed.
	assert.NotEmpty(t, entries)
}

func TestHandleIssueLogs_EmptyLogs(t *testing.T) {
	srv := testServerWithIssueLogs(t, func(string) []string { return nil })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/issues/ENG-1/logs", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", strings.TrimSpace(w.Body.String()))
}

func TestHandleClearIssueLogs_Success(t *testing.T) {
	var cleared string
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{ClearLogsFn: func(id string) error { cleared = id; return nil }}
	srv := server.New(cfg)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/issues/ENG-9/logs", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ENG-9", cleared)
}

// ─── handleAIReview ───────────────────────────────────────────────────────────

func TestHandleAIReview_Success(t *testing.T) {
	var got string
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{DispatchReviewerFn: func(id string) error { got = id; return nil }}
	srv := server.New(cfg)
	w := postJSON(t, srv, "/api/v1/issues/ENG-42/ai-review", "")
	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "ENG-42", got)
}

func TestHandleAIReview_DispatchError(t *testing.T) {
	cfg := makeTestConfig(baseSnap())
	cfg.Client = &server.FuncClient{DispatchReviewerFn: func(string) error { return errors.New("reviewer busy") }}
	srv := server.New(cfg)
	w := postJSON(t, srv, "/api/v1/issues/ENG-1/ai-review", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
