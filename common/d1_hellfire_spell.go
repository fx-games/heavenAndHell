package common

import (
	"fmt"
	"rpg/d1"
	"rpg/geometry"
)

// d1Doppelganger ports AddDoppelganger: a fresh monster of the same type (full life, its own rolls) on the first
// free cell around it, walking the directions from south-east round.
func (f *Factory) d1Doppelganger(original *Actor) {
	m := f.State.GetMap()
	for _, d := range [8]geometry.Point{{X: 1, Y: 1}, {Y: 1}, {X: -1, Y: 1}, {X: -1}, {X: -1, Y: -1}, {Y: -1}, {X: 1, Y: -1}, {X: 1}} {
		p := original.MapPosition().Add(d)
		if !m.Contains(p) || !m.CanPlaceActorHere(p) {
			continue
		}
		if m.IsObjectAt(p) {
			if o, isD1 := m.ObjectAt(p).(*D1Object); isD1 && o.O.Solid {
				continue
			}
		}
		a := f.NewD1Monster(original.d1Monster.Type)
		a.d1Monster.SpawnIndex = -1 // saved in LevelSnapshot.Spawned
		a.d1Monster.Dir = original.d1Monster.Dir
		a.SetDrawPosition(p.ToPointF())
		m.AddActor(a, p)
		f.State.OnActorSpawned(a)
		f.Msg(fmt.Sprintf("The %s splits in two!", original.Name()))
		return
	}
}

// d1Warp ports missiles_warp + MI_Teleport: the hero lands beside the nearest stairs or town warp.
func (f *Factory) d1Warp(c *spellCtx) bool {
	lvl := f.State.mapGen.levels[c.m.GetName()]
	if lvl == nil {
		return false
	}
	o := c.origin()
	best, found := geometry.Point{}, false
	for _, t := range lvl.Triggers {
		switch t.Msg {
		case d1.TrigTownWarpUp, d1.TrigPrevLevel, d1.TrigNextLevel, d1.TrigReturnLevel:
		default:
			continue
		}
		p := toCell(t.X+1, t.Y)
		if (lvl.Type == d1.DTypeCathedral || lvl.Type == d1.DTypeCatacombs) && t.Msg != d1.TrigTownWarpUp {
			p = toCell(t.X, t.Y+1)
		}
		if !found || geometry.DistanceSquared(o, p) < geometry.DistanceSquared(o, best) {
			best, found = p, true
		}
	}
	if !found {
		return false
	}
	dest, ok := c.freeNear(best, func(p geometry.Point) bool {
		_, stairs := c.m.GetTransitionAt(p)
		return !stairs && c.m.IsCurrentlyPassable(p) && !c.m.IsObjectAt(p)
	})
	if !ok {
		return false
	}
	f.d1MovePlayer(c.caster, dest, "Teleport")
	return true
}

// d1Berserk ports missiles_berserk: the nearest monster around the target that isn't unique, Diablo or immune to
// magic (resistant ones half the time) turns on its own kind, hitting 20-29% harder plus the spell level.
// ponytail: a berserk monster fights like the golem, in melee; its damage bonus is lost when the level is loaded.
func (f *Factory) d1Berserk(c *spellCtx, target geometry.Point, slvl int) bool {
	var victim *Actor
	c.freeNear(target, func(p geometry.Point) bool {
		a, ok := c.m.TryGetActorAt(p)
		if !ok || a.d1Monster == nil || !a.IsAlive() {
			return false
		}
		mon := a.d1Monster
		if mon.Golem || mon.Berserk || mon.Unique != nil || mon.AI == "AI_DIABLO" || mon.MagicRes&d1.ImmuneMagic != 0 {
			return false
		}
		if mon.MagicRes&d1.ResistMagic != 0 && d1Rng.Intn(2) != 0 {
			return false
		}
		victim = a
		return true
	})
	if victim == nil {
		return false
	}
	mon := victim.d1Monster
	grow := func(v int) int { return v*(d1Rng.Intn(10)+120)/100 + slvl }
	mon.MinDam, mon.MaxDam = grow(mon.MinDam), grow(mon.MaxDam)
	mon.MinDam2, mon.MaxDam2 = grow(mon.MinDam2), grow(mon.MaxDam2)
	mon.Berserk = true
	c.ctx.Msg(fmt.Sprintf("%s goes berserk!", victim.Name()))
	return true
}
