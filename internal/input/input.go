package input

/*
#cgo LDFLAGS: -lX11 -lXtst

#include <X11/Xlib.h>
#include <X11/extensions/record.h>
#include <stdlib.h>
#include <string.h>

// xEvent wire format (first 32 bytes matter for key/button events)
typedef struct {
    unsigned char  type;
    unsigned char  detail;   // keycode or button number
    unsigned short seqno;
    unsigned int   time;
    unsigned int   root, event, child;
    short          rootX, rootY, eventX, eventY;
    unsigned short state;
    unsigned char  sameScreen;
    unsigned char  pad;
} RawEvent;

// Callback is called by XRecord from a separate thread.
// We use a global Go-registered C callback trampoline.
extern void goInputCallback(XPointer priv, XRecordInterceptData *data);

static XRecordContext start_record(Display *ctrl, Display *data_disp) {
    XRecordClientSpec clients = XRecordAllClients;
    XRecordRange *range = XRecordAllocRange();
    if (!range) return 0;

    memset(range, 0, sizeof(*range));
    range->device_events.first = KeyPress;
    range->device_events.last  = ButtonRelease;

    XRecordContext ctx = XRecordCreateContext(
        ctrl, 0, &clients, 1, &range, 1
    );
    XFree(range);
    if (!ctx) return 0;

    XSync(ctrl, False);

    if (!XRecordEnableContextAsync(data_disp, ctx, goInputCallback, NULL)) {
        XRecordFreeContext(ctrl, ctx);
        return 0;
    }
    return ctx;
}

static void process_pending(Display *data_disp) {
    XRecordProcessReplies(data_disp);
}

static void stop_record(Display *ctrl, Display *data_disp, XRecordContext ctx) {
    XRecordDisableContext(ctrl, ctx);
    XRecordFreeContext(ctrl, ctx);
    XSync(ctrl, False);
    XCloseDisplay(data_disp);
    XCloseDisplay(ctrl);
}
*/
import "C"

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

// EventType mirrors X11 event types we care about.
type EventType uint8

const (
	KeyPress     EventType = 2
	KeyRelease   EventType = 3
	ButtonPress  EventType = 4
	ButtonRelease EventType = 5
)

// Event is delivered to the handler callback.
type Event struct {
	Type   EventType
	Detail uint8 // keycode or button number
}

var globalHandler atomic.Value // stores func(Event)

//export goInputCallback
func goInputCallback(_ C.XPointer, data *C.XRecordInterceptData) {
	if data.category != C.XRecordFromServer {
		C.XRecordFreeData(data)
		return
	}
	if data.data_len < 8 {
		C.XRecordFreeData(data)
		return
	}

	raw := (*C.RawEvent)(unsafe.Pointer(data.data))
	evType := EventType(raw._type)

	switch evType {
	case KeyPress, KeyRelease, ButtonPress, ButtonRelease:
		if fn, ok := globalHandler.Load().(func(Event)); ok && fn != nil {
			fn(Event{Type: evType, Detail: uint8(raw.detail)})
		}
	}

	C.XRecordFreeData(data)
}

// Monitor listens to all X11 input events and delivers them to handler.
// Run() blocks until Stop() is called.
type Monitor struct {
	ctrl    *C.Display
	dataDpy *C.Display
	ctx     C.XRecordContext
}

func New(handler func(Event)) (*Monitor, error) {
	globalHandler.Store(handler)

	ctrl := C.XOpenDisplay(nil)
	if ctrl == nil {
		return nil, fmt.Errorf("XOpenDisplay (ctrl) failed")
	}
	dataDpy := C.XOpenDisplay(nil)
	if dataDpy == nil {
		C.XCloseDisplay(ctrl)
		return nil, fmt.Errorf("XOpenDisplay (data) failed")
	}

	ctx := C.start_record(ctrl, dataDpy)
	if ctx == 0 {
		C.XCloseDisplay(dataDpy)
		C.XCloseDisplay(ctrl)
		return nil, fmt.Errorf("XRecordCreateContext/EnableContext failed")
	}

	return &Monitor{ctrl: ctrl, dataDpy: dataDpy, ctx: ctx}, nil
}

// Poll processes pending XRecord replies. Call this in your event loop.
func (m *Monitor) Poll() {
	C.process_pending(m.dataDpy)
}

// Stop disables XRecord and closes both X11 connections.
func (m *Monitor) Stop() {
	C.stop_record(m.ctrl, m.dataDpy, m.ctx)
}

// IsLeft returns true for keycodes on the left side of the keyboard.
// Matches the same layout split as the original BongoCat-mac.
func IsLeft(keycode uint8) bool {
	switch keycode {
	// Row 0: Escape, 1-5, ~
	case 9, 10, 11, 12, 13, 14, 49:
		return true
	// Row 1: Tab, Q-T
	case 23, 24, 25, 26, 27, 28:
		return true
	// Row 2: CapsLock, A-G
	case 66, 38, 39, 40, 41, 42, 43:
		return true
	// Row 3: left Shift, Z-B
	case 50, 52, 53, 54, 55, 56:
		return true
	// Row 4: left Ctrl, left Super, left Alt, Space
	case 37, 133, 64, 65:
		return true
	}
	return false
}

// IsMouseLeft returns true for button 1 (left click).
func IsMouseLeft(button uint8) bool { return button == 1 }

// IsMouseRight returns true for button 3 (right click).
func IsMouseRight(button uint8) bool { return button == 3 }

// SizeofXPointer silences cgo unused-import warning.
var _ = unsafe.Sizeof(C.XPointer(nil))
