package xwindow

import (
	"encoding/binary"
	"fmt"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/shape"
	"github.com/jezek/xgb/xproto"
)

// ARGBWindow is an X11 window with a 32-bit ARGB visual for per-pixel alpha.
// A compositing manager (e.g. picom) must be running for alpha to be visible.
type ARGBWindow struct {
	Conn            *xgb.Conn
	Wid             xproto.Window
	gc              xproto.Gcontext
	W, H            uint16
	X, Y            int16
	maxRows         int
	maxRequestBytes int // MaximumRequestLength*4 - 24
}

// New creates a centered ARGB X11 window. Pass x=-1, y=-1 to center.
func New(x, y, width, height int) (*ARGBWindow, error) {
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("xwindow: connect: %w", err)
	}

	if err := shape.Init(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("xwindow: shape init: %w", err)
	}

	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)

	// Find 32-bit TrueColor visual.
	var visualID xproto.Visualid
	found := false
	for _, depth := range screen.AllowedDepths {
		if depth.Depth != 32 {
			continue
		}
		for _, vis := range depth.Visuals {
			if vis.Class == xproto.VisualClassTrueColor {
				visualID = vis.VisualId
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		conn.Close()
		return nil, fmt.Errorf("xwindow: no 32-bit TrueColor visual available")
	}

	root := screen.Root
	w, h := uint16(width), uint16(height)

	if x < 0 {
		x = (int(screen.WidthInPixels) - width) / 2
	}
	if y < 0 {
		y = (int(screen.HeightInPixels) - height) / 2
	}

	colormapID, err := xproto.NewColormapId(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("xwindow: new colormap id: %w", err)
	}
	xproto.CreateColormap(conn, xproto.ColormapAllocNone, colormapID, root, visualID)

	wid, err := xproto.NewWindowId(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("xwindow: new window id: %w", err)
	}

	// ValueList must be in ascending CW bit order.
	// CwBackPixel=2, CwBorderPixel=8, CwOverrideRedirect=512, CwEventMask=2048, CwColormap=8192
	mask := uint32(xproto.CwBackPixel | xproto.CwBorderPixel | xproto.CwOverrideRedirect |
		xproto.CwEventMask | xproto.CwColormap)
	eventMask := uint32(xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease |
		xproto.EventMaskPointerMotion | xproto.EventMaskStructureNotify)
	values := []uint32{0, 0, 1, eventMask, uint32(colormapID)}

	err = xproto.CreateWindowChecked(conn, 32, wid, root,
		int16(x), int16(y), w, h, 0,
		xproto.WindowClassInputOutput, visualID,
		mask, values).Check()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("xwindow: create window: %w", err)
	}

	// Tell compositor not to bypass compositing for this window.
	bypassAtom, err := internAtom(conn, "_NET_WM_BYPASS_COMPOSITOR")
	if err == nil {
		cardAtom, err2 := internAtom(conn, "CARDINAL")
		if err2 == nil {
			var val [4]byte
			binary.LittleEndian.PutUint32(val[:], 0)
			xproto.ChangeProperty(conn, xproto.PropModeReplace, wid,
				bypassAtom, cardAtom, 32, 1, val[:])
		}
	}

	xproto.MapWindow(conn, wid)

	gc, err := xproto.NewGcontextId(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("xwindow: new gc id: %w", err)
	}
	xproto.CreateGC(conn, gc, xproto.Drawable(wid), 0, nil)

	maxRequestBytes := int(setup.MaximumRequestLength)*4 - 24
	maxRows := maxRequestBytes / (width * 4)
	if maxRows < 1 {
		maxRows = 1
	}

	return &ARGBWindow{
		Conn:            conn,
		Wid:             wid,
		gc:              gc,
		W:               w,
		H:               h,
		X:               int16(x),
		Y:               int16(y),
		maxRows:         maxRows,
		maxRequestBytes: maxRequestBytes,
	}, nil
}

// PutImage renders a full-window BGRA frame, chunked to fit request limits.
func (w *ARGBWindow) PutImage(data []byte) {
	rowBytes := int(w.W) * 4
	totalRows := int(w.H)
	for dstY := 0; dstY < totalRows; {
		rows := w.maxRows
		if dstY+rows > totalRows {
			rows = totalRows - dstY
		}
		chunk := data[dstY*rowBytes : (dstY+rows)*rowBytes]
		xproto.PutImage(w.Conn, xproto.ImageFormatZPixmap, xproto.Drawable(w.Wid), w.gc,
			w.W, uint16(rows), 0, int16(dstY), 0, 32, chunk)
		dstY += rows
	}
}

// PollEvent returns the next pending X11 event without blocking, or nil.
func (w *ARGBWindow) PollEvent() xgb.Event {
	ev, _ := w.Conn.PollForEvent()
	return ev
}

// Move repositions the window.
func (w *ARGBWindow) Move(x, y int16) {
	w.X, w.Y = x, y
	xproto.ConfigureWindow(w.Conn, w.Wid,
		xproto.ConfigWindowX|xproto.ConfigWindowY,
		[]uint32{uint32(x), uint32(y)})
}

// MoveResize repositions and resizes the window in one round trip.
func (w *ARGBWindow) MoveResize(x, y int16, newW, newH uint16) {
	w.X, w.Y = x, y
	w.W, w.H = newW, newH
	w.maxRows = w.maxRequestBytes / (int(newW) * 4)
	if w.maxRows < 1 {
		w.maxRows = 1
	}
	xproto.ConfigureWindow(w.Conn, w.Wid,
		xproto.ConfigWindowX|xproto.ConfigWindowY|
			xproto.ConfigWindowWidth|xproto.ConfigWindowHeight,
		[]uint32{uint32(x), uint32(y), uint32(newW), uint32(newH)})
}

// Destroy frees all X11 resources.
func (w *ARGBWindow) Destroy() {
	xproto.FreeGC(w.Conn, w.gc)
	xproto.DestroyWindow(w.Conn, w.Wid)
	w.Conn.Close()
}

func internAtom(conn *xgb.Conn, name string) (xproto.Atom, error) {
	reply, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
	if err != nil {
		return 0, err
	}
	return reply.Atom, nil
}
