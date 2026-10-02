package tray

import (
	"fmt"
	"log"

	"github.com/getlantern/systray"
)

type Tray struct {
	MoveCh    chan bool
	SkinCh    chan string
	OpacityCh chan int
	QuitCh    chan struct{}
}

func New(iconPNG []byte, skins []string, current string, opacity int) *Tray {
	t := &Tray{
		MoveCh:    make(chan bool, 1),
		SkinCh:    make(chan string, 1),
		OpacityCh: make(chan int, 1),
		QuitCh:    make(chan struct{}),
	}
	go systray.Run(func() { t.onReady(iconPNG, skins, current, opacity) }, nil)
	return t
}

func (t *Tray) onReady(iconPNG []byte, skins []string, current string, opacity int) {
	systray.SetIcon(iconPNG)
	systray.SetTooltip("BongoCat")

	mMove := systray.AddMenuItemCheckbox("Enable Move and Resize", "Drag or resize the window", false)

	mSkin := systray.AddMenuItem("Character", "Choose a character")
	items := make(map[string]*systray.MenuItem, len(skins))
	for _, name := range skins {
		item := mSkin.AddSubMenuItemCheckbox(name, "", name == current)
		items[name] = item
		go func(name string, item *systray.MenuItem) {
			for range item.ClickedCh {
				for n, it := range items {
					if n == name {
						it.Check()
					} else {
						it.Uncheck()
					}
				}
				t.SkinCh <- name
			}
		}(name, item)
	}

	mOpacity := systray.AddMenuItem("Opacity", "Window transparency")
	levels := map[int]*systray.MenuItem{}
	for _, pct := range []int{100, 75, 50, 25} {
		item := mOpacity.AddSubMenuItemCheckbox(fmt.Sprintf("%d%%", pct), "", pct == opacity)
		levels[pct] = item
		go func(pct int, item *systray.MenuItem) {
			for range item.ClickedCh {
				for p, it := range levels {
					if p == pct {
						it.Check()
					} else {
						it.Uncheck()
					}
				}
				t.OpacityCh <- pct
			}
		}(pct, item)
	}

	mStart := systray.AddMenuItemCheckbox("Launch on Startup", "Start BongoCat at login", autostartEnabled())

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit BongoCat")

	for {
		select {
		case <-mMove.ClickedCh:
			if mMove.Checked() {
				mMove.Uncheck()
				t.MoveCh <- false
			} else {
				mMove.Check()
				t.MoveCh <- true
			}
		case <-mStart.ClickedCh:
			on := !mStart.Checked()
			if err := setAutostart(on); err != nil {
				log.Printf("autostart: %v", err)
				break
			}
			if on {
				mStart.Check()
			} else {
				mStart.Uncheck()
			}
		case <-mQuit.ClickedCh:
			systray.Quit()
			close(t.QuitCh)
			return
		}
	}
}
