package window

import (
	"fmt"

	"github.com/randspace0/bongocat/internal/xwindow"
	"github.com/randspace0/bongocat/xshape"
)

const (
	Width  = 397
	Height = 201
)

type Window struct {
	*xwindow.ARGBWindow
}

func New() (*Window, error) {
	xwin, err := xwindow.New(-1, -1, Width, Height)
	if err != nil {
		return nil, fmt.Errorf("window: %w", err)
	}

	if err := xshape.MakeClickThrough(xwin.Conn, xwin.Wid); err != nil {
		xwin.Destroy()
		return nil, fmt.Errorf("window: click-through: %w", err)
	}

	return &Window{xwin}, nil
}
