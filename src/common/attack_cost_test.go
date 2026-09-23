package common

import (
	"rpg/src/rules"
	"testing"
)

func TestAttackAUT(t *testing.T) {
	sword := [5]int{20, 19, 18, 16, 15} // 16 frames: none, quick, fast, faster, fastest
	for i, flag := range []uint32{0, rules.FlagAttackSpeed1, rules.FlagAttackSpeed2, rules.FlagAttackSpeed3, rules.FlagAttackSpeed4} {
		s := rules.Sheet{AttackFrames: 16, Flags: flag}
		if got := ticksToAUT(s.MeleeFrames()); got != sword[i] {
			t.Errorf("sword, affix %d: %d AUT, want %d", i, got, sword[i])
		}
		if got := ticksToAUT(s.AttackFrames); got != 20 { // ranged attacks skip no frames
			t.Errorf("unskipped, affix %d: %d AUT, want 20", i, got)
		}
	}
}
