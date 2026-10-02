package renderer

// ApplyOpacity scales a premultiplied BGRA frame in place to pct percent (1-100).
func ApplyOpacity(bgra []byte, pct int) {
	if pct >= 100 {
		return
	}
	for i, v := range bgra {
		bgra[i] = byte(int(v) * pct / 100)
	}
}
