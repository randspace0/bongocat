package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/andraantariksa/bongocat-x11/internal/cat"
	"github.com/andraantariksa/bongocat-x11/internal/input"
	"github.com/andraantariksa/bongocat-x11/internal/renderer"
	"github.com/andraantariksa/bongocat-x11/internal/window"
	"github.com/veandco/go-sdl2/sdl"
)

func assetDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "assets"
	}
	return filepath.Join(filepath.Dir(file), "assets")
}

func main() {
	win, err := window.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer win.Destroy()

	rnd, err := renderer.New(win.Renderer, assetDir())
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

	for {
		mon.Poll()

		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			switch event.(type) {
			case *sdl.QuitEvent:
				return
			case *sdl.KeyboardEvent:
				ke := event.(*sdl.KeyboardEvent)
				if ke.Keysym.Sym == sdl.K_ESCAPE {
					return
				}
			}
		}

		switch c.State() {
		case cat.LeftDown:
			rnd.DrawLeftDown()
		case cat.RightDown:
			rnd.DrawRightDown()
		case cat.BothDown:
			rnd.DrawBothDown()
		default:
			rnd.DrawIdle()
		}

		sdl.Delay(16)
	}
}
