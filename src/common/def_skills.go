package common

import (
	"cmp"
	"fmt"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
	"rpg/src/util"
	"slices"
)

// The engine's named skills (src/rules/skills.rec). Each is an on-hit effect like any other, so a mod can hand one
// to a class, a scroll or a monster by name; the cast appends the skill level as the last argument, which is what
// each one scales with.

// The statuses the skills leave on an actor. A negative value never ticks down: it lasts until something uses it.
const (
	StatusDebilitating StatusEffect = "debilitating_shot" // the next arrow that hits debilitates; value -level
	StatusTriumph      StatusEffect = "triumph_of_steel"  // kills heal and add damage
	StatusTriumphKills StatusEffect = "triumph_kills"     // value -kills made under Triumph of Steel
	StatusEternalVow   StatusEffect = "eternal_vow"       // the next lethal blow leaves 1 life
	StatusMultiShot    StatusEffect = "multi_shot"        // Multi-Shot is up (the spell's id)
	StatusShield       StatusEffect = "shield"            // value -points of damage it still absorbs
)

func skillLevel(args util.Arguments) int {
	if len(args) == 0 {
		return 1
	}
	return max(args.GetInt(len(args)-1), 1)
}

func (g *Factory) skillEffects() map[string]func(args util.Arguments) *HitEffect {
	return map[string]func(args util.Arguments) *HitEffect{
		"arrow_rain":        g.arrowRain,
		"trick_shot":        g.trickShot,
		"debilitating_shot": g.debilitatingShot,
		"man_stopper":       g.manStopper,
		"crush":             g.crush,
		"shout":             g.shout,
		"swing":             g.swing,
		"whirlwind":         g.whirlwind,
		"triumph_of_steel":  g.triumphOfSteel,
		"call_to_odin":      g.callToOdin,
		"black_clouds":      g.blackClouds,
		"burning_rain":      g.burningRain,
		"great_thunder":     g.greatThunder,
		"eternal_vow":       g.eternalVow,
		"bat_out_of_hell":   g.batOutOfHell,
		"shield":            g.shield,
	}
}

func skillEffect(desc string, activate func(a *Actor, target geometry.Point, hits []geometry.Point)) *HitEffect {
	return &HitEffect{Activate: activate, Description: []util.LabelText{util.LabelTextFromString(desc)}}
}

// foeAt is the living actor at p that is hostile to a, if any.
func (g *Factory) foeAt(a *Actor, p geometry.Point) (*Actor, bool) {
	v, ok := g.State.GetMap().TryGetActorAt(p)
	if !ok || v == a || !v.IsAlive() || !v.IsHostileTo(a) {
		return nil, false
	}
	return v, true
}

// againstWall: a wall or anything else unwalkable stands next to p.
func (g *Factory) againstWall(p geometry.Point) bool {
	m := g.State.GetMap()
	for _, d := range geometry.Dirs {
		if !m.IsTileWalkable(p.Add(d)) {
			return true
		}
	}
	return false
}

func pinned(v *Actor) bool {
	return v.HasStatusEffect(StatusStone) || v.HasStatusEffect(StatusStunned) || v.HasStatusEffect(StatusImmobile)
}

// box lists the cells of the square of radius r around c.
func box(c geometry.Point, r int) []geometry.Point {
	var cells []geometry.Point
	for y := -r; y <= r; y++ {
		for x := -r; x <= r; x++ {
			cells = append(cells, c.Add(geometry.Point{X: x, Y: y}))
		}
	}
	return cells
}

func (g *Factory) shoot(a *Actor, p geometry.Point, why string) *RangedAttackAction {
	return NewRangedAttackAction(NewActorCause(ActionName(why), a), a, p)
}

func (g *Factory) blow(a, v *Actor, factor float64, why string) {
	if g.againstWall(v.MapPosition()) && factor > 0 {
		factor *= 1.5
	}
	attack := NewMeleeAttackAction(NewActorCause(ActionName(why), a), a, v)
	attack.SetDamageFactor(factor)
	g.ActionBroker.Execute(attack)
}

// arrowRain: arrows fall on a 3x3 area (5x5 from level 3); each foe there is struck half the time, a pinned one always.
func (g *Factory) arrowRain(args util.Arguments) *HitEffect {
	r := 1
	if skillLevel(args) >= 3 {
		r = 2
	}
	return skillEffect("Rain arrows on an area", func(a *Actor, target geometry.Point, _ []geometry.Point) {
		for _, p := range box(target, r) {
			if v, ok := g.foeAt(a, p); ok && (pinned(v) || rng.Intn(2) == 0) {
				g.ActionBroker.Execute(g.shoot(a, p, "Arrow Rain"))
			}
		}
	})
}

// trickShot: one arrow flies on in the aimed direction (one of eight), through level+1 foes (all from level 5),
// bouncing off level+1 walls.
// ponytail: bounces on the eight grid directions; a true reflected ray if the angles ever matter.
func (g *Factory) trickShot(args util.Arguments) *HitEffect {
	l := skillLevel(args)
	pierce, bounces := l+1, l+1
	if l >= 5 {
		pierce = 1 << 30
	}
	return skillEffect("Shoot an arrow that pierces and bounces", func(a *Actor, target geometry.Point, _ []geometry.Point) {
		m := g.State.GetMap()
		p := a.MapPosition()
		d := geometry.Dirs[DirTo(p, target)]
		left, walls := pierce, bounces
		for step := 0; step < 40 && left > 0; step++ {
			n := p.Add(d)
			if !m.IsTileWalkable(n) {
				if walls == 0 {
					return
				}
				walls--
				switch {
				case m.IsTileWalkable(p.Add(geometry.Point{X: -d.X, Y: d.Y})):
					d.X = -d.X
				case m.IsTileWalkable(p.Add(geometry.Point{X: d.X, Y: -d.Y})):
					d.Y = -d.Y
				default:
					d = geometry.Point{X: -d.X, Y: -d.Y}
				}
				continue
			}
			p = n
			if _, ok := g.foeAt(a, p); ok {
				g.ActionBroker.Execute(g.shoot(a, p, "Trick Shot"))
				left--
			}
		}
	})
}

// debilitatingShot readies the next arrow that hits: see debilitate.
func (g *Factory) debilitatingShot(args util.Arguments) *HitEffect {
	l := skillLevel(args)
	return skillEffect("Your next arrow debilitates", func(a *Actor, _ geometry.Point, _ []geometry.Point) {
		a.AddStatusEffect(StatusDebilitating, -l)
		g.logIfPlayer(a, "You ready a debilitating arrow.")
	})
}

// debilitate is what a readied Debilitating Shot does to the foe its arrow hit: pinned down against a wall,
// otherwise stunned, scared, confused, enraged or poisoned, for level+1 turns.
func (g *Factory) debilitate(a, v *Actor) []Action {
	l := -a.statusEffects.AllEntries()[string(StatusDebilitating)]
	a.GetStatusEffects().SetFalse(string(StatusDebilitating))
	turns, cause := l+1, NewActorCause("Debilitating Shot", a)
	if g.againstWall(v.MapPosition()) {
		return []Action{NewApplyStatusAction(cause, v, StatusImmobile, turns)}
	}
	switch rng.Intn(5) {
	case 0:
		return []Action{NewApplyStatusAction(cause, v, StatusStunned, turns)}
	case 1:
		return []Action{NewApplyStatusAction(cause, v, StatusScared, turns)}
	case 2:
		return []Action{NewApplyStatusAction(cause, v, StatusConfused, turns)}
	case 3:
		return []Action{NewApplyStatusAction(cause, v, StatusBerserk, turns)}
	}
	return []Action{NewApplyStatusParametrizedAction(cause, v, g.NewPoisonedEffect(turns, 1+l))}
}

// manStopper: a shot that knocks what it hits back a cell per level (a wall it hits hurts, see KnockbackAction).
func (g *Factory) manStopper(args util.Arguments) *HitEffect {
	l := skillLevel(args)
	return skillEffect("Shoot and knock back", func(a *Actor, target geometry.Point, _ []geometry.Point) {
		shot := g.shoot(a, target, "Man Stopper")
		shot.SetAdditionalOnHitActions(func(attacker *Actor, at geometry.Point, _ []geometry.Point) []Action {
			v, ok := g.State.GetMap().TryGetActorAt(at)
			if !ok || !v.IsAlive() {
				return nil
			}
			return []Action{NewKnockbackAction(NewActorCause("Man Stopper", attacker), v, geometry.Dirs[DirTo(attacker.MapPosition(), at)], l)}
		})
		g.ActionBroker.Execute(shot)
	})
}

// crush: a blow at 1+0.2·level damage through the target and the foe behind it.
func (g *Factory) crush(args util.Arguments) *HitEffect {
	factor := 1 + 0.2*float64(skillLevel(args))
	return skillEffect("Crush two enemies in a line", func(a *Actor, target geometry.Point, _ []geometry.Point) {
		d := geometry.Dirs[DirTo(a.MapPosition(), target)]
		for _, p := range []geometry.Point{target, target.Add(d)} {
			if v, ok := g.foeAt(a, p); ok {
				g.blow(a, v, factor, "Crush")
			}
		}
	})
}

// shout: every monster on the level comes for the shouter, and those who see him flee for level+2 turns.
func (g *Factory) shout(args util.Arguments) *HitEffect {
	turns := skillLevel(args) + 2
	return skillEffect("Alert the level, scare who sees you", func(a *Actor, _ geometry.Point, _ []geometry.Point) {
		for _, v := range g.State.GetMap().Actors() {
			if v.monster != nil && v.IsAlive() && v.IsHostileTo(a) {
				alert(v, a)
			}
		}
		for _, v := range g.getVisibleEnemiesByDistance(a, false) {
			g.ActionBroker.Execute(NewApplyStatusAction(NewActorCause("Shout", a), v, StatusScared, turns))
		}
		g.logIfPlayer(a, "You shout!")
	})
}

// swing: one blow at every foe in the three cells in front.
func (g *Factory) swing(args util.Arguments) *HitEffect {
	return skillEffect("Strike the three cells in front of you", func(a *Actor, target geometry.Point, _ []geometry.Point) {
		o, d := a.MapPosition(), DirTo(a.MapPosition(), target)
		for _, dir := range []int{(d + 7) % 8, d, (d + 1) % 8} {
			if v, ok := g.foeAt(a, o.Add(geometry.Dirs[dir])); ok {
				g.blow(a, v, 1, "Swing")
			}
		}
	})
}

// whirlwind: spin up to level cells in a line, striking every foe on the way, and stop on the last free cell.
func (g *Factory) whirlwind(args util.Arguments) *HitEffect {
	l := skillLevel(args)
	return skillEffect("Spin through a line of enemies", func(a *Actor, target geometry.Point, _ []geometry.Point) {
		m := g.State.GetMap()
		d := geometry.Dirs[DirTo(a.MapPosition(), target)]
		end := a.MapPosition()
		for i, p := 1, a.MapPosition().Add(d); i <= l && m.IsTileWalkable(p); i, p = i+1, p.Add(d) {
			if v, ok := g.foeAt(a, p); ok {
				g.blow(a, v, 1, "Whirlwind of Doom")
			} else if m.IsCurrentlyPassable(p) {
				end = p
			}
		}
		if end != a.MapPosition() {
			move := NewMoveAction(NewActorCause("Whirlwind of Doom", a), a, end, a == g.State.GetPlayer())
			move.DisableAnimation()
			g.ActionBroker.Execute(move)
		}
	})
}

// triumphOfSteel: for 5·level turns every kill heals a tenth of life and adds 10% damage (see triumph).
func (g *Factory) triumphOfSteel(args util.Arguments) *HitEffect {
	turns := 5 * skillLevel(args)
	return skillEffect("Kills heal you and add damage", func(a *Actor, _ geometry.Point, _ []geometry.Point) {
		a.AddStatusEffect(StatusTriumph, turns)
		a.GetStatusEffects().SetFalse(string(StatusTriumphKills))
		g.logIfPlayer(a, "Steel will triumph!")
	})
}

// triumph is a kill under Triumph of Steel.
func (g *Factory) triumph(killer *Actor) {
	if killer == nil || !killer.HasStatusEffect(StatusTriumph) || !killer.IsAlive() {
		return
	}
	kills := -killer.statusEffects.AllEntries()[string(StatusTriumphKills)]
	killer.AddStatusEffect(StatusTriumphKills, -(kills + 1))
	heal := max(killer.GetStats().GetResource(stats.Health).GetMax()/10, 1)
	g.ActionBroker.Execute(NewChangeResourceAction(NewActorCause("Triumph of Steel", killer), killer, stats.Health, float64(heal)))
}

// skillDamageFactor is what the skills' statuses do to a blow's or an arrow's damage.
func skillDamageFactor(a *Actor) float64 {
	if !a.HasStatusEffect(StatusTriumph) {
		return 1
	}
	return 1 + 0.1*float64(-a.statusEffects.AllEntries()[string(StatusTriumphKills)])
}

// callToOdin: lightning strikes up to level+1 visible foes, nearest first.
func (g *Factory) callToOdin(args util.Arguments) *HitEffect {
	l := skillLevel(args)
	return skillEffect("Lightning strikes your enemies", func(a *Actor, _ geometry.Point, _ []geometry.Point) {
		foes := g.getVisibleEnemiesByDistance(a, false)
		for _, v := range foes[:min(l+1, len(foes))] {
			p := v.MapPosition()
			g.ActionBroker.Execute(NewSpellDamageAction(a, "lightning", rules.ResistKindLightning, "", l, p, []geometry.Point{p}))
		}
	})
}

// lingering keeps an effect on some cells for a number of turns, running each on every turn (and once at once).
func (g *Factory) lingering(p geometry.Point, turns int, icon int32, each func(turnsLeft int)) {
	m := g.State.GetMap()
	if m.IsObjectAt(p) {
		return
	}
	o := NewFire(0)
	o.icon, o.turnsUntilRemoval = icon, turns
	o.SetDealDamage(func(geometry.Point) {})
	o.SetOnTurn(func() {
		if !o.IsAlive() || g.State.GetMap() != m {
			m.RemoveObject(o)
			o.turnsUntilRemoval = 0
			return
		}
		each(o.turnsUntilRemoval)
	})
	g.GetTimeKeeper().AddTimed(o)
	m.AddObject(o, p)
	each(turns)
}

// blackClouds: a cloud of radius 1 (2 from level 3) that poisons and confuses whatever stands in it, for level+3 turns.
// ponytail: confused stands in for blinded, which no code reads yet; blind them once something does.
func (g *Factory) blackClouds(args util.Arguments) *HitEffect {
	l := skillLevel(args)
	r := 1
	if l >= 3 {
		r = 2
	}
	return skillEffect("A choking cloud", func(a *Actor, target geometry.Point, _ []geometry.Point) {
		m := g.State.GetMap()
		for _, p := range box(target, r) {
			if !m.Contains(p) || !m.IsTileWalkable(p) {
				continue
			}
			cell := p
			g.lingering(cell, l+3, defs.IconEffectHit, func(int) {
				if v, ok := g.foeAt(a, cell); ok {
					cause := NewActorCause("Black Clouds", a)
					g.ActionBroker.Execute(NewApplyStatusAction(cause, v, StatusConfused, 2))
					g.ActionBroker.Execute(NewApplyStatusParametrizedAction(cause, v, g.NewPoisonedEffect(2, l)))
				}
			})
		}
	})
}

// burningRain: for level+2 turns, fire falls on three random cells within two of the target every turn.
func (g *Factory) burningRain(args util.Arguments) *HitEffect {
	l := skillLevel(args)
	return skillEffect("Fire rains down", func(a *Actor, target geometry.Point, _ []geometry.Point) {
		m := g.State.GetMap()
		area := box(target, 2)
		g.lingering(target, l+2, 0, func(int) {
			for i := 0; i < 3; i++ {
				if p := area[rng.Intn(len(area))]; m.Contains(p) && m.IsTileWalkable(p) && a.hero != nil {
					g.spawnFireWall(a, p, l, 1)
				}
			}
		})
	})
}

// greatThunder: a thunderbolt of one and a half Fireballs on the target; everything next to it is stunned
// for 1 + level/2 turns.
func (g *Factory) greatThunder(args util.Arguments) *HitEffect {
	l := skillLevel(args)
	return skillEffect("A thunderbolt that stuns", func(a *Actor, target geometry.Point, _ []geometry.Point) {
		g.ActionBroker.Execute(NewSpellDamageAction(a, "great_thunder", rules.ResistKindLightning, "", l, target, []geometry.Point{target}))
		for _, d := range geometry.Dirs {
			if v, ok := g.foeAt(a, target.Add(d)); ok {
				g.ActionBroker.Execute(NewApplyStatusAction(NewActorCause("Great Thunder", a), v, StatusStunned, 1+l/2))
			}
		}
	})
}

// eternalVow: for 5·level turns the first blow that would kill leaves 1 life instead (see ChangeResourceAction).
func (g *Factory) eternalVow(args util.Arguments) *HitEffect {
	turns := 5 * skillLevel(args)
	return skillEffect("Survive one lethal blow", func(a *Actor, _ geometry.Point, _ []geometry.Point) {
		a.AddStatusEffect(StatusEternalVow, turns)
		g.logIfPlayer(a, "You swear an eternal vow.")
	})
}

// batOutOfHell: fly up to level+2 cells in a line through foes, burning each one passed with a Firebolt, and land
// on the last free cell.
func (g *Factory) batOutOfHell(args util.Arguments) *HitEffect {
	l := skillLevel(args)
	return skillEffect("Fly through your enemies", func(a *Actor, target geometry.Point, _ []geometry.Point) {
		m := g.State.GetMap()
		d := geometry.Dirs[DirTo(a.MapPosition(), target)]
		end, burnt := a.MapPosition(), []geometry.Point(nil)
		for i, p := 1, a.MapPosition().Add(d); i <= l+2 && m.IsTileWalkable(p); i, p = i+1, p.Add(d) {
			if _, ok := g.foeAt(a, p); ok {
				burnt = append(burnt, p)
			} else if m.IsCurrentlyPassable(p) {
				end = p
			}
		}
		if len(burnt) > 0 {
			g.ActionBroker.Execute(NewSpellDamageAction(a, "firebolt", rules.ResistKindFire, "", l, target, burnt))
		}
		if end != a.MapPosition() {
			move := NewMoveAction(NewActorCause("Bat out of Hell", a), a, end, a == g.State.GetPlayer())
			move.DisableAnimation()
			g.ActionBroker.Execute(move)
		}
	})
}

// multiShotVolley is what Multi-Shot adds to a hero's shot at target: one more arrow per level at the nearest other
// foes, each paid for with the skill's mana; it stops at the first arrow he cannot pay for.
func (g *Factory) multiShotVolley(a *Actor, target geometry.Point) []Action {
	h := a.hero
	spell, ok := g.Generator.D.SpellIdx(string(StatusMultiShot))
	if h == nil || !ok || !a.HasStatusEffect(StatusMultiShot) {
		return nil
	}
	s := a.HeroSheet()
	cost := rules.Whole(g.Generator.D.ManaCost(h, s, spell))
	mana := a.GetStats().GetResource(stats.Mana)
	var volley []Action
	for _, v := range g.getVisibleEnemiesByDistance(a, false) {
		if len(volley) >= h.SpellLevel(s, spell) {
			break
		}
		if v.MapPosition() == target {
			continue
		}
		if mana.Value() < cost {
			g.logIfPlayer(a, fmt.Sprintf("Not enough mana for %s.", g.Generator.D.Spells[spell].Name))
			break
		}
		mana.Decrease(cost)
		h.ManaBase -= rules.Fixed(cost)
		shot := g.shoot(a, v.MapPosition(), "Multi-Shot")
		shot.volley = true
		volley = append(volley, shot)
	}
	return volley
}

// shieldPoints is the damage a Shield of this level absorbs.
func shieldPoints(level int) int { return 10 * level }

// shield: the caster, or the ally on the target cell, absorbs the next shieldPoints(level) damage it takes.
func (g *Factory) shield(args util.Arguments) *HitEffect {
	l := skillLevel(args)
	return skillEffect(fmt.Sprintf("Absorb %d damage", shieldPoints(l)), func(a *Actor, target geometry.Point, _ []geometry.Point) {
		if v, ok := g.State.GetMap().TryGetActorAt(target); ok && v.IsAlive() && v.isHostile == a.isHostile {
			g.raiseShield(a, v, l)
		}
	})
}

// shieldAllies: the untargeted Shield a monster casts on its 3 nearest unshielded allies in sight (not itself).
func (g *Factory) shieldAllies(args util.Arguments) UntargetedEffect {
	l := skillLevel(args)
	return UntargetedEffect{Effect: func(a *Actor, _ EffectSource, _ bool) {
		allies := g.unshieldedAllies(a)
		for _, v := range allies[:min(3, len(allies))] {
			g.raiseShield(a, v, l)
		}
	}}
}

// unshieldedAllies are a's living, unshielded allies it can see, nearest first.
func (g *Factory) unshieldedAllies(a *Actor) []*Actor {
	var allies []*Actor
	for _, v := range g.State.GetMap().Actors() {
		if v != a && v.IsAlive() && v.isHostile == a.isHostile && !v.HasStatusEffect(StatusShield) && a.CanSee(v.MapPosition()) {
			allies = append(allies, v)
		}
	}
	slices.SortStableFunc(allies, func(x, y *Actor) int {
		return cmp.Compare(geometry.DistanceSquared(a.MapPosition(), x.MapPosition()), geometry.DistanceSquared(a.MapPosition(), y.MapPosition()))
	})
	return allies
}

func (g *Factory) raiseShield(caster, v *Actor, level int) {
	v.AddStatusEffect(StatusShield, -shieldPoints(level))
	v.shieldedBy = caster.id
	icon, ok := defs.IconOK("effect.shield")
	if !ok {
		icon = defs.IconEffectHit
	}
	g.ActionBroker.Emit(Flash{At: v.MapPosition(), Seconds: 0.8, Icon: icon, Fade: true})
}

// absorbShield takes up to dam damage off v's Shield and returns how much it took.
func absorbShield(v *Actor, dam float64) float64 {
	if !v.HasStatusEffect(StatusShield) {
		return 0
	}
	left := float64(-v.statusEffects.AllEntries()[string(StatusShield)])
	took := min(left, dam)
	if left -= took; left < 1 {
		v.GetStatusEffects().SetFalse(string(StatusShield))
		v.shieldedBy = 0
	} else {
		v.AddStatusEffect(StatusShield, -int(left))
	}
	return took
}

// dropShieldsOf removes every Shield the dead caster raised.
func dropShieldsOf(actors []*Actor, caster *Actor) {
	for _, v := range actors {
		if v.shieldedBy == caster.id {
			v.GetStatusEffects().SetFalse(string(StatusShield))
			v.shieldedBy = 0
		}
	}
}

// DropOrphanShields drops every Shield on m (and the hero's) whose caster is neither on m nor the hero: one that
// did not come back from a save, or from a save without casters.
func DropOrphanShields(m *gridmap.GridMap[*Actor, *Item, Object], hero *Actor) {
	here := map[uint64]bool{}
	actors := m.Actors()
	if hero != nil {
		here[hero.id] = true
		actors = append(slices.Clip(actors), hero)
	}
	for _, a := range m.Actors() {
		here[a.id] = true
	}
	for _, v := range actors {
		if v.HasStatusEffect(StatusShield) && !here[v.shieldedBy] {
			v.GetStatusEffects().SetFalse(string(StatusShield))
			v.shieldedBy = 0
		}
	}
}

// ShieldedBy is the ID of whoever cast a's Shield; 0 for none.
func (a *Actor) ShieldedBy() uint64 { return a.shieldedBy }

// SetShieldedBy restores ShieldedBy from a save.
func (a *Actor) SetShieldedBy(id uint64) { a.shieldedBy = id }
