// Package window is the on-screen overlay: an X11 ARGB window, or a
// wlr-layer-shell surface when running under Wayland.
package window

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
