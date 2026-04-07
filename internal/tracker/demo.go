package tracker

import (
	"fmt"
	"time"

	"github.com/vnovick/itervox/internal/domain"
)

// GenerateDemoIssues creates a set of synthetic issues for demo mode.
// The issues span multiple states to populate the dashboard realistically.
func GenerateDemoIssues(n int) []domain.Issue {
	titles := []string{
		"Fix authentication race condition in login flow",
		"Add dark mode toggle to settings panel",
		"Migrate user service to gRPC",
		"Implement rate limiting for public API",
		"Refactor button component to use CVA",
		"Add Redis caching to auth token flow",
		"Set up Terraform modules for staging environment",
		"Implement responsive sidebar navigation",
		"Fix pagination in search results page",
		"Update API rate limiting documentation",
		"Add E2E tests for checkout flow",
		"Optimize database queries for dashboard",
		"Implement WebSocket notifications",
		"Add CORS configuration for new origins",
		"Fix memory leak in image processing worker",
	}
	states := []string{"Todo", "Todo", "Todo", "In Progress", "In Progress"}
	priorities := []int{1, 2, 2, 3, 1}
	now := time.Now()

	issues := make([]domain.Issue, 0, n)
	for i := range n {
		title := titles[i%len(titles)]
		state := states[i%len(states)]
		prio := priorities[i%len(priorities)]
		created := now.Add(-time.Duration(n-i) * time.Hour)
		desc := fmt.Sprintf("This is a demo issue for testing. Priority: %d.", prio)
		url := fmt.Sprintf("https://example.com/issues/DEMO-%d", i+1)

		issues = append(issues, domain.Issue{
			ID:          fmt.Sprintf("demo-id-%d", i+1),
			Identifier:  fmt.Sprintf("DEMO-%d", i+1),
			Title:       title,
			State:       state,
			Description: &desc,
			URL:         &url,
			Priority:    &prio,
			CreatedAt:   &created,
			Labels:      []string{"demo"},
		})
	}
	return issues
}
