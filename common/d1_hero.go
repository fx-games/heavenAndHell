package common

import (
	"fmt"
	"math/rand"
	"rpg/d1"
	"rpg/geometry"
	"rpg/stats"
	"time"
)

var d1Rng = rand.New(rand.NewSource(time.Now().UnixNano()))

func (a *Actor) Hero() *d1.Hero     { return a.hero }
func (a *Actor) SetHero(h *d1.Hero) { a.hero = h }

// HeroSheet derives D1's character sheet from the hero and the D1 items they wear.
func (a *Actor) HeroSheet() d1.Sheet {
	return a.hero.Sheet(a.equippedD1())
}

// SyncHero copies the D1 character sheet into the stats the UI and energy system read.
func SyncHero(a *Actor) {
	h := a.hero
	if h == nil {
		return
	}
	s := a.HeroSheet()
	st := a.GetStats()
	st.SetAttribute(stats.Strength, s.Str)
	st.SetAttribute(stats.Magic, s.Mag)
	st.SetAttribute(stats.Dexterity, s.Dex)
	st.SetAttribute(stats.Vitality, s.Vit)
	st.SetAttribute(stats.Level, h.Level)
	st.SetAttribute(stats.Experience, h.Exp)
	st.SetAttribute(stats.MaxHealth, s.MaxHP>>6)
	st.SetAttribute(stats.MaxMana, s.MaxMana>>6)
	st.GetResource(stats.Health).Increase(0) // clamp to the new maximum
	st.GetResource(stats.Mana).Increase(0)
	st.SetAttributeAsFloat(stats.AttackSpeedFactor, d1.SpeedTier(d1.RefAttackTicks, s.AttackFrames))
}

func (a *Actor) heroInfoText() []string {
	h, s, st := a.hero, a.HeroSheet(), a.GetStats()
	lines := []string{a.name, fmt.Sprintf("Level %d %s", h.Level, d1.ClassNames[h.Class]), "",
		fmt.Sprintf("HP: %s", st.GetResource(stats.Health).ToString()),
		fmt.Sprintf("Mana: %s", st.GetResource(stats.Mana).ToString()),
		fmt.Sprintf("Experience: %d / %d", h.Exp, d1.ExpLvls[min(h.Level, 50)]),
		"",
		fmt.Sprintf("Str %d  Mag %d  Dex %d  Vit %d", s.Str, s.Mag, s.Dex, s.Vit),
	}
	if h.StatPts > 0 {
		lines = append(lines, fmt.Sprintf("Unspent stat points: %d", h.StatPts))
	}
	minD := s.MinDam + s.MinDam*s.BonusDam/100 + s.BonusDamMod + s.DamageMod
	maxD := s.MaxDam + s.MaxDam*s.BonusDam/100 + s.BonusDamMod + s.DamageMod
	return append(lines, "",
		fmt.Sprintf("Armor class: %d", s.AC+s.BonusAC+s.Dex/5),
		fmt.Sprintf("To hit: %d%%", h.MeleeHitChance(s, 0)),
		fmt.Sprintf("Damage: %d-%d", minD, maxD),
		fmt.Sprintf("Resist fire %d%%  lightning %d%%  magic %d%%", s.FR, s.LR, s.MR),
		fmt.Sprintf("Gold: %d", st.GetAttribute(stats.Gold)),
	)
}

// UseD1Potion ports UseItem for potions and elixirs.
func UseD1Potion(user *Actor, miscID string) {
	h := user.hero
	if h == nil {
		return
	}
	s := user.HeroSheet()
	hp, mana := user.GetStats().GetResource(stats.Health), user.GetStats().GetResource(stats.Mana)
	noMana := s.Flags&d1.ISplNoMana != 0
	heal := func() { hp.Increase(float64(h.PotionHeal(d1Rng, s.MaxHP) >> 6)) }
	restore := func() {
		if !noMana {
			mana.Increase(float64(h.PotionMana(d1Rng, s.MaxMana) >> 6))
		}
	}
	elixir := func(attr, n int) {
		dhp, dmana := h.AddBaseStat(attr, n)
		SyncHero(user)
		hp.Increase(float64(dhp >> 6))
		mana.Increase(float64(dmana >> 6))
	}
	switch miscID {
	case "IMISC_HEAL":
		heal()
	case "IMISC_FULLHEAL":
		hp.SetToMax()
	case "IMISC_MANA":
		restore()
	case "IMISC_FULLMANA":
		if !noMana {
			mana.SetToMax()
		}
	case "IMISC_REJUV":
		heal()
		restore()
	case "IMISC_FULLREJUV":
		hp.SetToMax()
		if !noMana {
			mana.SetToMax()
		}
	case "IMISC_ELIXSTR":
		elixir(d1.AttrStr, 1)
	case "IMISC_ELIXMAG":
		elixir(d1.AttrMag, 1)
	case "IMISC_ELIXDEX":
		elixir(d1.AttrDex, 1)
	case "IMISC_ELIXVIT":
		elixir(d1.AttrVit, 1)
	case "IMISC_SPECELIX":
		for attr := d1.AttrStr; attr <= d1.AttrVit; attr++ {
			elixir(attr, 3)
		}
	}
	SyncHero(user)
}

func d1DungeonLevel(state *State) int { return LevelFromName(state.GetMap().GetName()) }

// resolveD1Attack applies D1's hit formulas when a hero fights a D1 monster.
// handled=false means the pair isn't D1-driven and H&H's formulas apply.
func resolveD1Attack(ctx ActionContext, att, def *Actor, ranged bool) (dam int, extra []Action, handled bool) {
	state := ctx.GameState
	dist := geometry.DistanceChebyshev(att.MapPosition(), def.MapPosition())
	switch {
	case att.hero != nil && def.d1Monster != nil:
		h, s, mon := att.hero, att.HeroSheet(), def.d1Monster
		chance := h.MeleeHitChance(s, mon.AC)
		if ranged {
			chance = h.ArrowHitChance(s, mon.AC, dist)
		}
		if def.HasStatusEffect(StatusStone) {
			chance = 100
		}
		d1Alert(def, att)
		if d1Rng.Intn(100) >= chance {
			logMissed(ctx, att, def)
			return 0, nil, true
		}
		durrnd := 30
		if ranged {
			durrnd = 40
		}
		d1Broke(ctx, att, d1.WeaponDur(d1Rng, att.equippedD1(), durrnd))
		if ranged {
			dam = h.ArrowDamage(d1Rng, s)
		} else {
			dam = h.MeleeDamage(d1Rng, s, mon.Data.Class)
			if lhp, lmana := h.HitLeech(d1Rng, s, dam); lhp>>6 > 0 || lmana>>6 > 0 {
				extra = append(extra,
					NewChangeResourceAction(NewActorCause("Life steal", att), att, stats.Health, float64(lhp>>6)),
					NewChangeResourceAction(NewActorCause("Mana steal", att), att, stats.Mana, float64(lmana>>6)))
			}
		}
		dam = max(dam, 0)
		if s.Flags&d1.ISplNoHealMon != 0 {
			mon.NoHeal = true
		}
		return dam, extra, true
	case att.d1Monster != nil && def.hero != nil:
		h, s, mon := def.hero, def.HeroSheet(), att.d1Monster
		dlvl := d1DungeonLevel(state)
		hit, minDam, maxDam := mon.Hit, mon.MinDam, mon.MaxDam
		if mon.HitWith != nil { // M_StartSpAttack and charges
			hit, minDam, maxDam = mon.HitWith[0], mon.HitWith[1], mon.HitWith[2]
			mon.HitWith = nil
		}
		chance := h.MonsterMeleeHitChance(s, hit, mon.Level, dlvl)
		if ranged {
			chance = h.MonsterArrowHitChance(s, hit, mon.Level, dist, dlvl)
		}
		if d1Rng.Intn(100) >= chance {
			logMissed(ctx, att, def)
			return 0, nil, true
		}
		if d1Rng.Intn(100) < h.BlockChance(s, mon.Level) {
			ctx.Msg(fmt.Sprintf("You block the attack of %s.", att.Name()))
			d1Broke(ctx, def, d1.ShieldDur(def.equippedD1()))
			return 0, nil, true
		}
		d1Broke(ctx, def, d1.ArmorDur(d1Rng, def.equippedD1()))
		dam = d1ManaShield(def, d1.MonsterDamage(d1Rng, s, minDam, maxDam)>>6)
		if !ranged && s.Flags&d1.ISplThorns != 0 {
			extra = append(extra, NewChangeResourceAction(NewActorCause("Thorns", def), att, stats.Health, -float64(d1Rng.Intn(3)+1)))
		}
		return dam, extra, true
	case att.d1Monster != nil && def.d1Monster != nil: // the golem and the monsters fighting it: M_TryM2MHit
		mon := att.d1Monster
		mon.HitWith = nil
		if !def.HasStatusEffect(StatusStone) && d1Rng.Intn(100) >= mon.Hit {
			logMissed(ctx, att, def)
			return 0, nil, true
		}
		d1Alert(def, att)
		return mon.MinDam + d1Rng.Intn(max(mon.MaxDam-mon.MinDam+1, 1)), nil, true
	}
	return 0, nil, false
}

// d1Alert wakes a monster that is attacked before it noticed the player (MonsterMHit).
func d1Alert(victim, attacker *Actor) {
	if mon := victim.d1Monster; mon.Squelch == 0 {
		mon.Squelch, mon.Last = 255, attacker.MapPosition()
	}
}

// d1HitRecovery ports StartPlrHit and M_StartHit for any damage taken: a hard hit makes the victim flinch and lose
// time. A new hit restarts the recovery instead of adding to it. Ticks convert at a warrior's swing (RefAttackTicks
// ticks = one 10 AUT turn).
func d1HitRecovery(victim *Actor, dam int) {
	switch {
	case victim.hero != nil && dam >= victim.hero.Level:
		victim.hitRecovery = max(victim.hitRecovery, d1TicksToAUT(d1.HitRecoveryTicks(victim.hero.Class, victim.HeroSheet().Flags)))
	case victim.d1Monster != nil:
		d1MonsterGotHit(victim, dam)
	}
}

func d1TicksToAUT(ticks int) int { return (ticks*10 + d1.RefAttackTicks/2) / d1.RefAttackTicks }

// D1MonsterRecoveryAUT is the time a hard hit costs a monster: its hit animation minus the part of its attack
// animation after the hit frame, because our monsters' blows land when their action ends and D1's at mAFNum.
// Without that, a monster as fast as the hero could never strike back once every hit is hard.
func D1MonsterRecoveryAUT(mon *D1Monster) int {
	d := mon.Data
	return max(d1TicksToAUT(d.Frames[3]*(d.Rate[3]+1)-(d.Frames[2]-d.AFNum)*(d.Rate[2]+1)), 0)
}

// d1MonsterGotHit ports M_StartHit: AI side effects, and the monster loses D1MonsterRecoveryAUT of energy.
func d1MonsterGotHit(victim *Actor, dam int) {
	mon := victim.d1Monster
	if mon.Data.Family == "Sneak" || dam >= mon.Level+3 {
		if lost := D1MonsterRecoveryAUT(mon); lost > 0 && !victim.HasStatusEffect(StatusStone) && victim.GetEnergy() > -lost {
			victim.RemoveEnergy(victim.GetEnergy() + lost)
		}
		mon.LastMode, mon.StandTicks = D1ModeGotHit, 0
		if mon.Data.Family == "Scav" {
			mon.Goal = 1
		}
	}
}

var d1FallenRunDist = map[string]int{"MT_RFALLSP": 7, "MT_RFALLSD": 7, "MT_DFALLSP": 5, "MT_DFALLSD": 5,
	"MT_YFALLSP": 3, "MT_YFALLSD": 3, "MT_BFALLSP": 2, "MT_BFALLSD": 2}

// d1FallenFear ports M_FallenFear: nearby fallen ones flee from a death.
func d1FallenFear(state *State, pos geometry.Point) {
	for _, a := range state.GetMap().Actors() {
		mon := a.d1Monster
		if mon == nil || mon.AI != "AI_FALLEN" || !a.IsAlive() {
			continue
		}
		p := a.MapPosition()
		if rd := d1FallenRunDist[mon.Data.ID]; rd != 0 && geometry.DistanceChebyshev(p, pos) < 5 {
			mon.Goal, mon.GoalVar1, mon.Dir = 2, rd, D1DirTo(pos, p)
		}
	}
}

// d1OnKill ports experience, fallen fear and SpawnItem for a dead D1 monster.
func d1OnKill(ctx ActionContext, killer, victim *Actor) []Action {
	state := ctx.GameState
	mon := victim.d1Monster
	if killer.d1Monster != nil && killer.d1Monster.Golem {
		killer = state.GetPlayer() // the golem's kills are the player's
	}
	if mon.Golem {
		return nil
	}
	if h := killer.hero; h != nil {
		before := h.Level
		h.AddExperience(mon.Level, mon.Exp)
		SyncHero(killer)
		if h.Level > before {
			killer.GetStats().GetResource(stats.Health).SetToMax()
			killer.GetStats().GetResource(stats.Mana).SetToMax()
			ctx.Msg(fmt.Sprintf("You reached level %d! (%d stat points to spend)", h.Level, h.StatPts))
			state.factory.RefreshD1Spells(killer)
		}
	}
	d1QuestKill(ctx, mon)
	d1FallenFear(state, victim.MapPosition())
	state.factory.d1AcidDeath(ctx, victim)
	dlvl := d1DungeonLevel(state)
	if it := state.factory.D1().MonsterDrop(mon.Data, mon.Level, mon.Unique != nil, dlvl, dlvl > 12); it != nil {
		state.AddItemToMapWithDistribution(state.factory.NewD1Item(it), victim.MapPosition())
	}
	return nil
}
