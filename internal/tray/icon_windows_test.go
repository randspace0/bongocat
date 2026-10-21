package tray

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestTrayIconWrapsPNGInICO(t *testing.T) {
	var buf bytes.Buffer
	png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 200, 100)))
	ico := trayIcon(buf.Bytes())
	if !bytes.HasPrefix(ico, []byte{0, 0, 1, 0, 1, 0}) {
		t.Fatalf("missing ICO header: % x", ico[:6])
	}
	if _, err := png.Decode(bytes.NewReader(ico[22:])); err != nil {
		t.Fatalf("embedded PNG: %v", err)
	}
}
