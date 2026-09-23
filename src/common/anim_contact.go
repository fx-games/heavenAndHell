package common

import "rpg/src/geometry"

// syncToProjectiles holds back the hit flashes of an action that fires projectiles until the projectile gets
// there. The state has changed already, so a cancel or the player acting again skips the flight and shows them
// at once.
func (d *Broker) syncToProjectiles(anims []Animation) {
	flying := flightsIn(anims)
	if len(flying) == 0 {
		return
	}
	for _, a := range anims {
		if h, ok := a.(*HitAnimation); ok && h.startThisTick == nil {
			h.StartWhenTrue(d.contact(flying, h.targetPos))
		}
	}
}

// flightsIn are the projectiles among anims.
func flightsIn(anims []Animation) []*ProjectileDirectionAnimation {
	var flying []*ProjectileDirectionAnimation
	for _, a := range anims {
		if p, ok := a.(*ProjectileDirectionAnimation); ok {
			flying = append(flying, p)
		}
	}
	return flying
}

// whenShown is when a blow that landed at p on actor may be shown: once the missile that caused it arrives, or
// once the player has seen the actor reach p (a step, or a push). nil means right away, which is the usual case:
// a blow between two actors standing still.
func (d *Broker) whenShown(p geometry.Point, actor *Actor) func() bool {
	if len(d.flights) > 0 {
		if flying := d.flights[len(d.flights)-1]; len(flying) > 0 {
			return d.contact(flying, p)
		}
	}
	if actor == nil || actor.DrawPosition().ToPoint() == p {
		return nil
	}
	return d.givingUp(func() bool { return actor.DrawPosition().ToPoint() == p }, nil)
}

// contact is true once the projectile passing closest to p got there.
func (d *Broker) contact(flying []*ProjectileDirectionAnimation, p geometry.Point) func() bool {
	var by *ProjectileDirectionAnimation
	at, best := 0.0, 0.0
	for _, f := range flying {
		if along, dist := f.arrival(p); by == nil || dist < best {
			by, at, best = f, along, dist
		}
	}
	return d.givingUp(func() bool { return by.IsDone() || by.flown() >= at }, by.Cancel)
}

// givingUp is gate, but true anyway once the player has taken another turn, running onGiveUp first. What a gate
// waits for can stop coming: an actor killed on its way to p never walks the rest of the way, and its blow then
// waits for a step that will never happen. A gate that never opens is not a delay, it is a leak - the hit never
// plays, so it never finishes, so the queued actions behind it never run, and the dead actor it gates keeps
// VisualsPending forever, which draws it standing where it fell for the rest of the level.
func (d *Broker) givingUp(gate func() bool, onGiveUp func()) func() bool {
	turns := func() uint64 { return d.context.GameState.timekeeper.turnCounter }
	born := d.ticks
	var turn *uint64 // taken once the broker ticked, after the rest of this turn (e.g. the monsters' replies) ran
	return func() bool {
		if gate() {
			return true
		}
		if d.ticks == born {
			return false
		}
		if turn == nil {
			t := turns()
			turn = &t
		} else if turns() != *turn {
			if onGiveUp != nil {
				onGiveUp()
			}
			return true
		}
		return false
	}
}

// whenVisible runs fn now, or once the hit that changed a is shown (its death cry after the bolt hits).
func whenVisible(b *Broker, a *Actor, fn func()) {
	if a == nil || b == nil || !a.VisualsPending() {
		fn()
		return
	}
	b.AddAnimation(&gatedCall{gate: func() bool { return !a.VisualsPending() }, fn: fn})
}

type gatedCall struct {
	gate func() bool
	fn   func()
	done bool
}

func (g *gatedCall) IsDone() bool      { return g.done }
func (g *gatedCall) Draw(TileRenderer) {}
func (g *gatedCall) Cancel() {
	if !g.done {
		g.done = true
		g.fn()
	}
}
func (g *gatedCall) Tick() {
	if g.gate() {
		g.Cancel()
	}
}
