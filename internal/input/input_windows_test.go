package input

import "testing"

func TestIsLeftWindows(t *testing.T) {
	for _, vk := range []uint8{'A', 'Q', '1', 0x20, 0xA0} {
		if !IsLeft(vk) {
			t.Errorf("vk %#x should be left", vk)
		}
	}
	for _, vk := range []uint8{'J', 'P', '9', 0xA1, 0x0D} {
		if IsLeft(vk) {
			t.Errorf("vk %#x should be right", vk)
		}
	}
}
