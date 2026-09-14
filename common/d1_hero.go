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
	var items []*d1.Item
	for _, it := range a.GetEquipment().AllItems() {
		if it.d1 != nil {
			items = append(items, it.d1)
		}
	}
	return a.hero.Sheet(items)
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
		d1Alert(def, att)
		if d1Rng.Intn(100) >= chance {
			logMissed(ctx, att, def)
			return 0, nil, true
		}
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
		d1MonsterGotHit(def, dam)
		return dam, extra, true
	case att.d1Monster != nil && def.hero != nil:
		h, s, mon := def.hero, def.HeroSheet(), att.d1Monster
		dlvl := d1DungeonLevel(state)
		chance := h.MonsterMeleeHitChance(s, mon.Hit, mon.Level, dlvl)
		if ranged {
			chance = h.MonsterArrowHitChance(s, mon.Hit, mon.Level, dist, dlvl)
		}
		if d1Rng.Intn(100) >= chance {
			logMissed(ctx, att, def)
			return 0, nil, true
		}
		if d1Rng.Intn(100) < h.BlockChance(s, mon.Level) {
			ctx.Msg(fmt.Sprintf("You block the attack of %s.", att.Name()))
			return 0, nil, true
		}
		dam = d1.MonsterDamage(d1Rng, s, mon.MinDam, mon.MaxDam) >> 6
		if !ranged && s.Flags&d1.ISplThorns != 0 {
			extra = append(extra, NewChangeResourceAction(NewActorCause("Thorns", def), att, stats.Health, -float64(d1Rng.Intn(3)+1)))
		}
		return dam, extra, true
	}
	return 0, nil, false
}

// d1Alert wakes a monster that is attacked before it noticed the player (MonsterMHit).
func d1Alert(victim, attacker *Actor) {
	if mon := victim.d1Monster; mon.Squelch == 0 {
		mon.Squelch, mon.Last = 255, attacker.MapPosition()
	}
}

// d1MonsterGotHit ports M_StartHit's AI side effects. ponytail: hit recovery (losing time) is deferred.
func d1MonsterGotHit(victim *Actor, dam int) {
	mon := victim.d1Monster
	if mon.Data.Family == "Sneak" || dam >= mon.Level+3 {
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
	if h := killer.hero; h != nil {
		before := h.Level
		h.AddExperience(mon.Level, mon.Exp)
		SyncHero(killer)
		if h.Level > before {
			killer.GetStats().GetResource(stats.Health).SetToMax()
			killer.GetStats().GetResource(stats.Mana).SetToMax()
			ctx.Msg(fmt.Sprintf("You reached level %d! (%d stat points to spend)", h.Level, h.StatPts))
		}
	}
	d1FallenFear(state, victim.MapPosition())
	dlvl := d1DungeonLevel(state)
	if it := state.factory.D1().MonsterDrop(mon.Data, mon.Level, mon.Unique != nil, dlvl, dlvl > 12); it != nil {
		state.AddItemToMapWithDistribution(state.factory.NewD1Item(it), victim.MapPosition())
	}
	return nil
}
