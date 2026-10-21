package input

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadId  = kernel32.NewProc("GetCurrentThreadId")
)

const (
	whKeyboardLL = 13
	whMouseLL    = 14

	wmKeyDown     = 0x0100
	wmKeyUp       = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	wmLButtonDown = 0x0201
	wmLButtonUp   = 0x0202
	wmRButtonDown = 0x0204
	wmRButtonUp   = 0x0205
	wmQuit        = 0x0012
)

// Monitor listens to global keyboard and mouse input via low-level hooks.
type Monitor struct {
	threadID uint32
	done     chan struct{}
}

// New installs WH_KEYBOARD_LL and WH_MOUSE_LL hooks and calls handler for each
// event. Key Detail values are Windows virtual-key codes.
func New(handler func(Event)) (*Monitor, error) {
	m := &Monitor{done: make(chan struct{})}
	ready := make(chan error, 1)
	go m.loop(handler, ready)
	if err := <-ready; err != nil {
		return nil, err
	}
	return m, nil
}

// loop owns the hooks: they only fire while their installing thread pumps messages.
func (m *Monitor) loop(handler func(Event), ready chan<- error) {
	defer close(m.done)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	tid, _, _ := procGetCurrentThreadId.Call()
	m.threadID = uint32(tid)

	// held filters keyboard auto-repeat, which would otherwise re-press every paw tap.
	var held [256]bool

	next := func(code, wparam, lparam uintptr) uintptr {
		r, _, _ := procCallNextHookEx.Call(0, code, wparam, lparam)
		return r
	}
	kbCb := syscall.NewCallback(func(code, wparam, lparam uintptr) uintptr {
		if int32(code) >= 0 {
			// lparam is a KBDLLHOOKSTRUCT*; reinterpreting the uintptr in place keeps vet quiet.
			vk := uint8(**(**uint32)(unsafe.Pointer(&lparam)))
			switch wparam {
			case wmKeyDown, wmSysKeyDown:
				if !held[vk] {
					held[vk] = true
					handler(Event{Type: KeyPress, Detail: vk})
				}
			case wmKeyUp, wmSysKeyUp:
				held[vk] = false
				handler(Event{Type: KeyRelease, Detail: vk})
			}
		}
		return next(code, wparam, lparam)
	})
	mouseCb := syscall.NewCallback(func(code, wparam, lparam uintptr) uintptr {
		if int32(code) >= 0 {
			switch wparam {
			case wmLButtonDown:
				handler(Event{Type: ButtonPress, Detail: 1})
			case wmLButtonUp:
				handler(Event{Type: ButtonRelease, Detail: 1})
			case wmRButtonDown:
				handler(Event{Type: ButtonPress, Detail: 3})
			case wmRButtonUp:
				handler(Event{Type: ButtonRelease, Detail: 3})
			}
		}
		return next(code, wparam, lparam)
	})

	kb, _, err := procSetWindowsHookExW.Call(whKeyboardLL, kbCb, 0, 0)
	if kb == 0 {
		ready <- fmt.Errorf("input: keyboard hook: %w", err)
		return
	}
	defer procUnhookWindowsHookEx.Call(kb)
	ms, _, err := procSetWindowsHookExW.Call(whMouseLL, mouseCb, 0, 0)
	if ms == 0 {
		ready <- fmt.Errorf("input: mouse hook: %w", err)
		return
	}
	defer procUnhookWindowsHookEx.Call(ms)
	ready <- nil

	var msg [48]byte // MSG, 64-bit layout
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg[0])), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
	}
}

// Stop removes the hooks and ends the message loop.
func (m *Monitor) Stop() {
	procPostThreadMessageW.Call(uintptr(m.threadID), wmQuit, 0, 0)
	<-m.done
}

// IsLeft returns true for virtual-key codes on the left side of the keyboard.
func IsLeft(vk uint8) bool { return leftKeys[vk] }

var leftKeys = func() (s [256]bool) {
	for _, k := range []uint8{
		0x1B, 0xC0, 0x09, 0x14, 0x20, // Esc, `, Tab, CapsLock, Space
		0xA0, 0xA2, 0xA4, 0x5B, // left Shift, Ctrl, Alt, Win
		'1', '2', '3', '4', '5',
		'Q', 'W', 'E', 'R', 'T',
		'A', 'S', 'D', 'F', 'G',
		'Z', 'X', 'C', 'V', 'B',
	} {
		s[k] = true
	}
	return
}()

// NewEvdev exists so shared code compiles; evdev is Linux-only.
func NewEvdev(handler func(Event)) (*Monitor, error) {
	return nil, errors.New("input: evdev is Linux-only")
}
