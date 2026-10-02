// Package window is the on-screen overlay: an X11 ARGB window, or a
// wlr-layer-shell surface when running under Wayland.
package window

import (
	"fmt"
	"os"
)

// Kind is the type of a pointer Event.
type Kind uint8

const (
	Press Kind = iota
	Release
	Motion
	Closed
)

// Event is a pointer event on the window. X, Y are window-local; RootX, RootY
// are screen coordinates (same space as Pos).
type Event struct {
	Kind         Kind
	Button       uint8 // 1 = left
	X, Y         int
	RootX, RootY int
}

// Window is a transparent, click-through-by-default overlay.
type Window interface {
	Size() (w, h int)
	Pos() (x, y int)
	// PutImage draws a full-window premultiplied BGRA frame.
	PutImage(bgra []byte)
	// Poll returns the next pending pointer event without blocking.
	Poll() (Event, bool)
	// SetClickThrough lets clicks pass through (true) or captures them (false).
	SetClickThrough(on bool) error
	Move(x, y int)
	MoveResize(x, y, w, h int)
	Destroy()
}

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
