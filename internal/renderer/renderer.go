package renderer

import (
	"fmt"

	"github.com/veandco/go-sdl2/img"
	"github.com/veandco/go-sdl2/sdl"
)

type Sprites struct {
	Base      *sdl.Texture
	LeftUp    *sdl.Texture
	LeftDown  *sdl.Texture
	RightUp   *sdl.Texture
	RightDown *sdl.Texture
}

type Renderer struct {
	r       *sdl.Renderer
	sprites Sprites
}

func New(r *sdl.Renderer, assetDir string) (*Renderer, error) {
	if err := img.Init(img.INIT_PNG); err != nil {
		return nil, fmt.Errorf("img init: %w", err)
	}

	load := func(name string) (*sdl.Texture, error) {
		path := assetDir + "/" + name
		tex, err := img.LoadTexture(r, path)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", name, err)
		}
		return tex, nil
	}

	base, err := load("base.png")
	if err != nil {
		return nil, err
	}
	leftUp, err := load("left-up.png")
	if err != nil {
		return nil, err
	}
	leftDown, err := load("left-down.png")
	if err != nil {
		return nil, err
	}
	rightUp, err := load("right-up.png")
	if err != nil {
		return nil, err
	}
	rightDown, err := load("right-down.png")
	if err != nil {
		return nil, err
	}

	return &Renderer{
		r: r,
		sprites: Sprites{
			Base:      base,
			LeftUp:    leftUp,
			LeftDown:  leftDown,
			RightUp:   rightUp,
			RightDown: rightDown,
		},
	}, nil
}

func (rnd *Renderer) Destroy() {
	rnd.sprites.Base.Destroy()
	rnd.sprites.LeftUp.Destroy()
	rnd.sprites.LeftDown.Destroy()
	rnd.sprites.RightUp.Destroy()
	rnd.sprites.RightDown.Destroy()
	img.Quit()
}

// DrawIdle renders the cat in idle pose: base body + both paws raised.
func (rnd *Renderer) DrawIdle() {
	rnd.r.SetDrawColor(0, 0, 0, 0)
	rnd.r.Clear()
	rnd.r.Copy(rnd.sprites.Base, nil, nil)
	rnd.r.Copy(rnd.sprites.LeftUp, nil, nil)
	rnd.r.Copy(rnd.sprites.RightUp, nil, nil)
	rnd.r.Present()
}

// DrawLeftDown renders the cat with the left paw striking down.
func (rnd *Renderer) DrawLeftDown() {
	rnd.r.SetDrawColor(0, 0, 0, 0)
	rnd.r.Clear()
	rnd.r.Copy(rnd.sprites.Base, nil, nil)
	rnd.r.Copy(rnd.sprites.LeftDown, nil, nil)
	rnd.r.Copy(rnd.sprites.RightUp, nil, nil)
	rnd.r.Present()
}

// DrawRightDown renders the cat with the right paw striking down.
func (rnd *Renderer) DrawRightDown() {
	rnd.r.SetDrawColor(0, 0, 0, 0)
	rnd.r.Clear()
	rnd.r.Copy(rnd.sprites.Base, nil, nil)
	rnd.r.Copy(rnd.sprites.LeftUp, nil, nil)
	rnd.r.Copy(rnd.sprites.RightDown, nil, nil)
	rnd.r.Present()
}

// DrawBothDown renders the cat with both paws striking down.
func (rnd *Renderer) DrawBothDown() {
	rnd.r.SetDrawColor(0, 0, 0, 0)
	rnd.r.Clear()
	rnd.r.Copy(rnd.sprites.Base, nil, nil)
	rnd.r.Copy(rnd.sprites.LeftDown, nil, nil)
	rnd.r.Copy(rnd.sprites.RightDown, nil, nil)
	rnd.r.Present()
}
