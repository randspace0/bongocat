package input

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/record"
	"github.com/jezek/xgb/xproto"
)

// EventType mirrors X11 event types we care about.
type EventType uint8

const (
	KeyPress      EventType = 2
	KeyRelease    EventType = 3
	ButtonPress   EventType = 4
	ButtonRelease EventType = 5
)

// Event is delivered to the handler callback.
type Event struct {
	Type   EventType
	Detail uint8 // keycode or button number
}

// Monitor listens to global X11 input events via XRecord.
type Monitor struct {
	ctrl   *xgb.Conn
	data   net.Conn
	ctx    record.Context
	stopCh chan struct{}
}

// New starts XRecord monitoring and calls handler for each input event.
func New(handler func(Event)) (*Monitor, error) {
	ctrl, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("input: ctrl conn: %w", err)
	}

	if err := record.Init(ctrl); err != nil {
		ctrl.Close()
		return nil, fmt.Errorf("input: record init: %w", err)
	}

	ctxID, err := record.NewContextId(ctrl)
	if err != nil {
		ctrl.Close()
		return nil, fmt.Errorf("input: new context id: %w", err)
	}

	rng := record.Range{
		DeviceEvents: record.Range8{First: 2, Last: 5},
	}
	err = record.CreateContextChecked(ctrl, ctxID, 0, 1, 1,
		[]record.ClientSpec{record.CsAllClients},
		[]record.Range{rng}).Check()
	if err != nil {
		ctrl.Close()
		return nil, fmt.Errorf("input: create context: %w", err)
	}

	// Sync ctrl so server has processed CreateContext.
	xproto.GetInputFocus(ctrl).Reply()

	// Dial a second raw connection for XRecord streaming replies.
	// xgb's cookie mechanism only handles one reply per sequence number,
	// but XRecord sends many; we bypass it with a raw socket.
	data, err := dialX11Raw()
	if err != nil {
		record.FreeContext(ctrl, ctxID)
		ctrl.Close()
		return nil, fmt.Errorf("input: data conn: %w", err)
	}

	// Get the RECORD extension opcode from the ctrl connection.
	ctrl.ExtLock.RLock()
	opcode := ctrl.Extensions["RECORD"]
	ctrl.ExtLock.RUnlock()

	// Send EnableContext request on the raw data connection.
	// Byte layout (little-endian per LSB handshake):
	//   [0]   major opcode
	//   [1]   5 = EnableContext minor opcode
	//   [2-3] request length in 4-byte units = 2 (8 bytes total), LE → {2, 0}
	//   [4-7] context ID, LE
	req := [8]byte{opcode, 5, 2, 0}
	binary.LittleEndian.PutUint32(req[4:], uint32(ctxID))
	if _, err := data.Write(req[:]); err != nil {
		data.Close()
		record.FreeContext(ctrl, ctxID)
		ctrl.Close()
		return nil, fmt.Errorf("input: send EnableContext: %w", err)
	}

	m := &Monitor{
		ctrl:   ctrl,
		data:   data,
		ctx:    ctxID,
		stopCh: make(chan struct{}),
	}

	go m.readLoop(handler)
	return m, nil
}

func (m *Monitor) readLoop(handler func(Event)) {
	var header [32]byte
	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		if _, err := io.ReadFull(m.data, header[:]); err != nil {
			return
		}

		if header[0] != 1 {
			continue
		}

		category := header[1]
		extraLen := binary.LittleEndian.Uint32(header[4:8])

		if extraLen == 0 {
			continue
		}

		extra := make([]byte, extraLen*4)
		if _, err := io.ReadFull(m.data, extra); err != nil {
			return
		}

		if category != 0 || len(extra) < 2 {
			continue
		}

		evType := EventType(extra[0])
		detail := extra[1]
		switch evType {
		case KeyPress, KeyRelease, ButtonPress, ButtonRelease:
			handler(Event{Type: evType, Detail: detail})
		}
	}
}

// Stop disables XRecord and closes all connections.
func (m *Monitor) Stop() {
	close(m.stopCh)
	m.data.Close()
	record.DisableContext(m.ctrl, m.ctx)
	record.FreeContext(m.ctrl, m.ctx)
	m.ctrl.Close()
}

// dialX11Raw opens a raw TCP/unix connection to the X server and performs
// the initial handshake. The connection is ready to send X11 requests.
func dialX11Raw() (net.Conn, error) {
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
	}

	// Parse display number (e.g. ":0", ":0.0", "hostname:1").
	colonIdx := strings.LastIndex(display, ":")
	if colonIdx < 0 {
		return nil, fmt.Errorf("bad DISPLAY: %s", display)
	}
	dispStr := display[colonIdx+1:]
	if dot := strings.Index(dispStr, "."); dot >= 0 {
		dispStr = dispStr[:dot]
	}
	dispNum, _ := strconv.Atoi(dispStr)

	host := display[:colonIdx]

	var conn net.Conn
	var err error
	if host == "" || host == "unix" {
		conn, err = net.Dial("unix", fmt.Sprintf("/tmp/.X11-unix/X%d", dispNum))
	} else {
		conn, err = net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(6000+dispNum)))
	}
	if err != nil {
		return nil, err
	}

	authName, authData, err := readXauth(host, strconv.Itoa(dispNum))
	if err != nil {
		authName = ""
		authData = nil
	}

	// Build X11 connection-setup request.
	padLen := func(n int) int { return (4 - (n & 3)) & 3 }
	size := 12 + len(authName) + padLen(len(authName)) + len(authData) + padLen(len(authData))
	buf := make([]byte, size)
	buf[0] = 0x6c // LSB
	binary.LittleEndian.PutUint16(buf[2:], 11) // major version
	binary.LittleEndian.PutUint16(buf[6:], uint16(len(authName)))
	binary.LittleEndian.PutUint16(buf[8:], uint16(len(authData)))
	off := 12
	copy(buf[off:], authName)
	off += len(authName) + padLen(len(authName))
	copy(buf[off:], authData)

	if _, err := conn.Write(buf); err != nil {
		conn.Close()
		return nil, err
	}

	// Read setup reply header (8 bytes).
	var head [8]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		conn.Close()
		return nil, err
	}
	if head[0] != 1 {
		conn.Close()
		return nil, fmt.Errorf("X11 setup failed: code=%d", head[0])
	}
	// Read remaining setup data and discard.
	dataLen := binary.LittleEndian.Uint16(head[6:])
	rest := make([]byte, int(dataLen)*4)
	if _, err := io.ReadFull(conn, rest); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// readXauth reads MIT-MAGIC-COOKIE-1 from ~/.Xauthority for the given display.
// Adapted from github.com/jezek/xgb/auth.go (BSD license).
func readXauth(hostname, display string) (string, []byte, error) {
	if hostname == "" || hostname == "localhost" {
		h, err := os.Hostname()
		if err != nil {
			return "", nil, err
		}
		hostname = h
	}

	fname := os.Getenv("XAUTHORITY")
	if fname == "" {
		home := os.Getenv("HOME")
		if home == "" {
			return "", nil, fmt.Errorf("XAUTHORITY and HOME not set")
		}
		fname = home + "/.Xauthority"
	}

	f, err := os.Open(fname)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	readStr := func() (string, error) {
		var n uint16
		if err := binary.Read(f, binary.BigEndian, &n); err != nil {
			return "", err
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(f, b); err != nil {
			return "", err
		}
		return string(b), nil
	}
	readBytes := func() ([]byte, error) {
		var n uint16
		if err := binary.Read(f, binary.BigEndian, &n); err != nil {
			return nil, err
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(f, b); err != nil {
			return nil, err
		}
		return b, nil
	}

	const familyLocal = 256
	const familyWild = 65535

	for {
		var family uint16
		if err := binary.Read(f, binary.BigEndian, &family); err != nil {
			return "", nil, err
		}
		addr, err := readStr()
		if err != nil {
			return "", nil, err
		}
		disp, err := readStr()
		if err != nil {
			return "", nil, err
		}
		name, err := readStr()
		if err != nil {
			return "", nil, err
		}
		data, err := readBytes()
		if err != nil {
			return "", nil, err
		}

		addrMatch := family == familyWild || (family == familyLocal && addr == hostname)
		dispMatch := disp == "" || disp == display
		if addrMatch && dispMatch {
			return name, data, nil
		}
	}
}

// IsLeft returns true for keycodes on the left side of the keyboard.
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
