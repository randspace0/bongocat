package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"image/png"
)

const iconSize = 64

// trayIcon wraps the PNG in an ICO container, which is what the Windows tray
// loader expects. The image is fitted into a transparent square so a wide
// sprite is not distorted. On any decode error the input is returned as-is.
func trayIcon(src []byte) []byte {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return src
	}
	b := img.Bounds()
	scale := float64(iconSize) / float64(max(b.Dx(), b.Dy()))
	w, h := max(1, int(float64(b.Dx())*scale)), max(1, int(float64(b.Dy())*scale))
	dst := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	offX, offY := (iconSize-w)/2, (iconSize-h)/2
	in := toNRGBA(img)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(offX+x, offY+y, in.At(b.Min.X+int(float64(x)/scale), b.Min.Y+int(float64(y)/scale)))
		}
	}
	var pngBuf bytes.Buffer
	if png.Encode(&pngBuf, dst) != nil {
		return src
	}

	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, [3]uint16{0, 1, 1}) // ICONDIR: reserved, type=icon, count
	ico.Write([]byte{iconSize, iconSize, 0, 0})                 // width, height, palette, reserved
	binary.Write(&ico, binary.LittleEndian, [2]uint16{1, 32})   // planes, bpp
	binary.Write(&ico, binary.LittleEndian, [2]uint32{uint32(pngBuf.Len()), 22})
	ico.Write(pngBuf.Bytes())
	return ico.Bytes()
}

func toNRGBA(img image.Image) *image.NRGBA {
	out := image.NewNRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	return out
}
