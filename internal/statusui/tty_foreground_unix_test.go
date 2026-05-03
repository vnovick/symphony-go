//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || aix || zos

package statusui

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWaitForForegroundProcessGroupReturnsImmediatelyWhenForeground(t *testing.T) {
	var calls int
	err := waitForForegroundProcessGroup(
		context.Background(),
		func() int { return 42 },
		func() (int, error) {
			calls++
			return 42, nil
		},
		time.Millisecond,
	)

	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestWaitForForegroundProcessGroupPollsUntilForeground(t *testing.T) {
	var calls int
	err := waitForForegroundProcessGroup(
		context.Background(),
		func() int { return 42 },
		func() (int, error) {
			calls++
			if calls < 3 {
				return 7, nil
			}
			return 42, nil
		},
		time.Millisecond,
	)

	require.NoError(t, err)
	require.Equal(t, 3, calls)
}

func TestWaitForForegroundProcessGroupExitsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := waitForForegroundProcessGroup(
		ctx,
		func() int { return 42 },
		func() (int, error) { return 7, nil },
		time.Millisecond,
	)

	require.ErrorIs(t, err, context.Canceled)
}

func TestWaitForForegroundProcessGroupReturnsForegroundError(t *testing.T) {
	want := errors.New("ioctl failed")
	err := waitForForegroundProcessGroup(
		context.Background(),
		func() int { return 42 },
		func() (int, error) { return 0, want },
		time.Millisecond,
	)

	require.ErrorIs(t, err, want)
}
