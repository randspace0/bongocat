package tray

import "github.com/getlantern/systray"

type Tray struct {
	MoveCh chan bool
	QuitCh chan struct{}
}

func New(iconPNG []byte) *Tray {
	t := &Tray{
		MoveCh: make(chan bool, 1),
		QuitCh: make(chan struct{}),
	}
	go systray.Run(func() { t.onReady(iconPNG) }, nil)
	return t
}

func (t *Tray) onReady(iconPNG []byte) {
	systray.SetIcon(iconPNG)
	systray.SetTooltip("BongoCat")

	mMove := systray.AddMenuItemCheckbox("Enable Move and Resize", "Drag or resize the window", false)
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
