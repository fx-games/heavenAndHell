package common

import "testing"

// A blow on an actor that is not standing where it was drawn waits for it to get there. An actor killed on the
// way never does, and the gate used to stay shut for good: the hit never played, so the action queue behind it
// never drained, and the dead actor kept VisualsPending, which drew it standing where it fell forever.
func TestGateGivesUpWhenWhatItWaitsForNeverComes(t *testing.T) {
	b := &Broker{}
	b.context.GameState = &State{timekeeper: NewTimeKeeper(nil)}
	gaveUp := false
	gate := b.givingUp(func() bool { return false }, func() { gaveUp = true })

	if gate() {
		t.Fatal("the gate opened before anything happened")
	}
	b.ticks++
	if gate() {
		t.Fatal("the gate gave up within the turn its blow landed in")
	}
	b.context.GameState.timekeeper.turnCounter++
	if !gate() || !gaveUp {
		t.Fatalf("the gate did not give up after the player took another turn (opened %v, onGiveUp %v)", gate(), gaveUp)
	}
}
