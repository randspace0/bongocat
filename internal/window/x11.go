package window

import (
	"github.com/jezek/xgb/xproto"
	"github.com/randspace0/bongocat/internal/xwindow"
	"github.com/randspace0/bongocat/xshape"
)

type x11Window struct{ *xwindow.ARGBWindow }

func newX11(width, height int) (Window, error) {
	xwin, err := xwindow.New(-1, -1, width, height)
	if err != nil {
		return nil, err
	}
	if err := xshape.MakeClickThrough(xwin.Conn, xwin.Wid); err != nil {
		xwin.Destroy()
		return nil, err
	}
	return &x11Window{xwin}, nil
}

func (w *x11Window) Size() (int, int) { return int(w.W), int(w.H) }
func (w *x11Window) Pos() (int, int)  { return int(w.X), int(w.Y) }

func (w *x11Window) PutImage(bgra []byte) { w.ARGBWindow.PutImage(bgra) }

func (w *x11Window) SetClickThrough(on bool) error {
	if on {
		return xshape.MakeClickThrough(w.Conn, w.Wid)
	}
	return xshape.ResetClickThrough(w.Conn, w.Wid)
}

func (w *x11Window) Move(x, y int) { w.ARGBWindow.Move(int16(x), int16(y)) }

func (w *x11Window) MoveResize(x, y, width, height int) {
	w.ARGBWindow.MoveResize(int16(x), int16(y), uint16(width), uint16(height))
}

func (w *x11Window) Poll() (Event, bool) {
	for {
		switch e := w.PollEvent().(type) {
		case nil:
			return Event{}, false
		case xproto.DestroyNotifyEvent:
			return Event{Kind: Closed}, true
		case xproto.ButtonPressEvent:
			return Event{Kind: Press, Button: uint8(e.Detail), X: int(e.EventX), Y: int(e.EventY),
				RootX: int(e.RootX), RootY: int(e.RootY)}, true
		case xproto.ButtonReleaseEvent:
			return Event{Kind: Release, Button: uint8(e.Detail), X: int(e.EventX), Y: int(e.EventY),
				RootX: int(e.RootX), RootY: int(e.RootY)}, true
		case xproto.MotionNotifyEvent:
			return Event{Kind: Motion, X: int(e.EventX), Y: int(e.EventY),
				RootX: int(e.RootX), RootY: int(e.RootY)}, true
		}
	}
}
