package input

import "testing"

func TestTapTracker(t *testing.T) {
	var tr tapTracker
	tr.down()
	tr.move(absX, 1000)
	tr.move(absX, 1010)
	if !tr.up() {
		t.Fatal("short still contact should be a tap")
	}

	tr.down()
	tr.move(absX, 1000)
	tr.move(absX, 1000+tapMaxTravel+1)
	if tr.up() {
		t.Fatal("moving contact should not be a tap")
	}

	tr.down()
	tr.cancel()
	if tr.up() {
		t.Fatal("contact with physical click should not be a tap")
	}
}
