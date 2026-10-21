//go:build !windows

package window

import (
	"fmt"
	"os"
)

// IsWayland reports whether the session is Wayland.
func IsWayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != "" && os.Getenv("XDG_SESSION_TYPE") != "x11"
}

// New opens a window of the given size, centered on screen.
func New(width, height int) (Window, error) {
	var (
		w   Window
		err error
	)
	if IsWayland() {
		w, err = newWayland(width, height)
	} else {
		w, err = newX11(width, height)
	}
	if err != nil {
		return nil, fmt.Errorf("window: %w", err)
	}
	return w, nil
}
