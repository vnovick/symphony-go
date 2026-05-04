//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris || aix || zos

package statusui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

var errNoControllingTTY = errors.New("statusui: no controlling tty")

const foregroundTTYCheckInterval = 10 * time.Millisecond

func waitForForegroundTTY(ctx context.Context) error {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return fmt.Errorf("%w: %w", errNoControllingTTY, err)
	}
	defer tty.Close() //nolint:errcheck

	return waitForForegroundProcessGroup(
		ctx,
		unix.Getpgrp,
		func() (int, error) {
			return unix.IoctlGetInt(int(tty.Fd()), unix.TIOCGPGRP)
		},
		foregroundTTYCheckInterval,
	)
}

func waitForForegroundProcessGroup(
	ctx context.Context,
	currentPGID func() int,
	foregroundPGID func() (int, error),
	interval time.Duration,
) error {
	if interval <= 0 {
		interval = foregroundTTYCheckInterval
	}

	for {
		foreground, err := foregroundPGID()
		if err != nil {
			return fmt.Errorf("statusui: foreground process group: %w", err)
		}
		if foreground == currentPGID() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
