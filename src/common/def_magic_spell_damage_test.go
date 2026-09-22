package common

import (
	"rpg/src/util"
	"testing"
)

func TestSpellDamageModeAndLevel(t *testing.T) {
	for _, c := range []struct {
		args util.Arguments
		mode string
		slvl int
	}{
		{util.Arguments{"firebolt", "fire", "7"}, "", 7},
		{util.Arguments{"inferno", "fire", "per_cell", "3"}, "per_cell", 3},
	} {
		if mode, slvl := spellDamageModeAndLevel(c.args); mode != c.mode || slvl != c.slvl {
			t.Errorf("%v: got %q %d, want %q %d", c.args, mode, slvl, c.mode, c.slvl)
		}
	}
}
