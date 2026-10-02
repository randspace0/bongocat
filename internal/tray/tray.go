package tray

import "github.com/getlantern/systray"

type Tray struct {
	MoveCh chan bool
	SkinCh chan string
	QuitCh chan struct{}
}

func New(iconPNG []byte, skins []string, current string) *Tray {
	t := &Tray{
		MoveCh: make(chan bool, 1),
		SkinCh: make(chan string, 1),
		QuitCh: make(chan struct{}),
	}
	go systray.Run(func() { t.onReady(iconPNG, skins, current) }, nil)
	return t
}

func (t *Tray) onReady(iconPNG []byte, skins []string, current string) {
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
		case <-mQuit.ClickedCh:
			systray.Quit()
			close(t.QuitCh)
			return
		}
	}
}
