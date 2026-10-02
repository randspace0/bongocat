package renderer

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"

	"github.com/randspace0/bongocat/internal/cat"
)

const (
	borderThick = 2
	handleSize  = 12
)

// NativeWidth is the width every skin is normalised to on load.
const NativeWidth = 397

// Renderer draws one skin's frames, indexed by cat.State.
type Renderer struct {
	frames           [4]*image.RGBA
	nativeW, nativeH int
	outFrame         *image.RGBA
	bgra             []byte
	outW, outH       int
}

// Skins lists the skin directory names under fsys.
func Skins(fsys fs.FS) ([]string, error) {
	ents, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// New loads the skin directory fsys. A skin is either layered (base.png plus
// left/right up/down paws) or flat (idle.png, left.png, right.png).
func New(fsys fs.FS) (*Renderer, error) {
	load := func(name string) (*image.RGBA, error) {
		f, err := fsys.Open(name)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", name, err)
		}
		defer f.Close()
		img, err := png.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		return boxScale(toRGBA(img), NativeWidth), nil
	}
	loadAll := func(names ...string) ([]*image.RGBA, error) {
		imgs := make([]*image.RGBA, len(names))
		for i, n := range names {
			img, err := load(n)
			if err != nil {
				return nil, err
			}
			imgs[i] = img
		}
		return imgs, nil
	}

	r := &Renderer{}
	if _, err := fs.Stat(fsys, "base.png"); err == nil {
		l, err := loadAll("base.png", "left-up.png", "left-down.png", "right-up.png", "right-down.png")
		if err != nil {
			return nil, err
		}
		base, lu, ld, ru, rd := l[0], l[1], l[2], l[3], l[4]
		r.frames = [4]*image.RGBA{
			cat.Idle:      layer(base, lu, ru),
			cat.LeftDown:  layer(base, ld, ru),
			cat.RightDown: layer(base, lu, rd),
			cat.BothDown:  layer(base, ld, rd),
		}
	} else {
		l, err := loadAll("idle.png", "left.png", "right.png")
		if err != nil {
			return nil, err
		}
		// Flat skins have no both-paws frame; reuse the left one.
		r.frames = [4]*image.RGBA{cat.Idle: l[0], cat.LeftDown: l[1], cat.RightDown: l[2], cat.BothDown: l[1]}
	}
	b := r.frames[cat.Idle].Bounds()
	r.nativeW, r.nativeH = b.Dx(), b.Dy()
	return r, nil
}

// Size returns the skin's native pixel size.
func (r *Renderer) Size() (w, h int) { return r.nativeW, r.nativeH }

func (r *Renderer) Destroy() {}

// Draw renders state at w×h, with the move/resize border when border is set.
func (r *Renderer) Draw(state cat.State, w, h int, border bool) []byte {
	if r.outW != w || r.outH != h {
		r.outFrame = image.NewRGBA(image.Rect(0, 0, w, h))
		r.bgra = make([]byte, w*h*4)
		r.outW, r.outH = w, h
	}

	src := r.frames[state]
	if w == r.nativeW && h == r.nativeH {
		copy(r.outFrame.Pix, src.Pix)
	} else {
		scaleNN(src, r.outFrame)
	}

	if border {
		drawOverlay(r.outFrame)
	}

	return rgbaToBGRA(r.outFrame, r.bgra)
}

// layer composites the base and both paw sprites into one frame.
func layer(base, left, right *image.RGBA) *image.RGBA {
	out := image.NewRGBA(base.Bounds())
	for _, img := range []*image.RGBA{base, left, right} {
		draw.Draw(out, out.Bounds(), img, image.Point{}, draw.Over)
	}
	return out
}

// boxScale downscales src to width w (area average on premultiplied pixels).
// Images already at or below w are returned unchanged.
func boxScale(src *image.RGBA, w int) *image.RGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw <= w {
		return src
	}
	h := sh * w / sw
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0, y1 := y*sh/h, max((y+1)*sh/h, y*sh/h+1)
		for x := 0; x < w; x++ {
			x0, x1 := x*sw/w, max((x+1)*sw/w, x*sw/w+1)
			var sum [4]uint32
			n := uint32((y1 - y0) * (x1 - x0))
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					i := src.PixOffset(sx, sy)
					for c := 0; c < 4; c++ {
						sum[c] += uint32(src.Pix[i+c])
					}
				}
			}
			di := dst.PixOffset(x, y)
			for c := 0; c < 4; c++ {
				dst.Pix[di+c] = uint8(sum[c] / n)
			}
		}
	}
	return dst
}

// scaleNN scales src into dst using nearest-neighbour interpolation.
func scaleNN(src, dst *image.RGBA) {
	sw := src.Bounds().Dx()
	sh := src.Bounds().Dy()
	dw := dst.Bounds().Dx()
	dh := dst.Bounds().Dy()
	for dy := 0; dy < dh; dy++ {
		sy := dy * sh / dh
		for dx := 0; dx < dw; dx++ {
			sx := dx * sw / dw
			di := dst.PixOffset(dx, dy)
			si := src.PixOffset(sx, sy)
			dst.Pix[di+0] = src.Pix[si+0]
			dst.Pix[di+1] = src.Pix[si+1]
			dst.Pix[di+2] = src.Pix[si+2]
			dst.Pix[di+3] = src.Pix[si+3]
		}
	}
}

// drawOverlay renders the move/resize mode border and corner handles.
func drawOverlay(img *image.RGBA) {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	setPixel := func(x, y int, c color.RGBA) {
		if x < 0 || y < 0 || x >= w || y >= h {
			return
		}
		i := img.PixOffset(x, y)
		img.Pix[i+0] = c.R
		img.Pix[i+1] = c.G
		img.Pix[i+2] = c.B
		img.Pix[i+3] = c.A
	}

	border := color.RGBA{255, 255, 255, 220}
	handle := color.RGBA{60, 140, 255, 240}

	// Thin white border along all four edges.
	for t := 0; t < borderThick; t++ {
		for x := 0; x < w; x++ {
			setPixel(x, t, border)
			setPixel(x, h-1-t, border)
		}
		for y := 0; y < h; y++ {
			setPixel(t, y, border)
			setPixel(w-1-t, y, border)
		}
	}

	// Blue corner squares marking the resize handles.
	for i := 0; i < handleSize; i++ {
		for j := 0; j < handleSize; j++ {
			setPixel(i, j, handle)
			setPixel(w-1-i, j, handle)
			setPixel(i, h-1-j, handle)
			setPixel(w-1-i, h-1-j, handle)
		}
	}
}

// rgbaToBGRA converts RGBA pixel data to X11 ZPixmap BGRA format.
func rgbaToBGRA(src *image.RGBA, dst []byte) []byte {
	pix := src.Pix
	for i := 0; i < len(pix); i += 4 {
		dst[i+0] = pix[i+2]
		dst[i+1] = pix[i+1]
		dst[i+2] = pix[i+0]
		dst[i+3] = pix[i+3]
	}
	return dst
}

func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	return rgba
}
