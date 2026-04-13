package xshape

/*
#cgo LDFLAGS: -lX11 -lXext

#include <X11/Xlib.h>
#include <X11/extensions/shape.h>

void make_click_through(Display *display, Window window) {
    Region empty = XCreateRegion();
    XShapeCombineRegion(display, window, ShapeInput, 0, 0, empty, ShapeSet);
    XDestroyRegion(empty);
    XFlush(display);
}
*/
import "C"
import (
	"fmt"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
)

// MakeClickThrough makes the SDL2 window pass all mouse events through
// to whatever window is underneath, using the X11 Shape extension.
func MakeClickThrough(win *sdl.Window) error {
	wmInfo, err := win.GetWMInfo()
	if err != nil {
		return fmt.Errorf("GetWMInfo: %w", err)
	}

	x11 := wmInfo.GetX11Info()
	display := (*C.Display)(unsafe.Pointer(x11.Display))
	window := C.Window(x11.Window)

	C.make_click_through(display, window)
	return nil
}
