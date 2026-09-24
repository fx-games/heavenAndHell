package common

import (
	"fmt"
	"math/rand"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
	"strings"
	"time"
)

// rng is the one random source every roll of the running game draws from, so a seed replays a whole game.
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// Clock is the wall clock the game consults (the Solar shrine reads the hour, as D1's does). A reproducible run
// pins it, or its outcome depends on when it was started.
var Clock = time.Now

// SeedRandom pins the game's random source. Only reproducible runs use it - the game itself wants a fresh
// sequence every launch.
func SeedRandom(seed int64) { rng.Seed(seed) } // in place: everyone holding Random() follows

// Random is the game's random source.
func Random() *rand.Rand { return rng }

func (a *Actor) Hero() *rules.Hero     { return a.hero }
func (a *Actor) SetHero(h *rules.Hero) { a.hero = h }

// HeroSheet derives D1's character sheet from the hero and the D1 items they wear.
func (a *Actor) HeroSheet() rules.Sheet {
	s := a.hero.Sheet(a.equipped())
	if a.HasStatusEffect(StatusBlessed) { // +5 armor class, or double it from 5 up
		s.BonusAC += max(5, heroAC(s))
	}
	if a.HasStatusEffect(StatusStoneSkin) {
		s.BonusAC += max(0, StoneSkinAC-heroAC(s))
	}
	return s
}

// StoneSkinAC is the armor class Stoneskin raises the hero to; its other half is MoveCost's slow step.
const StoneSkinAC = 30

func heroAC(s rules.Sheet) int { return s.AC + s.BonusAC + s.Dex/5 }

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
	st.SetAttribute(stats.MaxHealth, rules.Whole(s.MaxHP))
	st.SetAttribute(stats.MaxMana, rules.Whole(s.MaxMana))
	st.GetResource(stats.Health).Increase(0) // clamp to the new maximum
	st.GetResource(stats.Mana).Increase(0)
}

// SheetAttribute is one of the four attributes on the character sheet: the base value stat points buy, the
// value with items on, the class maximum, and how to spend a point on it when one is left.
type SheetAttribute struct {
	Label    string
	Base     int
	Total    int
	Max      int
	CanRaise bool
	Raise    func()
}

// SheetSection is a titled block of label/value rows on the character sheet or in an info window. Notes are
// full-width prose lines under the rows, for what doesn't fit a label and a value (a monster's behavior, the
// rules behind a forecast).
type SheetSection struct {
	Title string
	Rows  [][2]string
	Notes []string
}

// SheetAttributes are the four attributes in D1's order, for the character sheet and for levelling up.
func (a *Actor) SheetAttributes() []SheetAttribute {
	h, s := a.hero, a.HeroSheet()
	if h == nil {
		return nil
	}
	base := [4]int{h.BaseStr, h.BaseMag, h.BaseDex, h.BaseVit}
	total := [4]int{s.Str, s.Mag, s.Dex, s.Vit}
	names := [4]stats.Name{stats.Strength, stats.Magic, stats.Dexterity, stats.Vitality}
	var out []SheetAttribute
	for i, label := range [4]string{"Strength", "Magic", "Dexterity", "Vitality"} {
		attr, name, maximum := i, names[i], h.Rules().Class(h.Class).MaxStats()[i]
		out = append(out, SheetAttribute{Label: label, Base: base[attr], Total: total[attr], Max: maximum,
			CanRaise: h.StatPts > 0 && base[attr] < maximum,
			Raise:    func() { a.LevelUp(name) }})
	}
	return out
}

// SkillRows are the skills the hero's class can put skill points into (classes.rec start_spells), as rows like the
// attributes': learned level, level with items, the class's max_skill_level, and a point to spend.
func (s *State) SkillRows(a *Actor) []SheetAttribute {
	h := a.hero
	if h == nil {
		return nil
	}
	sheet, maximum := a.HeroSheet(), h.Rules().Class(h.Class).MaxSkillLevel
	var out []SheetAttribute
	for spell := range h.SpellLevels {
		if !h.Learnable(spell) {
			continue
		}
		sp := spell
		out = append(out, SheetAttribute{Label: s.SpellName(spell), Base: h.SpellLevels[spell], Total: h.SpellLevel(sheet, spell),
			Max: maximum, CanRaise: h.SkillPts > 0 && h.SpellLevels[spell] < maximum,
			Raise: func() {
				if h.SpendSkillPoint(sp) {
					s.factory.RefreshSpells(a)
				}
			}})
	}
	return out
}

// SheetSections is everything on the character sheet apart from the four attributes.
func (a *Actor) SheetSections() []SheetSection {
	h, s, st := a.hero, a.HeroSheet(), a.GetStats()
	minD := s.MinDam + s.MinDam*s.BonusDam/100 + s.BonusDamMod + s.DamageMod
	maxD := s.MaxDam + s.MaxDam*s.BonusDam/100 + s.BonusDamMod + s.DamageMod
	return []SheetSection{
		{Title: "Character", Rows: [][2]string{
			{"Name", a.name},
			{"Class", h.Rules().Class(h.Class).Name},
			{"Level", fmt.Sprintf("%d", h.Level)},
			{"Experience", fmt.Sprintf("%d / %d", h.Exp, h.Rules().ExpLvls[min(h.Level, 50)])},
			{"Gold", fmt.Sprintf("%d", st.GetAttribute(stats.Gold))},
		}},
		{Title: "Condition", Rows: [][2]string{
			{"Life", st.GetResource(stats.Health).ToString()},
			{"Mana", st.GetResource(stats.Mana).ToString()},
		}},
		{Title: "Combat", Rows: append([][2]string{
			{"Armor class", fmt.Sprintf("%d", heroAC(s)) + a.statusNote(StatusBlessed, StatusStoneSkin)},
			{"Step", fmt.Sprintf("%d AUT", a.MoveCost()) + a.statusNote(StatusStoneSkin)},
			{"Chance to block", fmt.Sprintf("%d%% vs. own level", h.BlockChance(s, h.Level))},
			{"Chance to hit", fmt.Sprintf("%d%%", h.MeleeHitChance(s, 0))},
			{"Damage", fmt.Sprintf("%d-%d", minD, maxD)},
			{"Bonus from Str/Dex", fmt.Sprintf("+%d", s.DamageMod)},
		}, a.damageVsClassRows()...)},
		{Title: "Resistances", Rows: [][2]string{
			{"Fire", fmt.Sprintf("%d%%", s.FR)},
			{"Lightning", fmt.Sprintf("%d%%", s.LR)},
			{"Magic", fmt.Sprintf("%d%%", s.MR)},
		}},
		{Title: "Traits", Rows: a.traitRows()},
	}
}

// statusNote names the statuses behind a sheet value, with their turns left: " (Stoneskin, 7 turns)".
func (a *Actor) statusNote(effects ...StatusEffect) string {
	var notes []string
	for _, e := range effects {
		if a.HasStatusEffect(e) {
			notes = append(notes, fmt.Sprintf("%s, %d turns", StatusName(e), a.GetStatusEffects().AllEntries()[string(e)]))
		}
	}
	if len(notes) == 0 {
		return ""
	}
	return " (" + strings.Join(notes, "; ") + ")"
}

// ThreatSection says out loud that the dungeon got harder after Diablo, since nothing else announces it. It
// needs the world (the quest flags), so it is not part of the player's own SheetSections.
func (s *State) ThreatSection() SheetSection {
	threat := "Normal"
	if s.NewGamePlus() {
		threat = "Nightmare: Diablo is beaten"
	}
	return SheetSection{Title: "World", Rows: [][2]string{{"Threat", threat}}}
}

// damageVsClassRows spells out the melee damage the weapon in hand does against each kind of monster, but only
// where the kind changes it: the sword/mace rule and the "+200% vs. demons" affix are otherwise invisible.
func (a *Actor) damageVsClassRows() [][2]string {
	h, s := a.hero, a.HeroSheet()
	base := [2]int{}
	base[0], base[1] = h.MeleeDamageRange(s, "")
	var rows [][2]string
	for _, kind := range []struct{ class, label string }{{"MC_UNDEAD", "vs. the undead"}, {"MC_ANIMAL", "vs. animals"}, {"MC_DEMON", "vs. demons"}} {
		lo, hi := h.MeleeDamageRange(s, kind.class)
		if lo == base[0] && hi == base[1] {
			continue
		}
		rows = append(rows, [2]string{kind.label, fmt.Sprintf("%d-%d", lo, hi)})
	}
	return rows
}

// traitRows are the class' own rules, which nothing else in the game says out loud.
func (a *Actor) traitRows() [][2]string {
	h, s := a.hero, a.HeroSheet()
	rows := [][2]string{{"Missile damage bonus", "half of Str/Dex"}}
	if h.Rules().FullMissileDamageBonus(h.Class) {
		rows[0][1] = "all of Str/Dex"
	}
	if crit := h.CriticalStrikeChance(); crit > 0 {
		rows = append(rows, [2]string{"Critical strike", fmt.Sprintf("%d%% of swings hit twice", crit)})
	}
	if h.Rules().HasClassFlag(h.Class, defs.ClassTrapSense) {
		rows = append(rows, [2]string{"Trap sense", "sees traps on doors and chests"})
	}
	if h.Rules().HasClassFlag(h.Class, defs.ClassIronSkin) {
		rows = append(rows, [2]string{"Iron skin", fmt.Sprintf("+%d armor class from level", h.Level/4)})
	}
	if h.Rules().HasClassFlag(h.Class, defs.ClassNaturalResistance) {
		rows = append(rows, [2]string{"Natural resistance", fmt.Sprintf("+%d%% to every resistance", h.Level)})
	}
	if s.Flags&rules.FlagNoMana != 0 {
		rows = append(rows, [2]string{"No mana", "cannot cast spells that cost mana"})
	}
	if a.HasStatusEffect(StatusManaShield) {
		absorbed := "all damage paid from mana"
		if shieldTakesThird(a) {
			absorbed = "a third less damage, paid from mana"
		}
		rows = append(rows, [2]string{"Mana shield", absorbed})
	}
	return rows
}

// heroInfoText is the character sheet as plain lines, for the terminal UI and the stats server.
func (a *Actor) heroInfoText() []string {
	var lines []string
	for _, sec := range a.SheetSections() {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sec.Title)
		for _, row := range sec.Rows {
			lines = append(lines, fmt.Sprintf("%s: %s", row[0], row[1]))
		}
		if sec.Title != "Condition" {
			continue
		}
		lines = append(lines, "", "Attributes")
		for _, at := range a.SheetAttributes() {
			value := fmt.Sprintf("%d", at.Base)
			if at.Total != at.Base {
				value += fmt.Sprintf(" (%d)", at.Total)
			}
			lines = append(lines, fmt.Sprintf("%s: %s / %d", at.Label, value, at.Max))
		}
		if a.hero.StatPts > 0 {
			lines = append(lines, fmt.Sprintf("Unspent stat points: %d", a.hero.StatPts))
		}
	}
	return lines
}

// UsePotion ports UseItem for potions and elixirs.
func UsePotion(user *Actor, miscID string) {
	h := user.hero
	if h == nil {
		return
	}
	s := user.HeroSheet()
	hp, mana := user.GetStats().GetResource(stats.Health), user.GetStats().GetResource(stats.Mana)
	noMana := s.Flags&rules.FlagNoMana != 0
	heal := func() { hp.Increase(float64(rules.Whole(h.PotionHeal(rng, s.MaxHP)))) }
	restore := func() {
		if !noMana {
			mana.Increase(float64(rules.Whole(h.PotionMana(rng, s.MaxMana))))
		}
	}
	elixir := func(attr, n int) {
		dhp, dmana := h.AddBaseStat(attr, n)
		SyncHero(user)
		hp.Increase(float64(rules.Whole(dhp)))
		mana.Increase(float64(rules.Whole(dmana)))
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
		elixir(rules.AttrStr, 1)
	case "IMISC_ELIXMAG":
		elixir(rules.AttrMag, 1)
	case "IMISC_ELIXDEX":
		elixir(rules.AttrDex, 1)
	case "IMISC_ELIXVIT":
		elixir(rules.AttrVit, 1)
	case "IMISC_SPECELIX":
		for attr := rules.AttrStr; attr <= rules.AttrVit; attr++ {
			elixir(attr, 3)
		}
	}
	SyncHero(user)
}

// untouchable ports CheckMonsterHit and PlrHitMonst's early outs: a blow at a dormant gargoyle only wakes it,
// and a counselor-kind off its normal goal can't be hit; nor, by a weapon, an Illusion Weaver retreating.
// (MM_CHARGE never lasts past its own turn here.)
func untouchable(d *rules.Data, mon *Monster, weapon bool) bool {
	switch {
	case mon.Dormant:
		mon.Dormant = false
		return true
	case mon.Goal != goalNormal && (mon.Brain(d).Evasive || weapon && mon.Data.Has("weapon_proof_retreat") && mon.Goal == 2):
		return true
	}
	return false
}

// resolveAttack applies D1's hit formulas when a hero fights a D1 monster.
// handled=false means the pair isn't D1-driven and H&H's formulas apply.
func resolveAttack(ctx ActionContext, att, def *Actor, ranged bool) (dam int, extra []Action, handled bool) {
	state := ctx.GameState
	dist := geometry.DistanceChebyshev(att.MapPosition(), def.MapPosition())
	switch {
	case att.hero != nil && def.monster != nil:
		h, s, mon := att.hero, att.HeroSheet(), def.monster
		chance := h.MeleeHitChance(s, mon.AC)
		if ranged {
			chance = h.ArrowHitChance(s, mon.AC, dist)
		}
		if def.HasStatusEffect(StatusStone) {
			chance = 100
		}
		alert(def, att)
		if untouchable(ctx.GameState.factory.Generator.D, mon, true) {
			return 0, nil, true
		}
		blast := 0
		if !ranged { // PM_DoAttack: fire and lightning weapons explode on the swing's target, hit or miss (MI_Weapexp)
			if s.Flags&rules.FlagFireDamage != 0 {
				blast += elementalHit(def, rules.ResistKindFire, h.SpellHitChance(s, mon.Level), func() int { return s.FMinDam + rng.Intn(max(s.FMaxDam-s.FMinDam+1, 1)) })
			}
			if s.Flags&rules.FlagLightningDamage != 0 {
				blast += elementalHit(def, rules.ResistKindLightning, h.SpellHitChance(s, mon.Level), func() int { return s.LMinDam + rng.Intn(max(s.LMaxDam-s.LMinDam+1, 1)) })
			}
		}
		if ranged { // PM_DoRangeAttack wears the bow on every shot, a melee blow only when it lands
			itemBroke(ctx.ActionBroker, ctx.Msg, att, rules.WeaponDur(rng, att.equipped(), 40))
		}
		if rng.Intn(100) >= chance {
			logMissed(ctx, att, def)
			return blast, nil, true
		}
		if !ranged {
			itemBroke(ctx.ActionBroker, ctx.Msg, att, rules.WeaponDur(rng, att.equipped(), 30))
		}
		if ranged {
			dam = h.ArrowDamage(rng, s)
		} else {
			dam = h.MeleeDamage(rng, s, mon.Data.Class)
			if lhp, lmana := h.HitLeech(rng, s, dam); rules.Whole(lhp) > 0 || rules.Whole(lmana) > 0 {
				extra = append(extra,
					NewChangeResourceAction(NewActorCause("Life steal", att), att, stats.Health, float64(rules.Whole(lhp))),
					NewChangeResourceAction(NewActorCause("Mana steal", att), att, stats.Mana, float64(rules.Whole(lmana))))
			}
		}
		if ranged && s.Flags&(rules.FlagFireArrows|rules.FlagLightningArrows) != 0 { // MI_LArrow: the arrow bursts where it hits
			kind, mind, maxd := rules.ResistKindFire, s.FMinDam, s.FMaxDam
			if s.Flags&rules.FlagLightningArrows != 0 {
				kind, mind, maxd = rules.ResistKindLightning, s.LMinDam, s.LMaxDam
			}
			blast = elementalHit(def, kind, h.ArrowHitChance(s, mon.AC, dist), func() int { return h.ArrowBlastDamage(rng, s, mind, maxd) })
		}
		dam = hellfireHit(ctx, att, def, ranged, s, dam, &extra)
		dam = max(dam, 0) + blast
		// D1 stops a monster's healing on melee hits with the "can't heal" bit (0x100) and on hits by fire arrows;
		// the bit named for monsters (ISPL_NOHEALMON) is never read.
		if !ranged && s.Flags&rules.FlagNoHealPlayer != 0 || ranged && s.Flags&rules.FlagFireArrows != 0 {
			mon.NoHeal = true
		}
		if s.Flags&rules.FlagKnockback != 0 && !def.HasStatusEffect(StatusStone) && def.GetStats().GetResource(stats.Health).Value() > dam { // M_GetKnockback
			m := state.GetMap()
			push := def.MapPosition().Add(geometry.Dirs[DirTo(att.MapPosition(), def.MapPosition())])
			if _, isTransition := m.GetTransitionAt(push); m.IsCurrentlyPassable(push) && !isTransition {
				extra = append(extra, NewMoveAction(AbstractCause("Knockback"), def, push, false))
			}
		}
		return dam, extra, true
	case att.monster != nil && def.hero != nil:
		h, s, mon := def.hero, def.HeroSheet(), att.monster
		dlvl := state.CurrentLevel().Depth
		hit, minDam, maxDam := mon.Hit, mon.MinDam, mon.MaxDam
		if mon.HitWith != nil { // M_StartSpAttack and charges
			hit, minDam, maxDam = mon.HitWith[0], mon.HitWith[1], mon.HitWith[2]
			mon.HitWith = nil
		}
		chance := h.MonsterMeleeHitChance(s, hit, mon.Level, dlvl, mon.Data.Class)
		if ranged {
			chance = h.MonsterArrowHitChance(s, hit, mon.Level, dist, dlvl)
		}
		if rng.Intn(100) >= chance {
			logMissed(ctx, att, def)
			return 0, nil, true
		}
		// Hellfire's Reflect (M_TryH2HHit): a melee blow, blocked or not, turns 20-29% of its damage on the monster
		reflect := func(dam64 int) int {
			if ranged || h.Reflections <= 0 {
				return 0
			}
			h.Reflections--
			mdam := dam64 * (rng.Intn(10) + 20) / 100
			extra = append(extra, NewChangeResourceAction(NewActorCause("Reflect", def), att, stats.Health, -float64(mdam)/64))
			return mdam
		}
		if rng.Intn(100) < h.BlockChance(s, mon.Level) {
			ctx.Msg(fmt.Sprintf("You block the attack of %s.", att.Name()))
			playSfxAt(ctx, "IS_ISWORD", def.MapPosition()) // StartPlrBlock
			blockWear(ctx.ActionBroker, ctx.Msg, def)
			reflect(rules.MonsterDamage(rng, s, minDam, maxDam))
			return 0, extra, true
		}
		if mon.Data.Has("drains_max_life") && s.MaxHP > 64 && h.MaxHPBase > 64 { // Black Death: 1 max life, for good
			h.MaxHPBase -= 64
			h.HPBase = min(h.HPBase, h.MaxHPBase)
			SyncHero(def)
		}
		dam64 := rules.MonsterDamage(rng, s, minDam, maxDam)
		dam64 -= reflect(dam64)
		dam = manaShield(def, rules.Whole(max(dam64, 0)))
		StartPlrHit(state.factory, def, dam64, dam, false)
		if !ranged && mon.Data.Has("knockback") && def.GetStats().GetResource(stats.Health).Value() > dam {
			StartPlrHit(state.factory, def, 0, dam, true)
			m := state.GetMap()
			push := def.MapPosition().Add(geometry.Dirs[mon.Dir])
			if _, isTransition := m.GetTransitionAt(push); m.IsCurrentlyPassable(push) && !isTransition {
				extra = append(extra, NewMoveAction(AbstractCause("Knockback"), def, push, true))
			}
		}
		if !ranged && s.Flags&rules.FlagThorns != 0 {
			extra = append(extra, NewChangeResourceAction(NewActorCause("Thorns", def), att, stats.Health, -float64(rng.Intn(3)+1)))
		}
		if !ranged && s.ReturnDamage > 0 {
			extra = append(extra, NewChangeResourceAction(NewActorCause("Thorns", def), att, stats.Health, -float64(max(1, dam*s.ReturnDamage/100))))
		}
		return dam, extra, true
	case att.monster != nil && def.monster != nil: // the golem and the monsters fighting it: M_TryM2MHit
		mon := att.monster
		mon.HitWith = nil
		if !def.HasStatusEffect(StatusStone) && rng.Intn(100) >= mon.Hit {
			logMissed(ctx, att, def)
			return 0, nil, true
		}
		alert(def, att)
		return mon.MinDam + rng.Intn(max(mon.MaxDam-mon.MinDam+1, 1)), nil, true
	}
	return 0, nil, false
}

// ForecastSection mirrors resolveAttack for the right-click info window: both sides' chance to land a blow
// (the hero's block counted) and the fewest-most blows to kill at the current health, fewest with the best roll and
// every weapon blast landing, most with the worst roll. ok is false when the pair isn't a hero against a monster.
// ponytail: the monster's plain attack only, its special attacks and spell missiles roll their own numbers.
func ForecastSection(state *State, player, monster *Actor) (SheetSection, bool) {
	h, s, mon := player.hero, player.HeroSheet(), monster.monster
	if h == nil || mon == nil || mon.Golem {
		return SheetSection{}, false
	}
	blastMax := func(kind, dam int) int {
		if resist, immune := resistBits(kind); mon.MagicRes&immune != 0 {
			return 0
		} else if mon.MagicRes&resist != 0 {
			return max(dam, 0) >> 2
		}
		return max(dam, 0)
	}
	var chance, lo, hi int
	if s.WeaponType == rules.ITypeBow {
		chance = h.ArrowHitChance(s, mon.AC, geometry.DistanceChebyshev(player.MapPosition(), monster.MapPosition()))
		lo, hi = h.ArrowDamageRange(s, s.MinDam, s.MaxDam)
		if s.Flags&rules.FlagLightningArrows != 0 {
			_, burst := h.ArrowDamageRange(s, s.LMinDam, s.LMaxDam)
			hi += blastMax(rules.ResistKindLightning, burst)
		} else if s.Flags&rules.FlagFireArrows != 0 {
			_, burst := h.ArrowDamageRange(s, s.FMinDam, s.FMaxDam)
			hi += blastMax(rules.ResistKindFire, burst)
		}
	} else {
		chance = h.MeleeHitChance(s, mon.AC)
		lo, hi = h.MeleeDamageRange(s, mon.Data.Class)
		if s.Flags&rules.FlagFireDamage != 0 {
			hi += blastMax(rules.ResistKindFire, s.FMaxDam)
		}
		if s.Flags&rules.FlagLightningDamage != 0 {
			hi += blastMax(rules.ResistKindLightning, s.LMaxDam)
		}
	}
	if monster.HasStatusEffect(StatusStone) {
		chance = 100
	}
	st := player.GetStats()
	pool := st.GetResource(stats.Health).Value()
	if player.HasStatusEffect(StatusManaShield) { // the shield pays for the damage from mana first, a third off with the spell
		mana := st.GetResource(stats.Mana).Value()
		if shieldTakesThird(player) {
			mana = mana * 3 / 2
		}
		pool += mana
	}
	monChance := h.MonsterMeleeHitChance(s, mon.Hit, mon.Level, state.CurrentLevel().Depth, mon.Data.Class) * (100 - h.BlockChance(s, mon.Level)) / 100
	return SheetSection{Title: "Matchup", Rows: [][2]string{
		{"You", fmt.Sprintf("%d%% to hit, %s", chance, blowsToKill(monster.GetStats().GetResource(stats.Health).Value(), lo, hi))},
		{"It", fmt.Sprintf("%d%% to hit you, %s", monChance, blowsToKill(pool, max(mon.MinDam+s.GetHit, 1), max(mon.MaxDam+s.GetHit, 1)))},
	}, Notes: whyTheseNumbers(h, s, mon, geometry.DistanceChebyshev(player.MapPosition(), monster.MapPosition()))}, true
}

// monsterClassNames turns D1's monster classes into the words the rules are written in.
var monsterClassNames = map[string]string{"MC_UNDEAD": "the undead", "MC_ANIMAL": "animals", "MC_DEMON": "demons"}

// whyTheseNumbers names the rules behind the forecast above it, so the player can see why this monster is an
// easy or a bad matchup instead of having to guess: the weapon's kind, the affixes that only fire against this
// kind, and what distance is doing to a bow's accuracy.
func whyTheseNumbers(h *rules.Hero, s rules.Sheet, mon *Monster, dist int) []string {
	kind := monsterClassNames[mon.Data.Class]
	if kind == "" {
		return nil
	}
	var why []string
	if pct := h.Rules().WeaponVsMonsterClass(s, mon.Data.Class); pct != 0 {
		line := fmt.Sprintf("Your %s: %+d%% damage vs. %s", rules.WeaponTypeName(h.Rules().RuledWeapon(s)), pct, kind)
		for _, b := range h.Rules().WeaponBonuses { // the mod's rules name the weapon that would do better
			if b.Versus == mon.Data.Class && b.Weapon != h.Rules().RuledWeapon(s) && b.Percent > pct {
				line += fmt.Sprintf(" (a %s: %+d%%)", rules.WeaponTypeName(b.Weapon), b.Percent)
				break
			}
		}
		why = append(why, line)
	}
	if s.Flags&rules.FlagTripleDamageVsDemons != 0 && mon.Data.Class == "MC_DEMON" {
		why = append(why, "Your gear triples melee damage against demons")
	}
	if s.DamAcFlags&rules.FlagArmorVsUndead != 0 && mon.Data.Class == "MC_UNDEAD" {
		why = append(why, "Your armor is worth +20 AC against the undead")
	}
	if s.DamAcFlags&rules.FlagArmorVsDemons != 0 && mon.Data.Class == "MC_DEMON" {
		why = append(why, "Your armor is worth +40 AC against demons")
	}
	if crit := h.CriticalStrikeChance(); crit > 0 {
		why = append(why, fmt.Sprintf("%d%% of your melee swings hit twice", crit))
	}
	if s.WeaponType == rules.ITypeBow {
		closer := h.ArrowHitChance(s, mon.AC, max(dist-1, 1))
		why = append(why, fmt.Sprintf("Arrows lose accuracy with distance: %d tiles away, %d%% one step closer", dist, closer))
	}
	return why
}

// blowsToKill turns a damage range into the fewest-most landed blows that take hp.
func blowsToKill(hp, lo, hi int) string {
	switch {
	case hi <= 0:
		return "can't do damage"
	case lo <= 0:
		return fmt.Sprintf("%d+ hits to kill", (hp+hi-1)/hi)
	case (hp+hi-1)/hi == 1 && lo >= hp:
		return "1 hit to kill"
	case (hp+hi-1)/hi == (hp+lo-1)/lo:
		return fmt.Sprintf("%d hits to kill", (hp+lo-1)/lo)
	}
	return fmt.Sprintf("%d-%d hits to kill", (hp+hi-1)/hi, (hp+lo-1)/lo)
}

// elementalHit ports MonsterMHit for a fire or lightning blast: its own to-hit roll, nothing against immunity,
// a quarter against resistance.
func elementalHit(def *Actor, kind, chance int, roll func() int) int {
	mon := def.monster
	resist, immune := resistBits(kind)
	if mon.MagicRes&immune != 0 || !def.HasStatusEffect(StatusStone) && rng.Intn(100) >= chance {
		return 0
	}
	dam := max(roll(), 0)
	if mon.MagicRes&resist != 0 {
		dam >>= 2
	}
	return dam
}

// alert ports MonsterMHit's wake-up: a monster struck while idle (_msquelch 0) starts hunting from where the
// attacker stood.
func alert(victim, attacker *Actor) {
	if mon := victim.monster; mon.Squelch == 0 {
		mon.Squelch, mon.Last = 255, attacker.MapPosition()
	}
}

// monsterHitRecovery ports M_StartHit for any damage a monster takes. The hero flinches only where D1 calls
// StartPlrHit (StartPlrHit).
// monsterHitRecovery routes a blow to the flinch rules for whatever was hit. attacker is who dealt it, nil when the
// loss is self-inflicted (a spell's health cost, a status effect ticking): the hero flinches at blows, not at
// his own upkeep, so Blood Star must not stagger the caster who paid for it.
func monsterHitRecovery(f *Factory, victim *Actor, dam int, attacker *Actor) {
	if victim.monster != nil {
		monsterGotHit(f, victim, dam, attacker)
		return
	}
	if victim.hero != nil && attacker != nil && attacker != victim {
		StartPlrHit(f, victim, rules.Fixed(dam), dam, false)
	}
}

// StartPlrHit ports StartPlrHit and PM_DoGotHit: a blow of dam64 (in 64ths, before mana shield) of at least clvl
// (a Barbarian 1.25×) makes the hero flinch and lose time, and his armor wears 3 in 4 times. force is a charge's
// or knockback's flinch, which a hero already flinching doesn't get again. No flinch when dealt kills him. A new
// hit restarts the recovery instead of adding to it.
func StartPlrHit(f *Factory, a *Actor, dam64, dealt int, force bool) {
	h := a.hero
	if h == nil || a.GetStats().GetResource(stats.Health).Value() <= dealt ||
		force && a.hitRecovery > 0 || !force && rules.Whole(dam64) < h.Level+barbarianBonus(h) {
		return
	}
	if a.hitRecovery == 0 {
		f.Msg("You are staggered!")
		f.PlayHeroSound(69)
	}
	a.hitRecovery = max(a.hitRecovery, ticksToAUT(h.Rules().HitRecoveryTicks(h.Class, a.HeroSheet().Flags)))
	if rng.Intn(4) != 0 {
		itemBroke(f.ActionBroker, f.Msg, a, rules.ArmorDur(rng, a.equipped()))
	}
}

// barbarianBonus is what a Barbarian's StartPlrHit adds to the flinch threshold.
func barbarianBonus(h *rules.Hero) int {
	if h.Rules().HasClassFlag(h.Class, defs.ClassFlinchResist) {
		return h.Level / 4
	}
	return 0
}

// blockWear ports PM_DoBlock: a block wears the shield 1 in 10 times.
func blockWear(b *Broker, msg func(string), a *Actor) {
	if rng.Intn(10) == 0 {
		itemBroke(b, msg, a, rules.ShieldDur(a.equipped()))
	}
}

// ticksToAUT converts D1 ticks at a walk step (RefWalkTicks ticks = one 10-AUT turn).
func ticksToAUT(ticks int) int { return (ticks*10 + rules.RefWalkTicks/2) / rules.RefWalkTicks }

// MoveCost is a step's AUT: a monster's speed_move, anything else its speed factors (a D1 hero's step is
// RefWalkTicks).
func (a *Actor) MoveCost() int {
	cost := 100 / max(a.GetMoveSpeed(), 1)
	if a.monster != nil {
		cost = max(int(float64(a.monster.Data.SpeedMove)/a.stats.GetAttributeAsFloat(stats.MoveSpeedFactor)), 1)
	}
	if a.HasStatusEffect(StatusStoneSkin) {
		cost *= 2
	}
	return cost
}

// AttackCost is an attack's AUT. D1 heroes take their attack animation's ticks (decision 13), monsters their
// speed_attack over their weapon's speed, anything else its speed factors.
func (a *Actor) AttackCost(t AttackType) int {
	switch {
	case a.hero != nil && t == MeleeAttack:
		return ticksToAUT(a.HeroSheet().MeleeFrames())
	case a.hero != nil:
		return ticksToAUT(a.HeroSheet().AttackFrames)
	case a.monster != nil:
		cost := float64(a.monster.Data.SpeedAttack)
		if w := a.GetEquipment().GetWeapon(t); w != nil {
			cost /= w.GetWeaponStats().AttackSpeedFactor
		}
		return max(int(cost), 1)
	}
	return 100 / max(a.GetAttackSpeed(t), 1)
}

// SkillCost is a skill's AUT for this actor: a monster casts at its speed_attack (10 = the skill's own time).
func (a *Actor) SkillCost(aut int) int {
	if a.monster == nil {
		return aut
	}
	return max(aut*a.monster.Data.SpeedAttack/10, 1)
}

// MonsterRecoveryAUT is the time a hard hit costs a monster, its speed_recovery. D1's is its hit animation minus
// the part of its attack animation after the hit frame, because our monsters' blows land when their action ends
// and D1's at mAFNum. Without that, a monster as fast as the hero could never strike back once every hit is hard.
func MonsterRecoveryAUT(mon *Monster) int { return mon.Data.SpeedRecovery }

// monsterGotHit ports M_StartHit: AI side effects, and the monster loses MonsterRecoveryAUT of energy.
func monsterGotHit(f *Factory, victim *Actor, dam int, attacker *Actor) {
	mon := victim.monster
	if mon.Data.Has("flinches_at_any_hit") || dam >= mon.Level+3 {
		if attacker != nil {
			mon.Dir = DirTo(victim.MapPosition(), attacker.MapPosition())
		}
		if mon.Data.Has("blinks_when_hit") && attacker != nil {
			teleport(f.State.GetMap(), victim, attacker.MapPosition())
		} else if mon.Data.Has("turns_on_attacker") {
			mon.Goal = 1
		}
		if lost := MonsterRecoveryAUT(mon); lost > 0 && !victim.HasStatusEffect(StatusStone) && victim.GetEnergy() > -lost {
			victim.RemoveEnergy(victim.GetEnergy() + lost)
		}
		mon.LastMode, mon.StandTicks = ModeGotHit, 0
	}
}

// teleport ports M_Teleport (the Blink's escape when hit): a free cell diagonal to or beside the enemy, on a random
// side. D1's inner loop stops short (k < 1), so the row beyond the enemy is never tried; both coordinates must also
// differ from where the monster stands.
func teleport(m *gridmap.GridMap[*Actor, *Item, Object], victim *Actor, enemy geometry.Point) {
	if victim.HasStatusEffect(StatusStone) {
		return
	}
	pos := victim.MapPosition()
	rx, ry := 2*rng.Intn(2)-1, 2*rng.Intn(2)-1
	for j := -1; j <= 1; j++ {
		for k := -1; k < 1; k++ {
			p := geometry.Point{X: enemy.X + rx*j, Y: enemy.Y + ry*k}
			if j == 0 && k == 0 || p.X == pos.X || p.Y == pos.Y || !m.Contains(p) || !m.IsCurrentlyPassable(p) || m.IsObjectAt(p) && !m.IsWalkable(p) {
				continue
			}
			m.MoveActor(victim, p)
			victim.SetDrawPosition(p.ToPointF())
			victim.monster.Dir = DirTo(p, enemy)
			return
		}
	}
}

// fallenFear ports M_FallenFear: nearby fallen ones flee from a death, as far as their flee_distance.
func fallenFear(state *State, pos geometry.Point) {
	for _, a := range state.GetMap().Actors() {
		mon := a.monster
		if mon == nil || !mon.Brain(state.factory.Generator.D).Pack || !a.IsAlive() {
			continue
		}
		p := a.MapPosition()
		if rd := mon.Data.FleeDistance; rd != 0 && geometry.DistanceChebyshev(p, pos) < 5 {
			mon.Goal, mon.GoalVar1, mon.Dir = 2, rd, DirTo(pos, p)
		}
	}
}

// grantExperience adds experience earned against a foe of monLevel, and levels the hero up on it.
func (f *Factory) grantExperience(player *Actor, monLevel, exp int) {
	h := player.hero
	if !player.IsAlive() { // AddPlrExperience: a dead hero gets nothing
		return
	}
	before := h.Level
	h.AddExperience(monLevel, exp)
	SyncHero(player)
	if h.Level > before && h.Exp < h.Rules().ExpLvls[49] { // the jump to 50 at the cap heals nothing
		player.GetStats().GetResource(stats.Health).SetToMax()
		player.GetStats().GetResource(stats.Mana).SetToMax()
		msg := fmt.Sprintf("You reached level %d! Press 'u' to spend %d stat points.", h.Level, h.StatPts)
		if h.SkillPts > 0 {
			msg = fmt.Sprintf("You reached level %d! Press 'u' to spend %d stat and %d skill points.", h.Level, h.StatPts, h.SkillPts)
		}
		f.Msg(msg)
		f.RefreshSpells(player)
	}
}

// monsterKilled ports experience, fallen fear and SpawnItem for a dead D1 monster.
func monsterKilled(ctx ActionContext, killer, victim *Actor) []Action {
	state := ctx.GameState
	mon := victim.monster
	if killer.monster != nil && killer.monster.Golem {
		killer = state.GetPlayer() // the golem's kills are the player's
	}
	if mon.Golem {
		return nil
	}
	if killer.hero != nil {
		state.factory.grantExperience(killer, mon.Level, mon.Exp)
		if n := killer.HeroSheet().ManaOnKill; n > 0 {
			killer.GetStats().GetResource(stats.Mana).IncreaseByFloat(float64(n))
		}
	}
	questKill(ctx, mon)
	fallenFear(state, victim.MapPosition())
	state.factory.acidDeath(ctx, victim)
	state.dropsFrom = victim
	defer func() { state.dropsFrom = nil }()
	dlvl := state.CurrentLevel().Depth
	if it := state.factory.questDrop(mon); it != nil {
		state.AddItemToMapWithDistribution(state.factory.NewRolledItem(it), victim.MapPosition())
	} else if it := state.factory.Gen().MonsterDrop(mon.Data, mon.Level, mon.Unique != nil, state.ItemLevel(dlvl), state.factory.Levels.LevelType(dlvl) == rules.DTypeHell); it != nil {
		state.AddItemToMapWithDistribution(state.factory.NewRolledItem(it), victim.MapPosition())
	}
	return nil
}

// hellfireHit applies Hellfire's own damage effects to a landed blow (PlrHitMonst's _iDamAcFlags block):
// Devastation's rare triple hit, the Jester's wild multiplier, and Peril's double damage paid for in blood.
// Decay wears the weapon out one hit at a time and is handled in decayWeapons. A Doppelganger's melee blow splits
// an ordinary monster in two.
func hellfireHit(ctx ActionContext, att, def *Actor, ranged bool, s rules.Sheet, dam int, extra *[]Action) int {
	if s.DamAcFlags&rules.FlagTripleDamage != 0 && rng.Intn(100) < 5 {
		dam *= 3
	}
	if mon := def.monster; !ranged && s.DamAcFlags&rules.FlagDoppelganger != 0 && !mon.Brain(ctx.GameState.factory.Generator.D).ArchBoss && mon.Unique == nil && rng.Intn(100) < 10 {
		ctx.GameState.factory.doppelganger(def)
	}
	if s.DamAcFlags&rules.FlagJesters != 0 {
		dam = rules.JesterDamage(dam, rng.Intn)
	}
	if s.DamAcFlags&rules.FlagPeril != 0 {
		if self := s.GetHit; self > 0 {
			*extra = append(*extra, NewChangeResourceAction(NewActorCause("Peril", att), att, stats.Health, float64(-self)))
		}
		dam *= 2
	}
	decayWeapons(ctx, att)
	return dam
}

// decayWeapons ports WeaponDecay: a weapon "of decay" loses 5% damage every swing and crumbles at -100%.
func decayWeapons(ctx ActionContext, att *Actor) {
	for _, it := range att.GetEquipment().AllItems() {
		if it.roll == nil || it.roll.Class != "ICLASS_WEAPON" || it.roll.DamAcFlags&rules.FlagDecay == 0 {
			continue
		}
		it.roll.PLDam -= 5
		if it.roll.PLDam <= -100 {
			att.GetEquipment().UnEquip(it)
			att.GetInventory().Remove(it)
			ctx.Msg(fmt.Sprintf("Your %s crumbles away.", it.Name()))
			SyncHero(att)
			continue
		}
		it.Refresh()
	}
	SyncHero(att)
}
