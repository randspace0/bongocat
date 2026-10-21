package window

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procSetWindowPos        = user32.NewProc("SetWindowPos")
	procShowWindow          = user32.NewProc("ShowWindow")
	procGetWindowLongW      = user32.NewProc("GetWindowLongW")
	procSetWindowLongW      = user32.NewProc("SetWindowLongW")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procSetCapture          = user32.NewProc("SetCapture")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procClientToScreen      = user32.NewProc("ClientToScreen")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	procSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")
	procCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	procCreateDIBSection    = gdi32.NewProc("CreateDIBSection")
	procSelectObject        = gdi32.NewProc("SelectObject")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procDeleteDC            = gdi32.NewProc("DeleteDC")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
)

const (
	wmDestroy     = 0x0002
	wmClose       = 0x0010
	wmMouseMove   = 0x0200
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmApp         = 0x8000

	wsPopup         = 0x80000000
	wsExTopmost     = 0x00000008
	wsExTransparent = 0x00000020
	wsExToolWindow  = 0x00000080
	wsExLayered     = 0x00080000
	wsExNoActivate  = 0x08000000

	gwlExStyle       = ^uintptr(19) // -20
	swShowNoActivate = 4

	swpNoSize     = 0x0001
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	smCxScreen = 0
	smCyScreen = 1

	ulwAlpha   = 2
	acSrcAlpha = 1
)

type wndClassEx struct {
	size, style                     uint32
	wndProc                         uintptr
	clsExtra, wndExtra              int32
	instance, icon, cursor, bgBrush uintptr
	menuName, className             *uint16
	iconSm                          uintptr
}

type point struct{ x, y int32 }
type size struct{ cx, cy int32 }

type blendFunction struct{ op, flags, alpha, format byte }

type bitmapInfoHeader struct {
	size                         uint32
	width, height                int32
	planes, bitCount             uint16
	compression, sizeImage       uint32
	xPelsPerMeter, yPelsPerMeter int32
	clrUsed, clrImportant        uint32
}

type winWindow struct {
	hwnd   uintptr
	events chan Event

	mu         sync.Mutex
	x, y, w, h int

	// DIB section backing PutImage; only touched from the render goroutine.
	memDC, dib uintptr
	dibW, dibH int
	dibBits    unsafe.Pointer
}

// IsWayland is always false on Windows.
func IsWayland() bool { return false }

// New opens a window of the given size, centered on screen.
func New(width, height int) (Window, error) {
	// Without this, Windows bitmap-scales the window on high-DPI displays.
	procSetProcessDPIAware.Call()

	win := &winWindow{events: make(chan Event, 256), w: width, h: height}
	sw, _, _ := procGetSystemMetrics.Call(smCxScreen)
	sh, _, _ := procGetSystemMetrics.Call(smCyScreen)
	win.x, win.y = (int(sw)-width)/2, (int(sh)-height)/2

	ready := make(chan error, 1)
	go win.loop(ready)
	if err := <-ready; err != nil {
		return nil, fmt.Errorf("window: %w", err)
	}
	return win, nil
}

// loop owns the HWND: window messages are only delivered to the creating thread.
func (win *winWindow) loop(ready chan<- error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	inst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("BongoCatWindow")
	wc := wndClassEx{
		wndProc:   syscall.NewCallback(win.wndProc),
		instance:  inst,
		className: className,
	}
	wc.size = uint32(unsafe.Sizeof(wc))
	if a, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); a == 0 {
		ready <- fmt.Errorf("RegisterClassEx: %w", err)
		return
	}
	hwnd, _, err := procCreateWindowExW.Call(
		wsExLayered|wsExTopmost|wsExToolWindow|wsExNoActivate|wsExTransparent,
		uintptr(unsafe.Pointer(className)), 0, wsPopup,
		uintptr(win.x), uintptr(win.y), uintptr(win.w), uintptr(win.h),
		0, 0, inst, 0)
	if hwnd == 0 {
		ready <- fmt.Errorf("CreateWindowEx: %w", err)
		return
	}
	win.hwnd = hwnd
	procShowWindow.Call(hwnd, swShowNoActivate)
	ready <- nil

	var msg [48]byte // MSG, 64-bit layout
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg[0])), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg[0])))
	}
}

func (win *winWindow) wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmLButtonDown, wmLButtonUp, wmMouseMove:
		x, y := int32(int16(lparam)), int32(int16(lparam>>16))
		pt := point{x, y}
		procClientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
		e := Event{Kind: Motion, X: int(x), Y: int(y), RootX: int(pt.x), RootY: int(pt.y)}
		switch msg {
		case wmLButtonDown:
			// Capture keeps the drag tracking when the cursor outruns the window.
			procSetCapture.Call(hwnd)
			e.Kind, e.Button = Press, 1
		case wmLButtonUp:
			procReleaseCapture.Call()
			e.Kind, e.Button = Release, 1
		}
		select {
		case win.events <- e:
		default: // backlog; Poll drains every frame
		}
		return 0
	case wmClose:
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		select {
		case win.events <- Event{Kind: Closed}:
		default:
		}
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

func (win *winWindow) Size() (int, int) {
	win.mu.Lock()
	defer win.mu.Unlock()
	return win.w, win.h
}

func (win *winWindow) Pos() (int, int) {
	win.mu.Lock()
	defer win.mu.Unlock()
	return win.x, win.y
}

func (win *winWindow) Poll() (Event, bool) {
	select {
	case e := <-win.events:
		return e, true
	default:
		return Event{}, false
	}
}

func (win *winWindow) Move(x, y int) {
	win.mu.Lock()
	win.x, win.y = x, y
	win.mu.Unlock()
	procSetWindowPos.Call(win.hwnd, 0, uintptr(x), uintptr(y), 0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate)
}

func (win *winWindow) MoveResize(x, y, w, h int) {
	win.mu.Lock()
	win.x, win.y, win.w, win.h = x, y, w, h
	win.mu.Unlock()
	procSetWindowPos.Call(win.hwnd, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		swpNoZOrder|swpNoActivate)
}

func (win *winWindow) SetClickThrough(on bool) error {
	r, _, _ := procGetWindowLongW.Call(win.hwnd, gwlExStyle)
	style := uint32(r)
	if on {
		style |= wsExTransparent
	} else {
		style &^= wsExTransparent
	}
	procSetWindowLongW.Call(win.hwnd, gwlExStyle, uintptr(style))
	return nil
}

// PutImage draws a premultiplied BGRA frame sized to the current window.
func (win *winWindow) PutImage(bgra []byte) {
	w, h := win.Size()
	if len(bgra) < w*h*4 {
		return
	}
	if win.dib == 0 || win.dibW != w || win.dibH != h {
		if !win.resetDIB(w, h) {
			return
		}
	}
	copy(unsafe.Slice((*byte)(win.dibBits), w*h*4), bgra)

	x, y := win.Pos()
	dst, sz, src := point{int32(x), int32(y)}, size{int32(w), int32(h)}, point{}
	bf := blendFunction{alpha: 255, format: acSrcAlpha}
	procUpdateLayeredWindow.Call(win.hwnd, 0,
		uintptr(unsafe.Pointer(&dst)), uintptr(unsafe.Pointer(&sz)),
		win.memDC, uintptr(unsafe.Pointer(&src)), 0,
		uintptr(unsafe.Pointer(&bf)), ulwAlpha)
}

func (win *winWindow) resetDIB(w, h int) bool {
	win.freeDIB()
	screen, _, _ := procGetDC.Call(0)
	defer procReleaseDC.Call(0, screen)
	win.memDC, _, _ = procCreateCompatibleDC.Call(screen)
	// Negative height = top-down rows, matching the renderer's output.
	bi := bitmapInfoHeader{width: int32(w), height: -int32(h), planes: 1, bitCount: 32}
	bi.size = uint32(unsafe.Sizeof(bi))
	win.dib, _, _ = procCreateDIBSection.Call(win.memDC, uintptr(unsafe.Pointer(&bi)), 0,
		uintptr(unsafe.Pointer(&win.dibBits)), 0, 0)
	if win.dib == 0 {
		win.freeDIB()
		return false
	}
	procSelectObject.Call(win.memDC, win.dib)
	win.dibW, win.dibH = w, h
	return true
}

func (win *winWindow) freeDIB() {
	if win.dib != 0 {
		procDeleteObject.Call(win.dib)
	}
	if win.memDC != 0 {
		procDeleteDC.Call(win.memDC)
	}
	win.dib, win.memDC, win.dibW, win.dibH = 0, 0, 0, 0
}

func (win *winWindow) Destroy() {
	procPostMessageW.Call(win.hwnd, wmClose, 0, 0)
	win.freeDIB()
}
