package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

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

	for {
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

		rnd.DrawIdle()
		sdl.Delay(16)
	}
}
