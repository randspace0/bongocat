package input

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

// IsMouseLeft returns true for button 1 (left click).
func IsMouseLeft(button uint8) bool { return button == 1 }

// IsMouseRight returns true for button 3 (right click).
func IsMouseRight(button uint8) bool { return button == 3 }
