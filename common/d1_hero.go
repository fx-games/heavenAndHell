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
}

// D1SheetAttribute is one of the four attributes on the character sheet: the base value stat points buy, the
// value with items on, the class maximum, and how to spend a point on it when one is left.
type D1SheetAttribute struct {
	Label    string
	Base     int
	Total    int
	Max      int
	CanRaise bool
	Raise    func()
}

// D1SheetSection is a titled block of label/value rows on the character sheet or in an info window. Notes are
// full-width prose lines under the rows, for what doesn't fit a label and a value (a monster's behavior, the
// rules behind a forecast).
type D1SheetSection struct {
	Title string
	Rows  [][2]string
	Notes []string
}

// D1SheetAttributes are the four attributes in D1's order, for the character sheet and for levelling up.
func (a *Actor) D1SheetAttributes() []D1SheetAttribute {
	h, s := a.hero, a.HeroSheet()
	if h == nil {
		return nil
	}
	base := [4]int{h.BaseStr, h.BaseMag, h.BaseDex, h.BaseVit}
	total := [4]int{s.Str, s.Mag, s.Dex, s.Vit}
	names := [4]stats.Name{stats.Strength, stats.Magic, stats.Dexterity, stats.Vitality}
	var out []D1SheetAttribute
	for i, label := range [4]string{"Strength", "Magic", "Dexterity", "Vitality"} {
		attr, name, maximum := i, names[i], d1.MaxStats[h.Class][i]
		out = append(out, D1SheetAttribute{Label: label, Base: base[attr], Total: total[attr], Max: maximum,
			CanRaise: h.StatPts > 0 && base[attr] < maximum,
			Raise:    func() { a.LevelUp(name) }})
	}
	return out
}

// D1SheetSections is everything on the character sheet apart from the four attributes.
func (a *Actor) D1SheetSections() []D1SheetSection {
	h, s, st := a.hero, a.HeroSheet(), a.GetStats()
	minD := s.MinDam + s.MinDam*s.BonusDam/100 + s.BonusDamMod + s.DamageMod
	maxD := s.MaxDam + s.MaxDam*s.BonusDam/100 + s.BonusDamMod + s.DamageMod
	return []D1SheetSection{
		{Title: "Character", Rows: [][2]string{
			{"Name", a.name},
			{"Class", d1.ClassNames[h.Class]},
			{"Level", fmt.Sprintf("%d", h.Level)},
			{"Experience", fmt.Sprintf("%d / %d", h.Exp, d1.ExpLvls[min(h.Level, 50)])},
			{"Gold", fmt.Sprintf("%d", st.GetAttribute(stats.Gold))},
		}},
		{Title: "Condition", Rows: [][2]string{
			{"Life", st.GetResource(stats.Health).ToString()},
			{"Mana", st.GetResource(stats.Mana).ToString()},
		}},
		{Title: "Combat", Rows: append([][2]string{
			{"Armor class", fmt.Sprintf("%d", s.AC+s.BonusAC+s.Dex/5)},
			{"Chance to block", fmt.Sprintf("%d%% vs. own level", h.BlockChance(s, h.Level))},
			{"Chance to hit", fmt.Sprintf("%d%%", h.MeleeHitChance(s, 0))},
			{"Damage", fmt.Sprintf("%d-%d", minD, maxD)},
			{"Bonus from Str/Dex", fmt.Sprintf("+%d", s.DamageMod)},
		}, a.d1DamageVsClassRows()...)},
		{Title: "Resistances", Rows: [][2]string{
			{"Fire", fmt.Sprintf("%d%%", s.FR)},
			{"Lightning", fmt.Sprintf("%d%%", s.LR)},
			{"Magic", fmt.Sprintf("%d%%", s.MR)},
		}},
		{Title: "Traits", Rows: a.d1TraitRows()},
	}
}

// D1ThreatSection says out loud that the dungeon got harder after Diablo, since nothing else announces it. It
// needs the world (the quest flags), so it is not part of the player's own D1SheetSections.
func (s *State) D1ThreatSection() D1SheetSection {
	threat := "Normal"
	if s.D1NewGamePlus() {
		threat = "Nightmare: Diablo is beaten"
	}
	return D1SheetSection{Title: "World", Rows: [][2]string{{"Threat", threat}}}
}

// d1DamageVsClassRows spells out the melee damage the weapon in hand does against each kind of monster, but only
// where the kind changes it: the sword/mace rule and the "+200% vs. demons" affix are otherwise invisible.
func (a *Actor) d1DamageVsClassRows() [][2]string {
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

// d1TraitRows are the class' own rules, which nothing else in the game says out loud.
func (a *Actor) d1TraitRows() [][2]string {
	h, s := a.hero, a.HeroSheet()
	rows := [][2]string{{"Missile damage bonus", "half of Str/Dex"}}
	if d1.FullMissileDamageBonus(h.Class) {
		rows[0][1] = "all of Str/Dex"
	}
	if crit := h.CriticalStrikeChance(); crit > 0 {
		rows = append(rows, [2]string{"Critical strike", fmt.Sprintf("%d%% of swings hit twice", crit)})
	}
	if d1.HasClassFlag(h.Class, d1.ClassTrapSense) {
		rows = append(rows, [2]string{"Trap sense", "sees traps on doors and chests"})
	}
	if d1.HasClassFlag(h.Class, d1.ClassIronSkin) {
		rows = append(rows, [2]string{"Iron skin", fmt.Sprintf("+%d armor class from level", h.Level/4)})
	}
	if d1.HasClassFlag(h.Class, d1.ClassNaturalResistance) {
		rows = append(rows, [2]string{"Natural resistance", fmt.Sprintf("+%d%% to every resistance", h.Level)})
	}
	if s.Flags&d1.ISplNoMana != 0 {
		rows = append(rows, [2]string{"No mana", "cannot cast spells that cost mana"})
	}
	if a.HasStatusEffect(StatusManaShield) {
		absorbed := "all damage paid from mana"
		if h.SpellLevel(s, d1.SplManaShield) > 0 {
			absorbed = "a third less damage, paid from mana"
		}
		rows = append(rows, [2]string{"Mana shield", absorbed})
	}
	return rows
}

// heroInfoText is the character sheet as plain lines, for the terminal UI and the stats server.
func (a *Actor) heroInfoText() []string {
	var lines []string
	for _, sec := range a.D1SheetSections() {
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
		for _, at := range a.D1SheetAttributes() {
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

// D1PotionWasted reports a potion the hero has no room for, and the message saying so. Drinking one is refused
// rather than silently throwing it away; elixirs always do something, so they are never wasted. A rejuvenation
// potion still counts as useful while either resource is short.
func D1PotionWasted(user *Actor, miscID string) (string, bool) {
	if user.hero == nil {
		return "", false
	}
	var health, mana bool
	switch miscID {
	case "IMISC_HEAL", "IMISC_FULLHEAL":
		health = true
	case "IMISC_MANA", "IMISC_FULLMANA":
		mana = true
	case "IMISC_REJUV", "IMISC_FULLREJUV":
		health, mana = true, true
	default:
		return "", false
	}
	st := user.GetStats()
	// a hero who cannot hold mana at all (the Warrior's no-mana gear) has no room for it either
	noMana := user.HeroSheet().Flags&d1.ISplNoMana != 0
	if health && !st.GetResource(stats.Health).IsAtMax() {
		return "", false
	}
	if mana && !noMana && !st.GetResource(stats.Mana).IsAtMax() {
		return "", false
	}
	switch {
	case health && mana:
		return "You are at full health and mana.", true
	case mana:
		return "You are at full mana.", true
	}
	return "You are at full health.", true
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
		blast := 0
		if !ranged { // PM_DoAttack: fire and lightning weapons explode on the swing's target, hit or miss (MI_Weapexp)
			if s.Flags&d1.ISplFireDam != 0 {
				blast += d1ElementalHit(def, d1.ResistKindFire, h.SpellHitChance(s, mon.Level), func() int { return s.FMinDam + d1Rng.Intn(max(s.FMaxDam-s.FMinDam+1, 1)) })
			}
			if s.Flags&d1.ISplLightDam != 0 {
				blast += d1ElementalHit(def, d1.ResistKindLightning, h.SpellHitChance(s, mon.Level), func() int { return s.LMinDam + d1Rng.Intn(max(s.LMaxDam-s.LMinDam+1, 1)) })
			}
		}
		if d1Rng.Intn(100) >= chance {
			logMissed(ctx, att, def)
			return blast, nil, true
		}
		durrnd := 30
		if ranged {
			durrnd = 40
		}
		d1Broke(ctx.Msg, att, d1.WeaponDur(d1Rng, att.equippedD1(), durrnd))
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
		if ranged && s.Flags&(d1.ISplFireArrows|d1.ISplLightArrows) != 0 { // MI_LArrow: the arrow bursts where it hits
			kind, mind, maxd := d1.ResistKindFire, s.FMinDam, s.FMaxDam
			if s.Flags&d1.ISplLightArrows != 0 {
				kind, mind, maxd = d1.ResistKindLightning, s.LMinDam, s.LMaxDam
			}
			blast = d1ElementalHit(def, kind, h.ArrowHitChance(s, mon.AC, dist), func() int { return h.ArrowBlastDamage(d1Rng, s, mind, maxd) })
		}
		dam = d1HellfireHit(ctx, att, def, ranged, s, dam, &extra)
		dam = max(dam, 0) + blast
		// D1 stops a monster's healing on melee hits with the "can't heal" bit (0x100) and on hits by fire arrows;
		// the bit named for monsters (ISPL_NOHEALMON) is never read.
		if !ranged && s.Flags&d1.ISplNoHealPlr != 0 || ranged && s.Flags&d1.ISplFireArrows != 0 {
			mon.NoHeal = true
		}
		if s.Flags&d1.ISplKnockback != 0 && !def.HasStatusEffect(StatusStone) && def.GetStats().GetResource(stats.Health).Value() > dam { // M_GetKnockback
			m := state.GetMap()
			push := def.MapPosition().Add(D1Dirs[D1DirTo(att.MapPosition(), def.MapPosition())])
			if _, isTransition := m.GetTransitionAt(push); m.IsCurrentlyPassable(push) && !isTransition {
				extra = append(extra, NewMoveAction(AbstractCause("Knockback"), def, push, false))
			}
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
		chance := h.MonsterMeleeHitChance(s, hit, mon.Level, dlvl, mon.Data.Class)
		if ranged {
			chance = h.MonsterArrowHitChance(s, hit, mon.Level, dist, dlvl)
		}
		if d1Rng.Intn(100) >= chance {
			logMissed(ctx, att, def)
			return 0, nil, true
		}
		// Hellfire's Reflect (M_TryH2HHit): a melee blow, blocked or not, turns 20-29% of its damage on the monster
		reflect := func(dam64 int) int {
			if ranged || h.Reflections <= 0 {
				return 0
			}
			h.Reflections--
			mdam := dam64 * (d1Rng.Intn(10) + 20) / 100
			extra = append(extra, NewChangeResourceAction(NewActorCause("Reflect", def), att, stats.Health, -float64(mdam)/64))
			return mdam
		}
		if d1Rng.Intn(100) < h.BlockChance(s, mon.Level) {
			ctx.Msg(fmt.Sprintf("You block the attack of %s.", att.Name()))
			playD1SfxAt(ctx, "IS_ISWORD", def.MapPosition()) // StartPlrBlock
			d1BlockWear(ctx.Msg, def)
			reflect(d1.MonsterDamage(d1Rng, s, minDam, maxDam))
			return 0, extra, true
		}
		dam64 := d1.MonsterDamage(d1Rng, s, minDam, maxDam)
		dam64 -= reflect(dam64)
		dam = d1ManaShield(def, max(dam64, 0)>>6)
		D1StartPlrHit(state.factory, def, dam64, dam, false)
		if !ranged && mon.Data.Flags&d1.MFlagKnockback != 0 && def.GetStats().GetResource(stats.Health).Value() > dam {
			D1StartPlrHit(state.factory, def, 0, dam, true)
			m := state.GetMap()
			push := def.MapPosition().Add(D1Dirs[mon.Dir])
			if _, isTransition := m.GetTransitionAt(push); m.IsCurrentlyPassable(push) && !isTransition {
				extra = append(extra, NewMoveAction(AbstractCause("Knockback"), def, push, true))
			}
		}
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

// D1ForecastSection mirrors resolveD1Attack for the right-click info window: both sides' chance to land a blow
// (the hero's block counted) and the fewest-most blows to kill at the current health, fewest with the best roll and
// every weapon blast landing, most with the worst roll. ok is false when the pair isn't a hero against a monster.
// ponytail: the monster's plain attack only, its special attacks and spell missiles roll their own numbers.
func D1ForecastSection(state *State, player, monster *Actor) (D1SheetSection, bool) {
	h, s, mon := player.hero, player.HeroSheet(), monster.d1Monster
	if h == nil || mon == nil || mon.Golem {
		return D1SheetSection{}, false
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
	if s.WeaponType == d1.ITypeBow {
		chance = h.ArrowHitChance(s, mon.AC, geometry.DistanceChebyshev(player.MapPosition(), monster.MapPosition()))
		lo, hi = h.ArrowDamageRange(s, s.MinDam, s.MaxDam)
		if s.Flags&d1.ISplLightArrows != 0 {
			_, burst := h.ArrowDamageRange(s, s.LMinDam, s.LMaxDam)
			hi += blastMax(d1.ResistKindLightning, burst)
		} else if s.Flags&d1.ISplFireArrows != 0 {
			_, burst := h.ArrowDamageRange(s, s.FMinDam, s.FMaxDam)
			hi += blastMax(d1.ResistKindFire, burst)
		}
	} else {
		chance = h.MeleeHitChance(s, mon.AC)
		lo, hi = h.MeleeDamageRange(s, mon.Data.Class)
		if s.Flags&d1.ISplFireDam != 0 {
			hi += blastMax(d1.ResistKindFire, s.FMaxDam)
		}
		if s.Flags&d1.ISplLightDam != 0 {
			hi += blastMax(d1.ResistKindLightning, s.LMaxDam)
		}
	}
	if monster.HasStatusEffect(StatusStone) {
		chance = 100
	}
	st := player.GetStats()
	pool := st.GetResource(stats.Health).Value()
	if player.HasStatusEffect(StatusManaShield) { // the shield pays for the damage from mana first, a third off with the spell
		mana := st.GetResource(stats.Mana).Value()
		if h.SpellLevel(s, d1.SplManaShield) > 0 {
			mana = mana * 3 / 2
		}
		pool += mana
	}
	monChance := h.MonsterMeleeHitChance(s, mon.Hit, mon.Level, d1DungeonLevel(state), mon.Data.Class) * (100 - h.BlockChance(s, mon.Level)) / 100
	return D1SheetSection{Title: "Matchup", Rows: [][2]string{
		{"You", fmt.Sprintf("%d%% to hit, %s", chance, d1BlowsToKill(monster.GetStats().GetResource(stats.Health).Value(), lo, hi))},
		{"It", fmt.Sprintf("%d%% to hit you, %s", monChance, d1BlowsToKill(pool, max(mon.MinDam+s.GetHit, 1), max(mon.MaxDam+s.GetHit, 1)))},
	}, Notes: d1WhyTheseNumbers(h, s, mon, geometry.DistanceChebyshev(player.MapPosition(), monster.MapPosition()))}, true
}

// d1MonsterClassNames turns D1's monster classes into the words the rules are written in.
var d1MonsterClassNames = map[string]string{"MC_UNDEAD": "the undead", "MC_ANIMAL": "animals", "MC_DEMON": "demons"}

// d1WhyTheseNumbers names the rules behind the forecast above it, so the player can see why this monster is an
// easy or a bad matchup instead of having to guess: the weapon's kind, the affixes that only fire against this
// kind, and what distance is doing to a bow's accuracy.
func d1WhyTheseNumbers(h *d1.Hero, s d1.Sheet, mon *D1Monster, dist int) []string {
	kind := d1MonsterClassNames[mon.Data.Class]
	if kind == "" {
		return nil
	}
	var why []string
	if pct := s.WeaponVsMonsterClass(mon.Data.Class); pct != 0 {
		other := "a mace would do the opposite"
		if pct < 0 {
			other = "a mace in hand would turn this around"
			if !s.Holds(d1.ITypeSword) {
				other = "a sword in hand would turn this around"
			}
		}
		why = append(why, fmt.Sprintf("Your weapon: %+d%% damage vs. %s (%s)", pct, kind, other))
	}
	if s.Flags&d1.ISpl3xDamVsDemons != 0 && mon.Data.Class == "MC_DEMON" {
		why = append(why, "Your gear triples melee damage against demons")
	}
	if s.DamAcFlags&d1.IHfACVsUndead != 0 && mon.Data.Class == "MC_UNDEAD" {
		why = append(why, "Your armor is worth +20 AC against the undead")
	}
	if s.DamAcFlags&d1.IHfACVsDemons != 0 && mon.Data.Class == "MC_DEMON" {
		why = append(why, "Your armor is worth +40 AC against demons")
	}
	if crit := h.CriticalStrikeChance(); crit > 0 {
		why = append(why, fmt.Sprintf("%d%% of your melee swings hit twice", crit))
	}
	if s.WeaponType == d1.ITypeBow {
		closer := h.ArrowHitChance(s, mon.AC, max(dist-1, 1))
		why = append(why, fmt.Sprintf("Arrows lose accuracy with distance: %d tiles away, %d%% one step closer", dist, closer))
	}
	return why
}

// d1BlowsToKill turns a damage range into the fewest-most landed blows that take hp.
func d1BlowsToKill(hp, lo, hi int) string {
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

// d1ElementalHit ports MonsterMHit for a fire or lightning blast: its own to-hit roll, nothing against immunity,
// a quarter against resistance.
func d1ElementalHit(def *Actor, kind, chance int, roll func() int) int {
	mon := def.d1Monster
	resist, immune := resistBits(kind)
	if mon.MagicRes&immune != 0 || !def.HasStatusEffect(StatusStone) && d1Rng.Intn(100) >= chance {
		return 0
	}
	dam := max(roll(), 0)
	if mon.MagicRes&resist != 0 {
		dam >>= 2
	}
	return dam
}

// Fight or flight, rolled every time the player attacks. Hellspawn don't scare easily: only one already close
// to death breaks, uniques never do, and the run is short. Knobs, not laws - tune them for how the game plays.
const (
	d1FleeBelowHealth = 0.2
	d1FleeChance      = 40
	d1FleeTurns       = 6
)

// d1Alert wakes a monster the player attacks (MonsterMHit) and makes it choose fight or flight. Attacked is
// enough: a monster that has been swung at fights back rather than idling or wandering off, which in a
// turn-based game is the difference between D1's AI and a punching bag. The health it rolls against is the
// health before this blow lands, so a monster breaks one hit later than it would - close enough for a threshold.
func d1Alert(victim, attacker *Actor) {
	mon := victim.d1Monster
	mon.Squelch, mon.Last, mon.Enraged = 255, attacker.MapPosition(), true
	if mon.Flee == 0 && mon.Unique == nil &&
		victim.GetStats().GetResource(stats.Health).IsLessThanPercent(d1FleeBelowHealth) &&
		d1Rng.Intn(100) < d1FleeChance {
		mon.Flee = d1FleeTurns
	}
}

// d1HitRecovery ports M_StartHit for any damage a monster takes. The hero flinches only where D1 calls
// StartPlrHit (D1StartPlrHit).
func d1HitRecovery(victim *Actor, dam int) {
	if victim.d1Monster != nil {
		d1MonsterGotHit(victim, dam)
	}
}

// D1StartPlrHit ports StartPlrHit and PM_DoGotHit: a blow of dam64 (in 64ths, before mana shield) of at least clvl
// (a Barbarian 1.25×) makes the hero flinch and lose time, and his armor wears 3 in 4 times. force is a charge's
// or knockback's flinch, which a hero already flinching doesn't get again. No flinch when dealt kills him. A new
// hit restarts the recovery instead of adding to it.
func D1StartPlrHit(f *Factory, a *Actor, dam64, dealt int, force bool) {
	h := a.hero
	if h == nil || a.GetStats().GetResource(stats.Health).Value() <= dealt ||
		force && a.hitRecovery > 0 || !force && dam64>>6 < h.Level+d1BarbarianBonus(h) {
		return
	}
	if a.hitRecovery == 0 {
		f.Msg("You are staggered!")
		f.PlayD1HeroSound(69)
	}
	a.hitRecovery = max(a.hitRecovery, d1TicksToAUT(d1.HitRecoveryTicks(h.Class, a.HeroSheet().Flags)))
	if d1Rng.Intn(4) != 0 {
		d1Broke(f.Msg, a, d1.ArmorDur(d1Rng, a.equippedD1()))
	}
}

// d1BarbarianBonus is what a Barbarian's StartPlrHit adds to the flinch threshold.
func d1BarbarianBonus(h *d1.Hero) int {
	if h.Class == d1.Barbarian {
		return h.Level / 4
	}
	return 0
}

// d1BlockWear ports PM_DoBlock: a block wears the shield 1 in 10 times.
func d1BlockWear(msg func(string), a *Actor) {
	if d1Rng.Intn(10) == 0 {
		d1Broke(msg, a, d1.ShieldDur(a.equippedD1()))
	}
}

// d1TicksToAUT converts D1 ticks at a walk step (RefWalkTicks ticks = one 10-AUT turn).
func d1TicksToAUT(ticks int) int { return (ticks*10 + d1.RefWalkTicks/2) / d1.RefWalkTicks }

// MoveCost is a step's AUT: a D1 monster's walk animation, anything else its speed factors (a D1 hero's step is
// RefWalkTicks).
func (a *Actor) MoveCost() int {
	if a.d1Monster != nil {
		d := a.d1Monster.Data
		return d1TicksToAUT(d.Frames[1] * (d.Rate[1] + 1))
	}
	return 100 / a.GetMoveSpeed()
}

// AttackCost is an attack's AUT. D1 heroes and monsters take their attack animation's ticks (decision 13),
// anything else its speed factors.
func (a *Actor) AttackCost(t AttackType) int {
	switch {
	case a.hero != nil && t == MeleeAttack:
		return d1TicksToAUT(a.HeroSheet().MeleeFrames())
	case a.hero != nil:
		return d1TicksToAUT(a.HeroSheet().AttackFrames)
	case a.d1Monster != nil:
		d := a.d1Monster.Data
		return d1TicksToAUT(d.Frames[2] * (d.Rate[2] + 1))
	}
	return 100 / a.GetAttackSpeed(t)
}

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
			ctx.Msg(fmt.Sprintf("You reached level %d! Press 'u' to spend %d stat points.", h.Level, h.StatPts))
			state.factory.RefreshD1Spells(killer)
		}
	}
	whenVisible(ctx.ActionBroker, victim, func() { playCueAt(ctx, victim.D1MonsterSoundCue("death"), victim.MapPosition()) })
	d1QuestKill(ctx, mon)
	d1FallenFear(state, victim.MapPosition())
	state.factory.d1AcidDeath(ctx, victim)
	state.dropsFrom = victim
	defer func() { state.dropsFrom = nil }()
	dlvl := d1DungeonLevel(state)
	if it := state.factory.d1QuestDrop(mon); it != nil {
		state.AddItemToMapWithDistribution(state.factory.NewD1Item(it), victim.MapPosition())
	} else if it := state.factory.D1().MonsterDrop(mon.Data, mon.Level, mon.Unique != nil, state.D1ItemLevel(dlvl), state.factory.D1().D.LevelType(dlvl) == d1.DTypeHell); it != nil {
		state.AddItemToMapWithDistribution(state.factory.NewD1Item(it), victim.MapPosition())
	}
	return nil
}

// d1HellfireHit applies Hellfire's own damage effects to a landed blow (PlrHitMonst's _iDamAcFlags block):
// Devastation's rare triple hit, the Jester's wild multiplier, and Peril's double damage paid for in blood.
// Decay wears the weapon out one hit at a time and is handled in d1DecayWeapons. A Doppelganger's melee blow splits
// an ordinary monster in two.
func d1HellfireHit(ctx ActionContext, att, def *Actor, ranged bool, s d1.Sheet, dam int, extra *[]Action) int {
	if s.DamAcFlags&d1.IHfDevastation != 0 && d1Rng.Intn(100) < 5 {
		dam *= 3
	}
	if mon := def.d1Monster; !ranged && s.DamAcFlags&d1.IHfDoppelganger != 0 && mon.AI != "AI_DIABLO" && mon.Unique == nil && d1Rng.Intn(100) < 10 {
		ctx.GameState.factory.d1Doppelganger(def)
	}
	if s.DamAcFlags&d1.IHfJesters != 0 {
		dam = d1.JesterDamage(dam, d1Rng.Intn)
	}
	if s.DamAcFlags&d1.IHfPeril != 0 {
		if self := s.GetHit; self > 0 {
			*extra = append(*extra, NewChangeResourceAction(NewActorCause("Peril", att), att, stats.Health, float64(-self)))
		}
		dam *= 2
	}
	d1DecayWeapons(ctx, att)
	return dam
}

// d1DecayWeapons ports WeaponDecay: a weapon "of decay" loses 5% damage every swing and crumbles at -100%.
func d1DecayWeapons(ctx ActionContext, att *Actor) {
	for _, it := range att.GetEquipment().AllItems() {
		if it.d1 == nil || it.d1.Class != "ICLASS_WEAPON" || it.d1.DamAcFlags&d1.IHfDecay == 0 {
			continue
		}
		it.d1.PLDam -= 5
		if it.d1.PLDam <= -100 {
			att.GetEquipment().UnEquip(it)
			att.GetInventory().Remove(it)
			ctx.Msg(fmt.Sprintf("Your %s crumbles away.", it.Name()))
			SyncHero(att)
			continue
		}
		it.RefreshD1()
	}
	SyncHero(att)
}
