package main

import (
	"embed"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/randspace0/bongocat/internal/cat"
	"github.com/randspace0/bongocat/internal/input"
	"github.com/randspace0/bongocat/internal/renderer"
	"github.com/randspace0/bongocat/internal/tray"
	"github.com/randspace0/bongocat/internal/window"
	"github.com/randspace0/bongocat/xshape"
	"github.com/jezek/xgb/xproto"
)

//go:embed assets/base.png
var iconPNG []byte

//go:embed assets
var assetsFS embed.FS

// aspectRatio is the native width/height ratio, locked during resize.
const aspectRatio = float64(window.Width) / float64(window.Height)

// minWidth is the smallest the window can be resized to.
const minWidth = 120

// cornerZone is the pixel margin from each corner that activates resize.
const cornerZone = 16

type corner int

const (
	noCorner corner = iota
	cornerTL
	cornerTR
	cornerBL
	cornerBR
)

func detectCorner(ex, ey int16, w, h uint16) corner {
	x, y := int(ex), int(ey)
	ww, wh := int(w), int(h)
	inL := x < cornerZone
	inR := x >= ww-cornerZone
	inT := y < cornerZone
	inB := y >= wh-cornerZone
	switch {
	case inL && inT:
		return cornerTL
	case inR && inT:
		return cornerTR
	case inL && inB:
		return cornerBL
	case inR && inB:
		return cornerBR
	}
	return noCorner
}

func main() {
	win, err := window.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer win.Destroy()

	sprites, _ := fs.Sub(assetsFS, "assets")
	rnd, err := renderer.New(sprites)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer rnd.Destroy()

	c := cat.New()

	mon, err := input.New(func(ev input.Event) {
		switch ev.Type {
		case input.ButtonPress:
			if input.IsMouseLeft(ev.Detail) {
				c.LeftPress()
			} else if input.IsMouseRight(ev.Detail) {
				c.RightPress()
			}
		case input.ButtonRelease:
			if input.IsMouseLeft(ev.Detail) {
				c.LeftRelease()
			} else if input.IsMouseRight(ev.Detail) {
				c.RightRelease()
			}
		case input.KeyPress:
			if input.IsLeft(ev.Detail) {
				c.LeftPress()
			} else {
				c.RightPress()
			}
		case input.KeyRelease:
			if input.IsLeft(ev.Detail) {
				c.LeftRelease()
			} else {
				c.RightRelease()
			}
		}
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer mon.Stop()

	tr := tray.New(iconPNG)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	moveEnabled := false
	moveDragging := false
	resizeDragging := false
	activeCorner := noCorner

	var dragStartRootX, dragStartRootY int16
	var resizeStartX, resizeStartY int16
	var resizeStartW, resizeStartH uint16

	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			return
		case <-tr.QuitCh:
			return
		case enabled := <-tr.MoveCh:
			moveEnabled = enabled
			moveDragging = false
			resizeDragging = false
			if enabled {
				xshape.ResetClickThrough(win.Conn, win.Wid)
			} else {
				xshape.MakeClickThrough(win.Conn, win.Wid)
			}
		case <-ticker.C:
			for {
				ev := win.PollEvent()
				if ev == nil {
					break
				}
				switch e := ev.(type) {
				case xproto.DestroyNotifyEvent:
					return

				case xproto.ButtonPressEvent:
					if !moveEnabled || e.Detail != 1 {
						break
					}
					cn := detectCorner(e.EventX, e.EventY, win.W, win.H)
					if cn != noCorner {
						resizeDragging = true
						activeCorner = cn
						dragStartRootX = e.RootX
						dragStartRootY = e.RootY
						resizeStartX = win.X
						resizeStartY = win.Y
						resizeStartW = win.W
						resizeStartH = win.H
					} else {
						moveDragging = true
						dragStartRootX = e.RootX
						dragStartRootY = e.RootY
					}

				case xproto.ButtonReleaseEvent:
					if e.Detail == 1 {
						moveDragging = false
						resizeDragging = false
						activeCorner = noCorner
					}

				case xproto.MotionNotifyEvent:
					if moveDragging {
						win.Move(
							win.X+e.RootX-dragStartRootX,
							win.Y+e.RootY-dragStartRootY,
						)
						dragStartRootX = e.RootX
						dragStartRootY = e.RootY
					} else if resizeDragging {
						applyResize(win, activeCorner,
							e.RootX-dragStartRootX,
							resizeStartX, resizeStartY,
							resizeStartW, resizeStartH)
					}
				}
			}

			// Render at current window size, with border when move/resize is on.
			w, h := int(win.W), int(win.H)
			border := moveEnabled
			switch c.State() {
			case cat.LeftDown:
				win.PutImage(rnd.DrawLeftDown(w, h, border))
			case cat.RightDown:
				win.PutImage(rnd.DrawRightDown(w, h, border))
			case cat.BothDown:
				win.PutImage(rnd.DrawBothDown(w, h, border))
			default:
				win.PutImage(rnd.DrawIdle(w, h, border))
			}
		}
	}
}

// applyResize computes and applies a ratio-locked resize for the given corner.
// dx is the horizontal drag delta from the resize start.
func applyResize(win interface {
	MoveResize(x, y int16, w, h uint16)
}, c corner, dx int16,
	startX, startY int16, startW, startH uint16,
) {
	var newW int
	switch c {
	case cornerBR, cornerTR:
		newW = int(startW) + int(dx)
	case cornerBL, cornerTL:
		newW = int(startW) - int(dx)
	}
	if newW < minWidth {
		newW = minWidth
	}
	newH := int(math.Round(float64(newW) / aspectRatio))

	var newX, newY int16
	newX = startX
	newY = startY
	switch c {
	case cornerBL, cornerTL:
		// Right edge stays fixed.
		newX = startX + int16(int(startW)-newW)
	}
	switch c {
	case cornerTR, cornerTL:
		// Bottom edge stays fixed.
		newY = startY + int16(int(startH)-newH)
	}

	win.MoveResize(newX, newY, uint16(newW), uint16(newH))
}
