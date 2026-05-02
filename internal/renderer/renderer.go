package renderer

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
)

const (
	borderThick = 2
	handleSize  = 12
)

type Renderer struct {
	base, leftUp, leftDown, rightUp, rightDown *image.RGBA
	nativeW, nativeH                           int
	nativeFrame                                *image.RGBA
	outFrame                                   *image.RGBA
	bgra                                       []byte
	outW, outH                                 int
}

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
		return toRGBA(img), nil
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

	bounds := base.Bounds()
	nativeW, nativeH := bounds.Dx(), bounds.Dy()
	nativeFrame := image.NewRGBA(bounds)

	return &Renderer{
		base:        base,
		leftUp:      leftUp,
		leftDown:    leftDown,
		rightUp:     rightUp,
		rightDown:   rightDown,
		nativeW:     nativeW,
		nativeH:     nativeH,
		nativeFrame: nativeFrame,
	}, nil
}

func (r *Renderer) Destroy() {}

func (r *Renderer) DrawIdle(w, h int, border bool) []byte {
	return r.compose(r.leftUp, r.rightUp, w, h, border)
}

func (r *Renderer) DrawLeftDown(w, h int, border bool) []byte {
	return r.compose(r.leftDown, r.rightUp, w, h, border)
}

func (r *Renderer) DrawRightDown(w, h int, border bool) []byte {
	return r.compose(r.leftUp, r.rightDown, w, h, border)
}

func (r *Renderer) DrawBothDown(w, h int, border bool) []byte {
	return r.compose(r.leftDown, r.rightDown, w, h, border)
}

func (r *Renderer) compose(left, right *image.RGBA, w, h int, border bool) []byte {
	// Compose sprites at native resolution.
	nb := r.nativeFrame.Bounds()
	pt := image.Point{}
	draw.Draw(r.nativeFrame, nb, image.Transparent, pt, draw.Src)
	draw.Draw(r.nativeFrame, nb, r.base, pt, draw.Over)
	draw.Draw(r.nativeFrame, nb, left, pt, draw.Over)
	draw.Draw(r.nativeFrame, nb, right, pt, draw.Over)

	// Resize output buffers if target size changed.
	if r.outW != w || r.outH != h {
		r.outFrame = image.NewRGBA(image.Rect(0, 0, w, h))
		r.bgra = make([]byte, w*h*4)
		r.outW, r.outH = w, h
	}

	// Scale or copy native frame to output.
	if w == r.nativeW && h == r.nativeH {
		copy(r.outFrame.Pix, r.nativeFrame.Pix)
	} else {
		scaleNN(r.nativeFrame, r.outFrame)
	}

	if border {
		drawOverlay(r.outFrame)
	}

	return rgbaToBGRA(r.outFrame, r.bgra)
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
