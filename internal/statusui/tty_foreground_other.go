//go:build !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !aix && !zos

package statusui

import (
	"context"
	"errors"
)

var errNoControllingTTY = errors.New("statusui: no controlling tty")

func waitForForegroundTTY(context.Context) error {
	return nil
}
