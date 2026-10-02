package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/randspace0/bongocat/internal/cat"
	"github.com/randspace0/bongocat/internal/config"
	"github.com/randspace0/bongocat/internal/input"
	"github.com/randspace0/bongocat/internal/renderer"
	"github.com/randspace0/bongocat/internal/tray"
	"github.com/randspace0/bongocat/internal/window"
)

//go:embed assets/skins/classic/base.png
var iconPNG []byte

//go:embed assets/skins
var skinsFS embed.FS

const defaultSkin = "classic"

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

func detectCorner(x, y, ww, wh int) corner {
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

func loadSkin(name string) (*renderer.Renderer, error) {
	dir, err := fs.Sub(skinsFS, "assets/skins/"+name)
	if err != nil {
		return nil, err
	}
	return renderer.New(dir)
}

func main() {
	skinName := flag.String("character", defaultSkin, "character to show (also selectable from the tray menu)")
	flag.Parse()

	cfg := config.Load()
	flagSet := false
	flag.Visit(func(f *flag.Flag) { flagSet = flagSet || f.Name == "character" })
	if !flagSet && cfg.Character != "" {
		*skinName = cfg.Character
	}

	skins, _ := fs.Sub(skinsFS, "assets/skins")
	skinNames, err := renderer.Skins(skins)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	rnd, err := loadSkin(*skinName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: character %q: %v\n", *skinName, err)
		os.Exit(1)
	}
	nativeW, nativeH := rnd.Size()
	// aspectRatio is the skin's width/height ratio, locked during resize.
	aspectRatio := float64(nativeW) / float64(nativeH)

	win, err := window.New(nativeW, nativeH)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer win.Destroy()

	currentSkin := *skinName
	if cfg.Width >= minWidth {
		win.MoveResize(cfg.X, cfg.Y, cfg.Width, int(math.Round(float64(cfg.Width)*float64(nativeH)/float64(nativeW))))
	}
	defer func() {
		x, y := win.Pos()
		w, _ := win.Size()
		if err := config.Save(config.Config{Character: currentSkin, X: x, Y: y, Width: w}); err != nil {
			fmt.Fprintln(os.Stderr, "error: save config:", err)
		}
	}()

	c := cat.New()

	newMonitor := func(h func(input.Event)) (interface{ Stop() }, error) { return input.New(h) }
	if window.IsWayland() {
		newMonitor = func(h func(input.Event)) (interface{ Stop() }, error) { return input.NewEvdev(h) }
	}
	mon, err := newMonitor(func(ev input.Event) {
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

	tr := tray.New(iconPNG, skinNames, *skinName)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	moveEnabled := false
	moveDragging := false
	resizeDragging := false
	activeCorner := noCorner

	var dragStartRootX, dragStartRootY int
	var resizeStartX, resizeStartY, resizeStartW, resizeStartH int

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
			if err := win.SetClickThrough(!enabled); err != nil {
				fmt.Fprintln(os.Stderr, "error: click-through:", err)
			}
		case name := <-tr.SkinCh:
			next, err := loadSkin(name)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: character %q: %v\n", name, err)
				break
			}
			rnd = next
			currentSkin = name
			nativeW, nativeH = rnd.Size()
			aspectRatio = float64(nativeW) / float64(nativeH)
			// Keep the current width; height follows the new aspect.
			x, y := win.Pos()
			w, _ := win.Size()
			win.MoveResize(x, y, w, int(math.Round(float64(w)/aspectRatio)))
		case <-ticker.C:
			for {
				e, ok := win.Poll()
				if !ok {
					break
				}
				switch e.Kind {
				case window.Closed:
					return

				case window.Press:
					if !moveEnabled || e.Button != 1 {
						break
					}
					ww, wh := win.Size()
					dragStartRootX, dragStartRootY = e.RootX, e.RootY
					if cn := detectCorner(e.X, e.Y, ww, wh); cn != noCorner {
						resizeDragging = true
						activeCorner = cn
						resizeStartX, resizeStartY = win.Pos()
						resizeStartW, resizeStartH = ww, wh
					} else {
						moveDragging = true
					}

				case window.Release:
					if e.Button == 1 {
						moveDragging = false
						resizeDragging = false
						activeCorner = noCorner
					}

				case window.Motion:
					if moveDragging {
						x, y := win.Pos()
						win.Move(x+e.RootX-dragStartRootX, y+e.RootY-dragStartRootY)
						dragStartRootX, dragStartRootY = e.RootX, e.RootY
					} else if resizeDragging {
						applyResize(aspectRatio, win, activeCorner,
							e.RootX-dragStartRootX,
							resizeStartX, resizeStartY,
							resizeStartW, resizeStartH)
					}
				}
			}

			// Render at current window size, with border when move/resize is on.
			w, h := win.Size()
			border := moveEnabled
			win.PutImage(rnd.Draw(c.State(), w, h, border))
		}
	}
}

// applyResize computes and applies a ratio-locked resize for the given corner.
// dx is the horizontal drag delta from the resize start.
func applyResize(aspectRatio float64, win window.Window, c corner, dx int,
	startX, startY, startW, startH int,
) {
	var newW int
	switch c {
	case cornerBR, cornerTR:
		newW = startW + dx
	case cornerBL, cornerTL:
		newW = startW - dx
	}
	if newW < minWidth {
		newW = minWidth
	}
	newH := int(math.Round(float64(newW) / aspectRatio))

	newX, newY := startX, startY
	switch c {
	case cornerBL, cornerTL:
		// Right edge stays fixed.
		newX = startX + startW - newW
	}
	switch c {
	case cornerTR, cornerTL:
		// Bottom edge stays fixed.
		newY = startY + startH - newH
	}

	win.MoveResize(newX, newY, newW, newH)
}
