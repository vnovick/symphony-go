package linear_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vnovick/symphony-go/internal/tracker"
	"github.com/vnovick/symphony-go/internal/tracker/linear"
)

func linearIssueNode(id, identifier, state string) map[string]interface{} {
	return map[string]interface{}{
		"id":               id,
		"identifier":       identifier,
		"title":            "Issue " + identifier,
		"state":            map[string]interface{}{"name": state},
		"labels":           map[string]interface{}{"nodes": []interface{}{}},
		"inverseRelations": map[string]interface{}{"nodes": []interface{}{}},
		"createdAt":        "2024-01-01T00:00:00Z",
		"updatedAt":        "2024-01-01T00:00:00Z",
	}
}

func singlePageResponse(nodes []map[string]interface{}) map[string]interface{} {
	rawNodes := make([]interface{}, len(nodes))
	for i, n := range nodes {
		rawNodes[i] = n
	}
	return map[string]interface{}{
		"data": map[string]interface{}{
			"issues": map[string]interface{}{
				"nodes": rawNodes,
				"pageInfo": map[string]interface{}{
					"hasNextPage": false,
					"endCursor":   nil,
				},
			},
		},
	}
}

func serveJSON(t *testing.T, responses []map[string]interface{}) *httptest.Server {
	t.Helper()
	callCount := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := callCount
		callCount++
		if idx >= len(responses) {
			t.Errorf("unexpected request %d (only %d responses configured)", idx+1, len(responses))
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses[idx])
	}))
}

func TestFetchCandidateIssuesSinglePage(t *testing.T) {
	nodes := []map[string]interface{}{
		linearIssueNode("id-1", "ENG-1", "Todo"),
		linearIssueNode("id-2", "ENG-2", "In Progress"),
	}
	srv := serveJSON(t, []map[string]interface{}{singlePageResponse(nodes)})
	defer srv.Close()

	client := linear.NewClient(linear.ClientConfig{
		APIKey:         "test-key",
		ProjectSlug:    "my-project",
		ActiveStates:   []string{"Todo", "In Progress"},
		TerminalStates: []string{"Done"},
		Endpoint:       srv.URL,
	})

	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	assert.Len(t, issues, 2)
	assert.Equal(t, "ENG-1", issues[0].Identifier)
	assert.Equal(t, "ENG-2", issues[1].Identifier)
}

func TestFetchCandidateIssuesPaginated(t *testing.T) {
	page1 := map[string]interface{}{
		"data": map[string]interface{}{
			"issues": map[string]interface{}{
				"nodes": []interface{}{linearIssueNode("id-1", "ENG-1", "Todo")},
				"pageInfo": map[string]interface{}{
					"hasNextPage": true,
					"endCursor":   "cursor-abc",
				},
			},
		},
	}
	page2 := map[string]interface{}{
		"data": map[string]interface{}{
			"issues": map[string]interface{}{
				"nodes": []interface{}{linearIssueNode("id-2", "ENG-2", "Todo")},
				"pageInfo": map[string]interface{}{
					"hasNextPage": false,
					"endCursor":   nil,
				},
			},
		},
	}
	srv := serveJSON(t, []map[string]interface{}{page1, page2})
	defer srv.Close()

	client := linear.NewClient(linear.ClientConfig{
		APIKey:       "test-key",
		ProjectSlug:  "my-project",
		ActiveStates: []string{"Todo"},
		Endpoint:     srv.URL,
	})

	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	assert.Len(t, issues, 2)
	assert.Equal(t, "ENG-1", issues[0].Identifier)
	assert.Equal(t, "ENG-2", issues[1].Identifier)
}

func TestFetchIssuesByStatesEmptyReturnsEmpty(t *testing.T) {
	client := linear.NewClient(linear.ClientConfig{
		APIKey:      "test-key",
		ProjectSlug: "my-project",
		Endpoint:    "http://should-not-be-called.invalid",
	})
	result, err := client.FetchIssuesByStates(context.Background(), []string{})
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestFetchIssuesByStatesPaginated(t *testing.T) {
	page1Resp := `{"data":{"issues":{"nodes":[{"id":"i1","identifier":"ABC-1","title":"T1","state":{"name":"Done"},"labels":{"nodes":[]},"inverseRelations":{"nodes":[]}}],"pageInfo":{"hasNextPage":true,"endCursor":"cur1"}}}}`
	page2Resp := `{"data":{"issues":{"nodes":[{"id":"i2","identifier":"ABC-2","title":"T2","state":{"name":"Cancelled"},"labels":{"nodes":[]},"inverseRelations":{"nodes":[]}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = fmt.Fprint(w, page1Resp)
		} else {
			_, _ = fmt.Fprint(w, page2Resp)
		}
	}))
	defer srv.Close()

	client := linear.NewClient(linear.ClientConfig{
		APIKey:       "tok",
		ProjectSlug:  "proj",
		ActiveStates: []string{"Done", "Cancelled"},
		Endpoint:     srv.URL,
	})
	issues, err := client.FetchIssuesByStates(context.Background(), []string{"Done", "Cancelled"})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(issues))
	assert.Equal(t, "i1", issues[0].ID)
	assert.Equal(t, "i2", issues[1].ID)
}

func TestFetchIssueStatesByIDs(t *testing.T) {
	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"issues": map[string]interface{}{
				"nodes": []interface{}{
					linearIssueNode("id-1", "ENG-1", "In Progress"),
				},
			},
		},
	}
	srv := serveJSON(t, []map[string]interface{}{resp})
	defer srv.Close()

	client := linear.NewClient(linear.ClientConfig{
		APIKey:      "test-key",
		ProjectSlug: "my-project",
		Endpoint:    srv.URL,
	})

	result, err := client.FetchIssueStatesByIDs(context.Background(), []string{"id-1"})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "In Progress", result[0].State)
}

func TestFetchCandidateIssuesNon200Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	client := linear.NewClient(linear.ClientConfig{
		APIKey:      "test-key",
		ProjectSlug: "my-project",
		Endpoint:    srv.URL,
	})
	_, err := client.FetchCandidateIssues(context.Background())
	require.Error(t, err)
	var apiErr *tracker.APIStatusError
	require.True(t, errors.As(err, &apiErr), "expected *tracker.APIStatusError, got %T: %v", err, err)
	assert.Equal(t, "linear", apiErr.Adapter)
	assert.Equal(t, 401, apiErr.Status)
}

func TestFetchCandidateIssuesGraphQLErrors(t *testing.T) {
	resp := map[string]interface{}{
		"errors": []interface{}{
			map[string]interface{}{"message": "Not authorized"},
		},
	}
	srv := serveJSON(t, []map[string]interface{}{resp})
	defer srv.Close()

	client := linear.NewClient(linear.ClientConfig{
		APIKey:      "test-key",
		ProjectSlug: "my-project",
		Endpoint:    srv.URL,
	})
	_, err := client.FetchCandidateIssues(context.Background())
	require.Error(t, err)
	var gqlErr *tracker.GraphQLError
	require.True(t, errors.As(err, &gqlErr), "expected *tracker.GraphQLError, got %T: %v", err, err)
	assert.Contains(t, gqlErr.Message, "Not authorized")
}

func TestFetchCandidateIssuesUnknownPayload(t *testing.T) {
	resp := map[string]interface{}{"unexpected": "payload"}
	srv := serveJSON(t, []map[string]interface{}{resp})
	defer srv.Close()

	client := linear.NewClient(linear.ClientConfig{
		APIKey:      "test-key",
		ProjectSlug: "my-project",
		Endpoint:    srv.URL,
	})
	_, err := client.FetchCandidateIssues(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear_unknown_payload")
}

func TestFetchCandidateIssuesMissingEndCursor(t *testing.T) {
	page1 := map[string]interface{}{
		"data": map[string]interface{}{
			"issues": map[string]interface{}{
				"nodes": []interface{}{linearIssueNode("id-1", "ENG-1", "Todo")},
				"pageInfo": map[string]interface{}{
					"hasNextPage": true,
					// endCursor absent
				},
			},
		},
	}
	srv := serveJSON(t, []map[string]interface{}{page1})
	defer srv.Close()

	client := linear.NewClient(linear.ClientConfig{
		APIKey:       "test-key",
		ProjectSlug:  "my-project",
		ActiveStates: []string{"Todo"},
		Endpoint:     srv.URL,
	})
	_, err := client.FetchCandidateIssues(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "linear_missing_end_cursor")
}

func TestNormalizeLabelsLowercase(t *testing.T) {
	node := linearIssueNode("id-1", "ENG-1", "Todo")
	node["labels"] = map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{"name": "BUG"},
			map[string]interface{}{"name": "BackEnd"},
		},
	}
	resp := singlePageResponse([]map[string]interface{}{node})
	srv := serveJSON(t, []map[string]interface{}{resp})
	defer srv.Close()

	client := linear.NewClient(linear.ClientConfig{
		APIKey:       "test-key",
		ProjectSlug:  "my-project",
		ActiveStates: []string{"Todo"},
		Endpoint:     srv.URL,
	})
	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)
	assert.Equal(t, []string{"bug", "backend"}, issues[0].Labels)
}

func TestNormalizeBlockersFromInverseRelations(t *testing.T) {
	node := linearIssueNode("id-1", "ENG-1", "Todo")
	node["inverseRelations"] = map[string]interface{}{
		"nodes": []interface{}{
			map[string]interface{}{
				"type": "blocks",
				"issue": map[string]interface{}{
					"id":         "blocker-id",
					"identifier": "ENG-0",
					"state":      map[string]interface{}{"name": "In Progress"},
				},
			},
			map[string]interface{}{
				"type": "duplicate",
				"issue": map[string]interface{}{
					"id":         "other-id",
					"identifier": "ENG-9",
					"state":      map[string]interface{}{"name": "Done"},
				},
			},
		},
	}
	resp := singlePageResponse([]map[string]interface{}{node})
	srv := serveJSON(t, []map[string]interface{}{resp})
	defer srv.Close()

	client := linear.NewClient(linear.ClientConfig{
		APIKey:       "test-key",
		ProjectSlug:  "my-project",
		ActiveStates: []string{"Todo"},
		Endpoint:     srv.URL,
	})
	issues, err := client.FetchCandidateIssues(context.Background())
	require.NoError(t, err)
	require.Len(t, issues, 1)
	require.Len(t, issues[0].BlockedBy, 1)
	assert.Equal(t, "blocker-id", *issues[0].BlockedBy[0].ID)
	assert.Equal(t, "ENG-0", *issues[0].BlockedBy[0].Identifier)
}
