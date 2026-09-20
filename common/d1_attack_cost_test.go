package common

import (
	"rpg/d1"
	"testing"
)

func TestD1AttackAUT(t *testing.T) {
	sword := [5]int{20, 19, 18, 16, 15} // 16 frames: none, quick, fast, faster, fastest
	for i, flag := range []uint32{0, d1.ISplQuickAttack, d1.ISplFastAttack, d1.ISplFasterAttack, d1.ISplFastestAttack} {
		s := d1.Sheet{AttackFrames: 16, Flags: flag}
		if got := d1TicksToAUT(s.MeleeFrames()); got != sword[i] {
			t.Errorf("sword, affix %d: %d AUT, want %d", i, got, sword[i])
		}
		if got := d1TicksToAUT(s.AttackFrames); got != 20 { // ranged attacks skip no frames
			t.Errorf("unskipped, affix %d: %d AUT, want 20", i, got)
		}
	}
}
