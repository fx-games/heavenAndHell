package d1

import "testing"

// A few graphics land where items.cpp says (common's TestD1SfxHaveSoundEvents checks every one has an event).
func TestItemSnds(t *testing.T) {
	d := testData(t)
	if drop, inv := d.ItemSnds(ICursBastardSword); drop != "IS_FSWOR" || inv != "IS_ISWORD" {
		t.Errorf("bastard sword: %s %s", drop, inv)
	}
	if drop, _ := d.ItemSnds(-1); drop != "" {
		t.Error("out of range")
	}
}
