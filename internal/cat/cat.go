package cat

import (
	"sync"
	"time"
)

type State int

const (
	Idle       State = iota
	LeftDown
	RightDown
	BothDown
)

// minPawDuration ensures a fast tap is still visually visible.
const minPawDuration = 80 * time.Millisecond

// Cat is the animation state machine. Safe for concurrent use.
type Cat struct {
	mu        sync.Mutex
	state     State
	leftHeld  bool
	rightHeld bool
	pawDownAt time.Time
}

func New() *Cat {
	return &Cat{}
}

func (c *Cat) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

func (c *Cat) LeftPress() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.leftHeld = true
	c.pawDownAt = time.Now()
	c.recalc()
}

func (c *Cat) LeftRelease() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.leftHeld = false
	c.releaseDelay()
}

func (c *Cat) RightPress() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rightHeld = true
	c.pawDownAt = time.Now()
	c.recalc()
}

func (c *Cat) RightRelease() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rightHeld = false
	c.releaseDelay()
}

// recalc updates state based on which paws are held. Must be called with mu held.
func (c *Cat) recalc() {
	switch {
	case c.leftHeld && c.rightHeld:
		c.state = BothDown
	case c.leftHeld:
		c.state = LeftDown
	case c.rightHeld:
		c.state = RightDown
	default:
		c.state = Idle
	}
}

// releaseDelay waits for minPawDuration before returning to idle,
// so fast taps are still visible. Must be called with mu held.
func (c *Cat) releaseDelay() {
	if c.leftHeld || c.rightHeld {
		c.recalc()
		return
	}
	elapsed := time.Since(c.pawDownAt)
	if elapsed < minPawDuration {
		remaining := minPawDuration - elapsed
		go func() {
			time.Sleep(remaining)
			c.mu.Lock()
			defer c.mu.Unlock()
			if !c.leftHeld && !c.rightHeld {
				c.state = Idle
			}
		}()
	} else {
		c.state = Idle
	}
}
