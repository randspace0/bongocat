package main

import (
	"fmt"
	"os"

	"github.com/andraantariksa/bongocat-x11/internal/window"
	"github.com/veandco/go-sdl2/sdl"
)

func main() {
	win, err := window.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer win.Destroy()

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

		win.Renderer.SetDrawColor(0, 0, 0, 0)
		win.Renderer.Clear()
		win.Renderer.Present()

		sdl.Delay(16)
	}
}
