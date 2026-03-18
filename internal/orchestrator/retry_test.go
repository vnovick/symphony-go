package orchestrator_test

import (
	"testing"
	"time"

	"github.com/vnovick/symphony-go/internal/orchestrator"
	"github.com/stretchr/testify/assert"
)

func TestBackoffMsContinuation(t *testing.T) {
	assert.Equal(t, 10000, orchestrator.BackoffMs(1, 300000))
}

func TestBackoffMsAttempt2(t *testing.T) {
	assert.Equal(t, 20000, orchestrator.BackoffMs(2, 300000))
}

func TestBackoffMsAttempt3(t *testing.T) {
	assert.Equal(t, 40000, orchestrator.BackoffMs(3, 300000))
}

func TestBackoffMsCappedAtMax(t *testing.T) {
	assert.Equal(t, 5000, orchestrator.BackoffMs(10, 5000))
}

func TestScheduleRetry(t *testing.T) {
	cfg := baseConfig()
	state := orchestrator.NewState(cfg)
	now := time.Now()
	state = orchestrator.ScheduleRetry(state, "id1", 1, "ENG-1", "some error", now, 10000)
	entry, ok := state.RetryAttempts["id1"]
	assert.True(t, ok)
	assert.Equal(t, "id1", entry.IssueID)
	assert.Equal(t, 1, entry.Attempt)
	assert.NotNil(t, entry.Error)
	assert.Contains(t, *entry.Error, "some error")
	_, claimed := state.Claimed["id1"]
	assert.True(t, claimed)
}

func TestCancelRetry(t *testing.T) {
	cfg := baseConfig()
	state := orchestrator.NewState(cfg)
	now := time.Now()
	state = orchestrator.ScheduleRetry(state, "id1", 1, "ENG-1", "", now, 10000)
	state = orchestrator.CancelRetry(state, "id1")
	_, ok := state.RetryAttempts["id1"]
	assert.False(t, ok)
	_, claimed := state.Claimed["id1"]
	assert.False(t, claimed)
}
