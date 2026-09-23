package common

import (
	"fmt"
	"rpg/src/dungen"
	"rpg/src/geometry"
	"rpg/src/rules"
	"rpg/src/util"
)

// doppelganger ports AddDoppelganger: a fresh monster of the same type (full life, its own rolls) on the first
// free cell around it, walking the directions from south-east round.
func (f *Factory) doppelganger(original *Actor) {
	m := f.State.GetMap()
	for _, d := range [8]geometry.Point{{X: 1, Y: 1}, {Y: 1}, {X: -1, Y: 1}, {X: -1}, {X: -1, Y: -1}, {Y: -1}, {X: 1, Y: -1}, {X: 1}} {
		p := original.MapPosition().Add(d)
		if !m.Contains(p) || !m.CanPlaceActorHere(p) {
			continue
		}
		obj, _ := m.TryGetObjectAt(p)
		if o, isD1 := obj.(*Prop); isD1 && o.O.Solid {
			continue
		}
		a := f.NewMonster(original.monster.Type)
		a.monster.SpawnIndex = -1 // saved in LevelSnapshot.Spawned
		a.monster.Dir = original.monster.Dir
		a.SetDrawPosition(p.ToPointF())
		m.AddActor(a, p)
		f.State.OnActorSpawned(a)
		f.Msg(fmt.Sprintf("The %s splits in two!", original.Name()))
		return
	}
}

// warp ports missiles_warp + MI_Teleport: the hero lands beside the nearest stairs or town warp.
func (f *Factory) warp(c *spellCtx) bool {
	lvl := f.State.mapGen.generated[c.m.GetName()]
	if lvl == nil {
		return false
	}
	o := c.origin()
	best, found := geometry.Point{}, false
	for _, t := range lvl.Triggers {
		switch t.Msg {
		case dungen.TrigTownWarpUp, dungen.TrigPrevLevel, dungen.TrigNextLevel, dungen.TrigReturnLevel:
		default:
			continue
		}
		p := toCell(t.X+1, t.Y)
		if (lvl.Type == rules.DTypeCathedral || lvl.Type == rules.DTypeCatacombs) && t.Msg != dungen.TrigTownWarpUp {
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
	f.movePlayer(c.caster, dest, "Teleport")
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
				if !ok || a.monster == nil || !a.IsAlive() {
					continue
				}
				mon := a.monster
				if mon.Golem || mon.Berserk || mon.Unique != nil || mon.Brain(g.Generator.D).ArchBoss || mon.MagicRes&rules.ImmuneMagic != 0 {
					continue
				}
				if mon.MagicRes&rules.ResistMagic != 0 && rng.Intn(2) != 0 {
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
		Effect: func(user *Actor, source EffectSource, forced bool) {
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
