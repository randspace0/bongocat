package renderer

import (
	"io/fs"
	"os"
	"testing"

	"github.com/randspace0/bongocat/internal/cat"
)

func TestAllSkinsLoadAndDraw(t *testing.T) {
	root := os.DirFS("../../assets/skins")
	names, err := Skins(root)
	if err != nil || len(names) == 0 {
		t.Fatalf("Skins: %v %v", names, err)
	}
	for _, name := range names {
		fsys, err := fs.Sub(root, name)
		if err != nil {
			t.Fatal(err)
		}
		r, err := New(fsys)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		w, h := r.Size()
		if w != NativeWidth {
			t.Errorf("%s: width %d, want %d", name, w, NativeWidth)
		}
		for _, s := range []cat.State{cat.Idle, cat.LeftDown, cat.RightDown, cat.BothDown} {
			if got := len(r.Draw(s, w, h, false)); got != w*h*4 {
				t.Errorf("%s state %d: %d bytes, want %d", name, s, got, w*h*4)
			}
		}
	}
}
