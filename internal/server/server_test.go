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

func testServer(t *testing.T) *server.Server {
	t.Helper()
	snap := server.StateSnapshot{
		GeneratedAt: time.Now(),
		Running:     []server.RunningRow{},
		Retrying:    []server.RetryRow{},
	}
	return server.New(func() server.StateSnapshot { return snap }, make(chan struct{}, 1), "", nil, nil, nil, nil)
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

func TestUnknownIdentifierReturns404(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ENG-999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "error")
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

// testServerWithWorkers creates a server whose snapshot reports n max concurrent agents.
func testServerWithWorkers(t *testing.T, n int) *server.Server {
	t.Helper()
	snap := server.StateSnapshot{
		GeneratedAt:         time.Now(),
		Running:             []server.RunningRow{},
		Retrying:            []server.RetryRow{},
		MaxConcurrentAgents: n,
	}
	return server.New(func() server.StateSnapshot { return snap }, make(chan struct{}, 1), "", nil, nil, nil, nil)
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

// TestSetWorkersNotWired verifies that POST /api/v1/settings/workers returns 501 when
// the worker setter has not been wired.
func TestSetWorkersNotWired(t *testing.T) {
	srv := testServer(t)
	w := postJSON(t, srv, "/api/v1/settings/workers", `{"workers":5}`)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Contains(t, w.Body.String(), "error")
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
			srv := testServerWithWorkers(t, tc.currentWorkers)
			var called int
			srv.SetWorkerSetter(func(n int) { called = n })

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

// TestSetIssueProfileNotWired verifies that POST /api/v1/issues/{id}/profile returns 501
// when the profile setter has not been wired.
func TestSetIssueProfileNotWired(t *testing.T) {
	srv := testServer(t)
	w := postJSON(t, srv, "/api/v1/issues/ENG-1/profile", `{"profile":"fast"}`)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Contains(t, w.Body.String(), "error")
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
			srv := testServer(t)
			var gotIdentifier, gotProfile string
			srv.SetIssueProfileSetter(func(identifier, profile string) {
				gotIdentifier = identifier
				gotProfile = profile
			})

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

// TestUpdateIssueStateNotWired verifies that PATCH /api/v1/issues/{id}/state returns 501
// when the state updater has not been wired.
func TestUpdateIssueStateNotWired(t *testing.T) {
	srv := testServer(t)
	w := patchJSON(t, srv, "/api/v1/issues/ENG-1/state", `{"state":"In Progress"}`)
	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Contains(t, w.Body.String(), "error")
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
			srv := testServer(t)
			srv.SetUpdateIssueState(func(ctx context.Context, identifier, stateName string) error {
				return tc.updaterErr
			})

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
