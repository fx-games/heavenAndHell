package gui

import (
	"rpg/src/util"
	"testing"
)

// The death ceremony is the one screen the player is not allowed to hurry: it holds the world for a moment so
// the killing blow lands, then bleeds for several seconds, hits every heartbeat exactly once, and only then
// hands over to the menu - once, however long it keeps running afterwards.
func TestDeathCeremonyRunsItsCourseBeforeTheMenu(t *testing.T) {
	menuOpenedAt, opens := 0.0, 0
	d := &deathScreen{}
	d.openMenu = func() { opens++; menuOpenedAt = d.seconds() }
	u := &UIController{death: d}

	ticksPerSecond := float64(util.SecondsToTicks(1))
	for range int((deathMenuAt + 2) * ticksPerSecond) {
		wasFrozen := d.frozen()
		u.updateDeathScreen()
		if !wasFrozen && d.seconds() > deathFreezeAt+1/ticksPerSecond {
			t.Fatalf("the world was still running at %.2fs, past the freeze at %.2fs", d.seconds(), deathFreezeAt)
		}
	}

	if opens != 1 {
		t.Fatalf("the game over menu opened %d times, want exactly once", opens)
	}
	if menuOpenedAt < 5 {
		t.Fatalf("the menu came up after %.2fs; the ceremony has to last at least 5s", menuOpenedAt)
	}
	if d.nextBeat != len(deathBeats) {
		t.Fatalf("heard %d of %d heartbeats", d.nextBeat, len(deathBeats))
	}
}
