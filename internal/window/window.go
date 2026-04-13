package window

import (
	"fmt"

	"github.com/veandco/go-sdl2/sdl"
)

const (
	Width  = 397
	Height = 201

	// SDL_WINDOW_TRANSPARENT (0x00100000) is not yet wrapped in go-sdl2 v0.4.40
	// but is present in SDL2 >= 2.0.16. Use the raw flag value.
	windowTransparent = uint32(0x00100000)
)

type Window struct {
	SDLWindow *sdl.Window
	Renderer  *sdl.Renderer
}

func New() (*Window, error) {
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		return nil, fmt.Errorf("sdl init: %w", err)
	}

	sdl.SetHint(sdl.HINT_VIDEO_X11_NET_WM_BYPASS_COMPOSITOR, "0")

	win, err := sdl.CreateWindow(
		"BongoCat",
		sdl.WINDOWPOS_UNDEFINED, sdl.WINDOWPOS_UNDEFINED,
		Width, Height,
		sdl.WINDOW_SHOWN|sdl.WINDOW_BORDERLESS|sdl.WINDOW_ALWAYS_ON_TOP|windowTransparent,
	)
	if err != nil {
		return nil, fmt.Errorf("create window: %w", err)
	}

	renderer, err := sdl.CreateRenderer(win, -1, sdl.RENDERER_ACCELERATED|sdl.RENDERER_PRESENTVSYNC)
	if err != nil {
		win.Destroy()
		return nil, fmt.Errorf("create renderer: %w", err)
	}

	renderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)

	return &Window{SDLWindow: win, Renderer: renderer}, nil
}

func (w *Window) Destroy() {
	w.Renderer.Destroy()
	w.SDLWindow.Destroy()
	sdl.Quit()
}
