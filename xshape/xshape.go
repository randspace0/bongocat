package xshape

import (
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/shape"
	"github.com/jezek/xgb/xproto"
)

// MakeClickThrough removes the input shape so clicks pass through the window.
func MakeClickThrough(conn *xgb.Conn, wid xproto.Window) error {
	return shape.RectanglesChecked(conn, shape.SoSet, shape.SkInput,
		xproto.ClipOrderingUnsorted, wid, 0, 0, []xproto.Rectangle{}).Check()
}

// ResetClickThrough restores full-window input coverage.
func ResetClickThrough(conn *xgb.Conn, wid xproto.Window) error {
	return shape.MaskChecked(conn, shape.SoSet, shape.SkInput,
		wid, 0, 0, xproto.PixmapNone).Check()
}
