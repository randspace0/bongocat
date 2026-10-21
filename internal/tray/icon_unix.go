//go:build !windows

package tray

func trayIcon(png []byte) []byte { return png }
