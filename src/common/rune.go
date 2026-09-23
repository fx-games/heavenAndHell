package common

import (
	"fmt"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/rules"
	"rpg/src/util"
)

// runeAction makes a trap item placeable.
func (f *Factory) runeAction(item *Item, it *rules.Item) {
	rune, ok := f.Generator.D.RuneTraps[it.MiscID]
	if !ok {
		return
	}
	misc := it.MiscID
	action := NewTargetedAction(it.Name, OneTargetTile(f.State.GetMap, util.Arguments{}),
		&HitEffect{Activate: func(caster *Actor, target geometry.Point, _ []geometry.Point) {
			m := f.State.GetMap()
			c := &spellCtx{f: f, caster: caster, m: m}
			p, found := geometry.Point{}, false
			if c.lineClear(caster.MapPosition(), target) {
				p, found = c.freeNear(target, func(p geometry.Point) bool { return m.IsTileWalkable(p) && !m.IsObjectAt(p) })
			}
			if !found {
				f.Msg(fmt.Sprintf("The %s finds no place to lie.", it.Name))
				return
			}
			caster.GetInventory().Remove(item)
			f.spawnRune(caster, misc, p)
			f.Msg(fmt.Sprintf("You set the %s.", it.Name))
		}, Description: []util.LabelText{util.LabelTextFromString(fmt.Sprintf("A trap that goes off as %s", rune.Effect))}},
		spellTargeting(f, rune.Spell))
	item.SetTargetedAction(action)
	item.SetCharges(1) // single use, like scrolls/potions/books; the Activate closure above removes it from the pack itself
}

// spawnRune lays a trap: every turn it checks its cell and goes off under the first monster or hero on it.
func (f *Factory) spawnRune(caster *Actor, misc string, p geometry.Point) {
	m := f.State.GetMap()
	rune := NewFire(0)
	rune.icon, rune.turnsUntilRemoval = defs.IconEffectHit, 1<<30
	rune.SetDealDamage(func(geometry.Point) {})
	rune.SetOnTurn(func() {
		if f.State.GetMap() != m {
			return
		}
		victim, ok := m.TryGetActorAt(p)
		if !ok || victim.monster == nil && victim != f.State.GetPlayer() || victim.monster != nil && victim.monster.Golem {
			return
		}
		m.RemoveObject(rune)
		rune.turnsUntilRemoval = 0
		spell := f.Generator.D.RuneTraps[misc].Spell
		from := p
		f.ActionBroker.Execute(&spellTickAction{BaseAction: BaseAction{cause: NewActorCause(ActionNameFireball, caster), name: ActionNameFireball},
			caster: caster, spell: spell, run: func(c *spellCtx) {
				c.from = &from
				runeGoesOff(c, misc, victim.MapPosition())
			}})
	})
	f.GetTimeKeeper().AddTimed(rune)
	m.AddObject(rune, p)
}

// runeGoesOff ports the missile each trap turns into, at the trap's cell. MI_Rune adds it as TARGET_BOTH: the
// fire blast and the lightning wall hurt the hero as well; Nova and Immolation go out TARGET_MONSTERS again from a
// hero's trap (MI_Nova, mi_fire_nova), and Stone Curse only takes monsters.
func runeGoesOff(c *spellCtx, misc string, victim geometry.Point) {
	o, clvl := c.origin(), c.h.Level
	switch misc {
	case "IMISC_RUNEF": // missiles_rune_explosion: 2·(clvl + 2d10) + 4 on the 3×3 around the rune
		dam := 2*(clvl+rng.Intn(10)+rng.Intn(10)) + 4
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				p := o.Add(geometry.Point{X: dx, Y: dy})
				if a := c.hitCell(p); a != nil {
					c.strike(a, rules.Fixed(dam), 1)
				}
				c.hurtHero(p, dam, dam, false, 1)
			}
		}
		c.flash(o, 0.5, defs.IconEffectFireBig)
	case "IMISC_RUNEL": // AddRuneOfLight: a lightning wall piece of 16·(2d10 + clvl + 2) a tick on the rune for 255 ticks
		// (1.09 adds MIS_LIGHTBALL with no damage set, so there the rune does nothing)
		dam := 16 * (rng.Intn(10) + rng.Intn(10) + clvl + 2)
		c.f.spawnSpellWall(c.caster, o, rules.ResistKindLightning, 255/rules.RefWalkTicks, true, 1, defs.IconEffectLightning, func() int { return dam })
	case "IMISC_GR_RUNEF", "IMISC_GR_RUNEL": // MIS_IMMOLATION and MIS_NOVA: the rune's own spell, as its content declares it
		c.castFromData(o)
	case "IMISC_RUNES": // AddStone on the monster that stepped on it, unless stone_immune
		if a := c.monsterAt(victim); a != nil && !a.monster.Data.Has("stone_immune") {
			a.AddStatusEffect(StatusStone, rules.StoneTurns(0))
			c.flash(victim, 0.4, defs.IconEffectHit)
		}
	}
}
