package github_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	ghclient "github.com/vnovick/symphony-go/internal/tracker/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ghIssue(number int, title, state string, labels []string) map[string]interface{} {
	labelObjs := make([]interface{}, len(labels))
	for i, l := range labels {
		labelObjs[i] = map[string]interface{}{"name": l}
	}
	return map[string]interface{}{
		"number":     float64(number),
		"title":      title,
		"state":      state,
		"labels":     labelObjs,
		"html_url":   fmt.Sprintf("https://github.com/owner/repo/issues/%d", number),
		"body":       "",
		"created_at": "2024-01-01T00:00:00Z",
		"updated_at": "2024-01-01T00:00:00Z",
	}
}

type ghServer struct {
	t         *testing.T
	responses []struct {
		body    interface{}
		headers map[string]string
		status  int
	}
	calls int
}

func newGHServer(t *testing.T) *ghServer {
	return &ghServer{t: t}
}

func (s *ghServer) addResponse(body interface{}, linkHeader string, status int) {
	headers := map[string]string{}
	if linkHeader != "" {
		headers["Link"] = linkHeader
	}
	s.responses = append(s.responses, struct {
		body    interface{}
		headers map[string]string
		status  int
	}{body, headers, status})
}

func (s *ghServer) serve() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := s.calls
		s.calls++
		if idx >= len(s.responses) {
			s.t.Errorf("unexpected call %d", idx+1)
			w.WriteHeader(500)
			return
		}
		resp := s.responses[idx]
		for k, v := range resp.headers {
			w.Header().Set(k, v)
		}
		w.Header().Set("Content-Type", "application/json")
		if resp.status != 0 {
			w.WriteHeader(resp.status)
		}
		_ = json.NewEncoder(w).Encode(resp.body)
	}))
}

func defaultConfig(endpoint string) ghclient.ClientConfig {
	return ghclient.ClientConfig{
		APIKey:         "ghp_test",
		ProjectSlug:    "owner/repo",
		ActiveStates:   []string{"todo", "in progress"},
		TerminalStates: []string{"closed"},
		Endpoint:       endpoint,
	}
}

func TestGHFetchCandidateIssuesSinglePage(t *testing.T) {
	// One request per active state label ("todo", "in progress").
	// Each returns its own issue; deduplication keeps both.
	srv := newGHServer(t)
	srv.addResponse([]interface{}{ghIssue(1, "Fix bug", "open", []string{"todo"})}, "", 0)
	srv.addResponse([]interface{}{ghIssue(2, "Add feature", "open", []string{"in progress"})}, "", 0)
	ts := srv.serve()
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	assert.Len(t, issues, 2)
	assert.Equal(t, "#1", issues[0].Identifier)
	assert.Equal(t, "#2", issues[1].Identifier)
}

func TestGHFetchCandidateIssuesPaginatedLinkHeader(t *testing.T) {
	mux := http.NewServeMux()
	callCount := 0
	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		if page == "2" || callCount > 0 {
			callCount++
			_ = json.NewEncoder(w).Encode([]interface{}{ghIssue(2, "Issue 2", "open", []string{"todo"})})
			return
		}
		callCount++
		nextURL := fmt.Sprintf("http://%s/repos/owner/repo/issues?page=2", r.Host)
		w.Header().Set("Link", fmt.Sprintf(`<%s>; rel="next"`, nextURL))
		_ = json.NewEncoder(w).Encode([]interface{}{ghIssue(1, "Issue 1", "open", []string{"todo"})})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	assert.Len(t, issues, 2)
	assert.Equal(t, "#1", issues[0].Identifier)
	assert.Equal(t, "#2", issues[1].Identifier)
}

func TestGHFetchIssuesByStatesEmptyReturnsEmpty(t *testing.T) {
	client := ghclient.NewClient(defaultConfig("http://should-not-be-called.invalid"))
	result, err := client.FetchIssuesByStates(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGHFetchIssuesByStatesClosedState(t *testing.T) {
	srv := newGHServer(t)
	srv.addResponse([]interface{}{
		ghIssue(10, "Closed issue", "closed", []string{}),
	}, "", 0)
	ts := srv.serve()
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	result, err := client.FetchIssuesByStates(context.Background(), []string{"closed"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "closed", result[0].State)
}

func TestGHFetchIssuesByStatesPaginated(t *testing.T) {
	page1Issues := `[{"number":1,"title":"Issue 1","state":"open","body":"","html_url":"","labels":[{"name":"done"}],"created_at":"2024-01-01T00:00:00Z","updated_at":"2024-01-01T00:00:00Z"}]`
	page2Issues := `[{"number":2,"title":"Issue 2","state":"open","body":"","html_url":"","labels":[{"name":"done"}],"created_at":"2024-01-02T00:00:00Z","updated_at":"2024-01-02T00:00:00Z"}]`
	calls := 0
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos/o/r/issues?page=2>; rel="next"`, ts.URL))
			_, _ = fmt.Fprint(w, page1Issues)
		} else {
			_, _ = fmt.Fprint(w, page2Issues)
		}
	}))
	defer ts.Close()

	client := ghclient.NewClient(ghclient.ClientConfig{
		APIKey:       "tok",
		ProjectSlug:  "o/r",
		ActiveStates: []string{"done"},
		Endpoint:     ts.URL,
	})
	issues, err := client.FetchIssuesByStates(context.Background(), []string{"done"})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(issues))
}

func TestGHFetchIssueStatesByIDsFanOut(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ghIssue(1, "Issue 1", "open", []string{"in progress"}))
	})
	mux.HandleFunc("/repos/owner/repo/issues/2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ghIssue(2, "Issue 2", "open", []string{"todo"}))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	result, err := client.FetchIssueStatesByIDs(context.Background(), []string{"1", "2"})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestGHFetchIssueStatesByIDs404Skipped(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ghIssue(1, "Issue 1", "open", []string{"todo"}))
	})
	mux.HandleFunc("/repos/owner/repo/issues/2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	result, err := client.FetchIssueStatesByIDs(context.Background(), []string{"1", "2"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "1", result[0].ID)
}

func TestGHFetchIssueStatesByIDsEmptyReturnsEmpty(t *testing.T) {
	client := ghclient.NewClient(defaultConfig("http://should-not-be-called.invalid"))
	result, err := client.FetchIssueStatesByIDs(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestGHNormalizeClosedIssueAlwaysTerminal(t *testing.T) {
	srv := newGHServer(t)
	srv.addResponse([]interface{}{
		ghIssue(5, "Was open", "closed", []string{"todo"}),
	}, "", 0)
	ts := srv.serve()
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	result, err := client.FetchIssuesByStates(context.Background(), []string{"closed"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "closed", result[0].State)
}

func TestGHNormalizeLabelsLowercase(t *testing.T) {
	srv := newGHServer(t)
	srv.addResponse([]interface{}{
		ghIssue(1, "Issue 1", "open", []string{"TODO", "Backend"}),
	}, "", 0)
	srv.addResponse([]interface{}{}, "", 0) // second active-state request ("in progress")
	ts := srv.serve()
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, []string{"todo", "backend"}, issues[0].Labels)
}

func TestGHNormalizeBlockersParsedFromBody(t *testing.T) {
	mux := http.NewServeMux()
	var listCalls atomic.Int32
	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if listCalls.Add(1) == 1 {
			issue := ghIssue(3, "Issue with blocker", "open", []string{"todo"})
			issue["body"] = "This is blocked by #10 and also blocked by #20."
			_ = json.NewEncoder(w).Encode([]interface{}{issue})
		} else {
			_ = json.NewEncoder(w).Encode([]interface{}{})
		}
	})
	mux.HandleFunc("/repos/owner/repo/issues/10", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ghIssue(10, "Blocker 10", "open", []string{"in progress"}))
	})
	mux.HandleFunc("/repos/owner/repo/issues/20", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ghIssue(20, "Blocker 20", "closed", []string{}))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Len(t, issues[0].BlockedBy, 2)
	assert.Equal(t, "#10", *issues[0].BlockedBy[0].Identifier)
	assert.Equal(t, "#20", *issues[0].BlockedBy[1].Identifier)
	// States must be populated — dispatch enforcement depends on this
	require.NotNil(t, issues[0].BlockedBy[0].State, "blocker #10 state must be set")
	assert.Equal(t, "in progress", *issues[0].BlockedBy[0].State)
	require.NotNil(t, issues[0].BlockedBy[1].State, "blocker #20 state must be set")
	assert.Equal(t, "closed", *issues[0].BlockedBy[1].State)
}

func TestGHBlockerStateMissingBlockerTreatedAsClosed(t *testing.T) {
	mux := http.NewServeMux()
	var listCalls atomic.Int32
	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if listCalls.Add(1) == 1 {
			issue := ghIssue(1, "Blocked issue", "open", []string{"todo"})
			issue["body"] = "Blocked by #99"
			_ = json.NewEncoder(w).Encode([]interface{}{issue})
		} else {
			_ = json.NewEncoder(w).Encode([]interface{}{})
		}
	})
	mux.HandleFunc("/repos/owner/repo/issues/99", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Len(t, issues[0].BlockedBy, 1)
	require.NotNil(t, issues[0].BlockedBy[0].State, "deleted blocker must default to closed")
	assert.Equal(t, "closed", *issues[0].BlockedBy[0].State)
}

func TestGHRateLimitsCaptured(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-RateLimit-Limit", "5000")
		w.Header().Set("X-RateLimit-Remaining", "4999")
		w.Header().Set("X-RateLimit-Reset", "1700000000")
		_ = json.NewEncoder(w).Encode(ghIssue(42, "Issue 42", "open", []string{"todo"}))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	limit0, _, _ := client.RateLimits()
	assert.Zero(t, limit0, "no rate limit observed yet")

	_, err := client.FetchIssueStatesByIDs(context.Background(), []string{"42"})
	require.NoError(t, err)

	limit, remaining, reset := client.RateLimits()
	assert.Equal(t, 5000, limit)
	assert.Equal(t, 4999, remaining)
	require.NotNil(t, reset)
	assert.Equal(t, int64(1700000000), reset.Unix())
}

// Ensure deduplicated blockers across issues only generate one fetch per unique ID.
func TestGHBlockerStateDeduplication(t *testing.T) {
	var mu sync.Mutex
	fetchCounts := map[string]int{}
	mux := http.NewServeMux()
	var listCalls atomic.Int32
	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if listCalls.Add(1) == 1 {
			i1 := ghIssue(1, "Issue A", "open", []string{"todo"})
			i1["body"] = "Blocked by #5"
			i2 := ghIssue(2, "Issue B", "open", []string{"todo"})
			i2["body"] = "Blocked by #5" // same blocker
			_ = json.NewEncoder(w).Encode([]interface{}{i1, i2})
		} else {
			_ = json.NewEncoder(w).Encode([]interface{}{})
		}
	})
	mux.HandleFunc("/repos/owner/repo/issues/5", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fetchCounts["5"]++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ghIssue(5, "Blocker 5", "open", []string{"in progress"}))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 2)

	mu.Lock()
	count := fetchCounts["5"]
	mu.Unlock()
	assert.Equal(t, 1, count, "blocker #5 must be fetched exactly once despite two referencing issues")
}

func TestGHNon200FetchCandidateIssues(t *testing.T) {
	srv := newGHServer(t)
	srv.addResponse(nil, "", 401)
	ts := srv.serve()
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	_, err := client.FetchCandidateIssues(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github_api_status")
}

func TestGHIdentifierFormat(t *testing.T) {
	srv := newGHServer(t)
	srv.addResponse([]interface{}{ghIssue(42, "Issue 42", "open", []string{"todo"})}, "", 0)
	srv.addResponse([]interface{}{}, "", 0) // second active-state request ("in progress")
	ts := srv.serve()
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, "42", issues[0].ID)
	assert.Equal(t, "#42", issues[0].Identifier)
}

func TestGHIssueOpenWithNoMatchingLabelNotEligible(t *testing.T) {
	srv := newGHServer(t)
	// Request for "todo": returns one eligible and one non-matching issue.
	srv.addResponse([]interface{}{
		ghIssue(1, "Issue 1", "open", []string{"todo"}),
		ghIssue(2, "Issue 2", "open", []string{"unrelated"}),
	}, "", 0)
	// Request for "in progress": empty.
	srv.addResponse([]interface{}{}, "", 0)
	ts := srv.serve()
	defer ts.Close()

	client := ghclient.NewClient(defaultConfig(ts.URL))
	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	// "unrelated" label → deriveState returns "" → filtered by fetchPaginated
	assert.Len(t, issues, 1)
	assert.Equal(t, "#1", issues[0].Identifier)
}

func TestGHMissingPageLinkError(t *testing.T) {
	_, err := ghclient.ParseNextLink("bad-link-header-with-no-rel-next")
	assert.ErrorIs(t, err, ghclient.ErrMissingPageLink)

	// Empty header = no next page, not an error
	url, err := ghclient.ParseNextLink("")
	assert.NoError(t, err)
	assert.Empty(t, url)
}
