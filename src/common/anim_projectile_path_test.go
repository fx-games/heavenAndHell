package common

import (
	"testing"

	"rpg/src/geometry"
)

func TestProjectilePathFollowsEveryCell(t *testing.T) {
	// a zig-zag: right, then down
	m := NewProjectilePathAnimation(nil, []geometry.Point{{X: 0, Y: 0}, {X: 2, Y: 0}, {X: 2, Y: 2}}, 0.1, 0, 1)
	for _, c := range []struct {
		t    float64
		want geometry.PointF
	}{{0, geometry.PointF{}}, {0.25, geometry.PointF{X: 1}}, {0.5, geometry.PointF{X: 2}}, {0.75, geometry.PointF{X: 2, Y: 1}}, {1, geometry.PointF{X: 2, Y: 2}}} {
		if got := m.position(c.t); got != c.want {
			t.Errorf("position(%v) = %v, want %v", c.t, got, c.want)
		}
	}
	before := m.ticksNeeded
	m.Extend(geometry.Point{X: 4, Y: 2})
	if m.ticksNeeded <= before || m.End() != (geometry.Point{X: 4, Y: 2}) {
		t.Errorf("Extend: ticks %d -> %d, end %v", before, m.ticksNeeded, m.End())
	}
}

// A hit's visuals wait until the bolt reaches the victim, or the player acts again, which skips the flight.
func TestHitVisualsWaitForContact(t *testing.T) {
	b := &Broker{context: ActionContext{GameState: &State{timekeeper: NewTimeKeeper(nil)}}}
	bolt := NewProjectileDirectionAnimation(nil, geometry.Point{}, geometry.Point{X: 10}, 0.1, 0, 1)
	hit := b.contact([]*ProjectileDirectionAnimation{bolt}, geometry.Point{X: 5, Y: 1}) // beside the path: closest at x 5
	for !hit() {
		b.ticks++
		bolt.Tick()
	}
	if f := bolt.flown(); f < 5 || f > 5.2 {
		t.Fatalf("shown after flying %v tiles, want 5", f)
	}

	bolt = NewProjectileDirectionAnimation(nil, geometry.Point{}, geometry.Point{X: 10}, 0.1, 0, 1)
	hit = b.contact([]*ProjectileDirectionAnimation{bolt}, geometry.Point{X: 10})
	b.context.GameState.timekeeper.PlayerTakesTurn(10) // the cast's own turn, before the broker ticked: no skip
	b.ticks++
	if hit() {
		t.Fatal("shown before the bolt flew")
	}
	b.context.GameState.timekeeper.PlayerTakesTurn(10)
	if !hit() || !bolt.IsDone() {
		t.Fatal("acting again didn't skip the flight")
	}
}

// A monster hit by a bolt steps away only after the hit has played on its tile.
func TestMoveWaitsForProjectileHit(t *testing.T) {
	landed := false
	hit := NewHitAnimation(geometry.Point{}, 0.2, 0, 1)
	hit.StartWhenTrue(func() bool { return landed })
	a := &Actor{moveHold: hit}
	move := &MovementAnimation{actor: a, path: []geometry.Point{{}, {X: 1}}, ticksNeeded: 4}
	move.Tick()
	hit.Tick()
	if move.ticksAlive != 0 {
		t.Fatal("moved before the bolt landed")
	}
	landed = true
	for !hit.IsDone() {
		hit.Tick()
		move.Tick()
		if !hit.IsDone() && move.ticksAlive != 0 {
			t.Fatal("moved while the hit played")
		}
	}
	if move.ticksAlive == 0 {
		t.Fatal("didn't move after the hit")
	}
}
