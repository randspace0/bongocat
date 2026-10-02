package window

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"syscall"

	"github.com/rajveermalviya/go-wayland/wayland/client"
	"github.com/randspace0/bongocat/internal/layershell"
)

const btnLeft = 0x110 // linux/input-event-codes.h

type shmBuf struct {
	buf  *client.Buffer
	mem  []byte
	busy bool
}

type wlWindow struct {
	// mu guards every use of the client Context (its object map is not
	// thread-safe) and the fields below. Only the blocking socket read in
	// loop runs without it.
	mu      sync.Mutex
	display *client.Display
	ctx     *client.Context

	compositor *client.Compositor
	shm        *client.Shm
	seat       *client.Seat
	output     *client.Output
	shell      *layershell.LayerShell
	surface    *client.Surface
	layer      *layershell.LayerSurface

	outW, outH, outScale int

	x, y, w, h int // requested geometry; x,y are layer margins from top-left
	cfgW, cfgH int // last size acked by the compositor
	bufW, bufH int
	bufs       [2]*shmBuf
	pool       *client.ShmPool
	poolFile   *os.File
	poolMem    []byte
	last       []byte
	px, py     int // last pointer position, window-local
	events     chan Event
}

func newWayland(width, height int) (Window, error) {
	display, err := client.Connect("")
	if err != nil {
		return nil, fmt.Errorf("wayland connect: %w", err)
	}
	w := &wlWindow{
		display: display, ctx: display.Context(),
		w: width, h: height, outScale: 1,
		events: make(chan Event, 256),
	}
	display.SetErrorHandler(func(e client.DisplayErrorEvent) {
		fmt.Fprintf(os.Stderr, "wayland protocol error %d: %s\n", e.Code, e.Message)
	})
	if err := w.setup(); err != nil {
		w.ctx.Close()
		return nil, err
	}
	go w.loop()
	return w, nil
}

// dispatchOne reads and handles one event. Callers hold mu or are single-threaded.
func (w *wlWindow) dispatchOne() error {
	id, opcode, fd, data, err := w.ctx.ReadMsg()
	if err != nil {
		return err
	}
	if p, ok := w.ctx.GetProxy(id).(client.Dispatcher); ok {
		p.Dispatch(opcode, fd, data)
	}
	return nil
}

func (w *wlWindow) roundtrip() error {
	cb, err := w.display.Sync()
	if err != nil {
		return err
	}
	done := false
	cb.SetDoneHandler(func(client.CallbackDoneEvent) { done = true })
	for !done {
		if err := w.dispatchOne(); err != nil {
			return err
		}
	}
	return nil
}

// bindGlobal is wl_registry.bind. client.Registry.Bind encodes the interface
// string length padded instead of len+NUL, which strict compositors
// (smithay/COSMIC) reject by disconnecting the client.
func bindGlobal(reg *client.Registry, name uint32, iface string, ver uint32, p client.Proxy) error {
	padded := client.PaddedLen(len(iface) + 1)
	buf := make([]byte, 20+padded+4)
	client.PutUint32(buf[0:], reg.ID())
	client.PutUint32(buf[4:], uint32(len(buf))<<16) // opcode 0
	client.PutUint32(buf[8:], name)
	client.PutUint32(buf[12:], uint32(len(iface)+1))
	copy(buf[16:], iface)
	client.PutUint32(buf[16+padded:], ver)
	client.PutUint32(buf[20+padded:], p.ID())
	return reg.Context().WriteMsg(buf, nil)
}

func (w *wlWindow) setup() error {
	reg, err := w.display.GetRegistry()
	if err != nil {
		return err
	}
	var bindErr error
	bind := func(name uint32, iface string, ver uint32, p client.Proxy) {
		if err := bindGlobal(reg, name, iface, ver, p); err != nil && bindErr == nil {
			bindErr = err
		}
	}
	reg.SetGlobalHandler(func(e client.RegistryGlobalEvent) {
		switch e.Interface {
		case "wl_compositor":
			w.compositor = client.NewCompositor(w.ctx)
			bind(e.Name, e.Interface, min(e.Version, 4), w.compositor)
		case "wl_shm":
			w.shm = client.NewShm(w.ctx)
			bind(e.Name, e.Interface, 1, w.shm)
		case "wl_seat":
			w.seat = client.NewSeat(w.ctx)
			bind(e.Name, e.Interface, min(e.Version, 5), w.seat)
		case "wl_output":
			if w.output == nil { // first output only
				w.output = client.NewOutput(w.ctx)
				bind(e.Name, e.Interface, min(e.Version, 2), w.output)
			}
		case "zwlr_layer_shell_v1":
			w.shell = layershell.NewLayerShell(w.ctx)
			bind(e.Name, e.Interface, 1, w.shell)
		}
	})
	if err := w.roundtrip(); err != nil {
		return err
	}
	if bindErr != nil {
		return bindErr
	}
	if w.compositor == nil || w.shm == nil || w.shell == nil {
		return fmt.Errorf("compositor lacks wl_compositor/wl_shm/zwlr_layer_shell_v1 (GNOME does not support layer-shell)")
	}

	if w.output != nil {
		w.output.SetModeHandler(func(e client.OutputModeEvent) {
			if e.Flags&1 != 0 { // current mode
				w.outW, w.outH = int(e.Width), int(e.Height)
			}
		})
		w.output.SetScaleHandler(func(e client.OutputScaleEvent) { w.outScale = max(int(e.Factor), 1) })
	}
	if w.seat != nil {
		w.seat.SetCapabilitiesHandler(w.onSeatCaps)
	}
	if err := w.roundtrip(); err != nil { // output + seat events
		return err
	}
	if w.outW > 0 {
		w.x = (w.outW/w.outScale - w.w) / 2
		w.y = (w.outH/w.outScale - w.h) / 2
	}

	if w.surface, err = w.compositor.CreateSurface(); err != nil {
		return err
	}
	w.layer, err = w.shell.GetLayerSurface(w.surface, nil, uint32(layershell.LayerShellLayerOverlay), "bongocat")
	if err != nil {
		return err
	}
	w.layer.SetConfigureHandler(func(e layershell.LayerSurfaceConfigureEvent) {
		w.layer.AckConfigure(e.Serial)
		w.cfgW, w.cfgH = int(e.Width), int(e.Height)
	})
	w.layer.SetClosedHandler(func(layershell.LayerSurfaceClosedEvent) { w.emit(Event{Kind: Closed}) })
	w.layer.SetAnchor(uint32(layershell.LayerSurfaceAnchorTop | layershell.LayerSurfaceAnchorLeft))
	w.layer.SetExclusiveZone(-1)
	w.layer.SetKeyboardInteractivity(uint32(layershell.LayerSurfaceKeyboardInteractivityNone))
	w.layer.SetSize(uint32(w.w), uint32(w.h))
	w.layer.SetMargin(int32(w.y), 0, 0, int32(w.x))
	if err := w.setClickThrough(true); err != nil {
		return err
	}
	if err := w.surface.Commit(); err != nil {
		return err
	}
	for w.cfgW == 0 { // wait for the first configure
		if err := w.dispatchOne(); err != nil {
			return err
		}
	}
	return nil
}

func (w *wlWindow) onSeatCaps(e client.SeatCapabilitiesEvent) {
	if e.Capabilities&uint32(client.SeatCapabilityPointer) == 0 {
		return
	}
	ptr, err := w.seat.GetPointer()
	if err != nil {
		return
	}
	ptr.SetEnterHandler(func(e client.PointerEnterEvent) { w.px, w.py = int(e.SurfaceX), int(e.SurfaceY) })
	ptr.SetMotionHandler(func(e client.PointerMotionEvent) {
		w.px, w.py = int(e.SurfaceX), int(e.SurfaceY)
		w.emit(w.pointerEvent(Motion, 0))
	})
	ptr.SetButtonHandler(func(e client.PointerButtonEvent) {
		var b uint8
		if e.Button == btnLeft {
			b = 1
		}
		k := Release
		if e.State == uint32(client.PointerButtonStatePressed) {
			k = Press
		}
		w.emit(w.pointerEvent(k, b))
	})
}

func (w *wlWindow) pointerEvent(k Kind, b uint8) Event {
	return Event{Kind: k, Button: b, X: w.px, Y: w.py, RootX: w.x + w.px, RootY: w.y + w.py}
}

func (w *wlWindow) emit(e Event) {
	select {
	case w.events <- e:
	default: // consumer is behind; drop
	}
}

// loop pumps compositor events until the connection closes.
func (w *wlWindow) loop() {
	for {
		id, opcode, fd, data, err := w.ctx.ReadMsg()
		if err != nil {
			w.emit(Event{Kind: Closed})
			return
		}
		w.mu.Lock()
		if p, ok := w.ctx.GetProxy(id).(client.Dispatcher); ok {
			p.Dispatch(opcode, fd, data)
		}
		w.mu.Unlock()
	}
}

func (w *wlWindow) Size() (int, int) { w.mu.Lock(); defer w.mu.Unlock(); return w.w, w.h }
func (w *wlWindow) Pos() (int, int)  { w.mu.Lock(); defer w.mu.Unlock(); return w.x, w.y }

func (w *wlWindow) Poll() (Event, bool) {
	select {
	case e := <-w.events:
		return e, true
	default:
		return Event{}, false
	}
}

func (w *wlWindow) SetClickThrough(on bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.setClickThrough(on); err != nil {
		return err
	}
	return w.surface.Commit()
}

func (w *wlWindow) setClickThrough(on bool) error {
	if !on {
		return w.surface.SetInputRegion(nil) // whole surface
	}
	r, err := w.compositor.CreateRegion() // empty region
	if err != nil {
		return err
	}
	defer r.Destroy()
	return w.surface.SetInputRegion(r)
}

func (w *wlWindow) Move(x, y int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.x, w.y = x, y
	w.layer.SetMargin(int32(y), 0, 0, int32(x))
	w.surface.Commit()
}

func (w *wlWindow) MoveResize(x, y, width, height int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.x, w.y, w.w, w.h = x, y, width, height
	w.layer.SetMargin(int32(y), 0, 0, int32(x))
	w.layer.SetSize(uint32(width), uint32(height))
	w.surface.Commit()
}

// PutImage commits a frame. It is skipped while the compositor has not yet
// acked the current size, when nothing changed, or when both buffers are busy.
func (w *wlWindow) PutImage(bgra []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.w != w.cfgW || w.h != w.cfgH || len(bgra) != w.w*w.h*4 {
		return
	}
	if w.bufW != w.w || w.bufH != w.h {
		if err := w.allocBuffers(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return
		}
	} else if bytes.Equal(bgra, w.last) {
		return
	}
	b := w.bufs[0]
	if b.busy {
		b = w.bufs[1]
		if b.busy {
			return
		}
	}
	copy(b.mem, bgra)
	b.busy = true
	w.surface.Attach(b.buf, 0, 0)
	w.surface.DamageBuffer(0, 0, int32(w.w), int32(w.h))
	w.surface.Commit()
	w.last = append(w.last[:0], bgra...)
}

// allocBuffers (re)creates a shm pool holding two w×h ARGB8888 buffers.
func (w *wlWindow) allocBuffers() error {
	w.freeBuffers()
	stride := w.w * 4
	size := stride * w.h * 2

	f, err := os.CreateTemp(os.Getenv("XDG_RUNTIME_DIR"), "bongocat-shm-")
	if err != nil {
		return fmt.Errorf("shm file: %w", err)
	}
	os.Remove(f.Name())
	if err := f.Truncate(int64(size)); err != nil {
		f.Close()
		return fmt.Errorf("shm truncate: %w", err)
	}
	mem, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return fmt.Errorf("shm mmap: %w", err)
	}
	pool, err := w.shm.CreatePool(int(f.Fd()), int32(size))
	if err != nil {
		syscall.Munmap(mem)
		f.Close()
		return err
	}
	w.poolFile, w.poolMem, w.pool = f, mem, pool
	for i := range w.bufs {
		off := i * stride * w.h
		buf, err := pool.CreateBuffer(int32(off), int32(w.w), int32(w.h), int32(stride), uint32(client.ShmFormatArgb8888))
		if err != nil {
			return err
		}
		b := &shmBuf{buf: buf, mem: mem[off : off+stride*w.h]}
		buf.SetReleaseHandler(func(client.BufferReleaseEvent) { b.busy = false })
		w.bufs[i] = b
	}
	w.bufW, w.bufH, w.last = w.w, w.h, nil
	return nil
}

func (w *wlWindow) freeBuffers() {
	for i, b := range w.bufs {
		if b != nil {
			b.buf.Destroy()
			w.bufs[i] = nil
		}
	}
	if w.pool != nil {
		w.pool.Destroy()
		syscall.Munmap(w.poolMem)
		w.poolFile.Close()
		w.pool = nil
	}
}

func (w *wlWindow) Destroy() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.freeBuffers()
	w.layer.Destroy()
	w.surface.Destroy()
	w.ctx.Close()
}
