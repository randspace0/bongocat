package input

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Linux input-event-codes.h values.
const (
	evKey    = 1
	btnLeft  = 0x110
	btnRight = 0x111
	btnMax   = 0x120 // codes in [btnLeft, btnMax) are mouse buttons
)

// x11KeycodeOffset converts an evdev key code to the X11 keycode that
// IsLeft expects.
const x11KeycodeOffset = 8

// EvdevMonitor reads /dev/input/event* directly, the only global input source
// under Wayland. The user must be in the `input` group.
type EvdevMonitor struct {
	files []*os.File
	once  sync.Once
}

// NewEvdev starts one reader per keyboard/mouse device and calls handler for
// each key or button event, using the same Event encoding as the X11 Monitor.
// Devices plugged in later are not picked up.
func NewEvdev(handler func(Event)) (*EvdevMonitor, error) {
	paths, _ := filepath.Glob("/sys/class/input/event*")
	m := &EvdevMonitor{}
	var denied bool
	for _, p := range paths {
		if !hasKeys(p) {
			continue
		}
		f, err := os.Open("/dev/input/" + filepath.Base(p))
		if err != nil {
			denied = denied || errors.Is(err, os.ErrPermission)
			continue
		}
		m.files = append(m.files, f)
		go readEvdev(f, handler)
	}
	if len(m.files) == 0 {
		if denied {
			return nil, fmt.Errorf("input: cannot read /dev/input/event*: add yourself to the input group " +
				"(sudo usermod -aG input $USER) and log in again")
		}
		return nil, fmt.Errorf("input: no keyboard or mouse devices found")
	}
	return m, nil
}

// hasKeys reports whether the sysfs input device has any EV_KEY capability.
func hasKeys(sysDev string) bool {
	b, err := os.ReadFile(sysDev + "/device/capabilities/key")
	if err != nil {
		return false
	}
	for _, w := range strings.Fields(string(b)) {
		if v, err := strconv.ParseUint(w, 16, 64); err == nil && v != 0 {
			return true
		}
	}
	return false
}

// readEvdev decodes struct input_event: timeval (16 bytes on 64-bit), then
// type u16, code u16, value s32.
func readEvdev(f *os.File, handler func(Event)) {
	var buf [24]byte
	for {
		if _, err := io.ReadFull(f, buf[:]); err != nil {
			return
		}
		if binary.LittleEndian.Uint16(buf[16:]) != evKey {
			continue
		}
		code := binary.LittleEndian.Uint16(buf[18:])
		value := binary.LittleEndian.Uint32(buf[20:])
		if value > 1 { // ignore autorepeat
			continue
		}
		pressed := value == 1

		switch {
		case code >= btnLeft && code < btnMax:
			var btn uint8
			switch code {
			case btnLeft:
				btn = 1
			case btnRight:
				btn = 3
			default:
				continue
			}
			t := ButtonRelease
			if pressed {
				t = ButtonPress
			}
			handler(Event{Type: t, Detail: btn})
		case code+x11KeycodeOffset <= 255 && code < btnLeft:
			t := KeyRelease
			if pressed {
				t = KeyPress
			}
			handler(Event{Type: t, Detail: uint8(code + x11KeycodeOffset)})
		}
	}
}

// Stop closes all device files, ending the reader goroutines.
func (m *EvdevMonitor) Stop() {
	m.once.Do(func() {
		for _, f := range m.files {
			f.Close()
		}
	})
}
