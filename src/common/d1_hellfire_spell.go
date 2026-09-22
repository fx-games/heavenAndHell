package common

import (
	"fmt"
	"rpg/src/d1"
	"rpg/src/geometry"
	"rpg/src/util"
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
		obj, _ := m.TryGetObjectAt(p)
		if o, isD1 := obj.(*D1Object); isD1 && o.O.Solid {
			continue
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
	lvl := f.State.mapGen.d1Levels[c.m.GetName()]
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

// enrage_monster (effect: enrage_monster) ports missiles_berserk: the first monster in the hit zone that isn't
// unique, Diablo or immune to magic (resistant ones half the time) turns on its own kind, hitting 20-29% harder
// plus the spell level, which the cast appends as the one argument.
// ponytail: a berserk monster fights like the golem, in melee; its damage bonus is lost when the level is loaded.
func (g *Factory) enrageMonster(args util.Arguments) *HitEffect {
	slvl := args.GetInt(0)
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			m := g.State.GetMap()
			for _, p := range hitPositions {
				a, ok := m.TryGetActorAt(p)
				if !ok || a.d1Monster == nil || !a.IsAlive() {
					continue
				}
				mon := a.d1Monster
				if mon.Golem || mon.Berserk || mon.Unique != nil || mon.AI == "AI_DIABLO" || mon.MagicRes&d1.ImmuneMagic != 0 {
					continue
				}
				if mon.MagicRes&d1.ResistMagic != 0 && rng.Intn(2) != 0 {
					continue
				}
				grow := func(v int) int { return v*(rng.Intn(10)+120)/100 + slvl }
				mon.MinDam, mon.MaxDam = grow(mon.MinDam), grow(mon.MaxDam)
				mon.MinDam2, mon.MaxDam2 = grow(mon.MinDam2), grow(mon.MaxDam2)
				mon.Berserk = true
				g.Msg(fmt.Sprintf("%s goes berserk!", a.Name()))
				return
			}
		},
		Description: []util.LabelText{util.LabelTextFromString("Turn a monster on its own kind")},
	}
}

// apply_reflect_self ports missiles_reflection: the blows Reflect turns back are the spell level (the argument
// when it is zero) times the character level. Hero bookkeeping, reached by name like any other self-cast.
func (g *Factory) getReflectSelfAction(args util.Arguments) UntargetedEffect {
	whenUnlearned := args.GetIntOrDefault(0, 2)
	slvl := args.GetInt(len(args) - 1) // the cast appends it
	return UntargetedEffect{
		Effect: func(user *Actor, source EffectSource) {
			h := user.hero
			if h == nil {
				return
			}
			blows := slvl
			if blows == 0 {
				blows = whenUnlearned
			}
			h.Reflections += blows * h.Level
			g.Msg(fmt.Sprintf("You will turn back the next %d blows.", h.Reflections))
		},
		Description: []util.LabelText{util.LabelTextFromString("Turn the next blows back on their attacker")},
	}
}
