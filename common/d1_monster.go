package common

import (
	"fmt"
	"image/color"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/stats"
	"strings"
)

// D1Monster holds the MonsterStruct fields D1's AI and combat need.
type D1Monster struct {
	Type                   int
	Data                   *d1.MonsterData
	Unique                 *d1.UniqueMonsterData
	Level, Exp, Int        int
	Hit, MinDam, MaxDam    int
	Hit2, MinDam2, MaxDam2 int
	AC, MagicRes           int
	AI                     string
	TalkMsg                string
	D1MonsterState
	Leader     *Actor
	SpawnIndex int     // order of creation on its level, identifies the monster in saves
	Golem      bool    // the player's golem (AI_GOLUM), fights for the player
	HitWith    *[3]int // set for one melee attack: to-hit, min and max damage of a special attack
}

// D1MonsterState is the AI state a save keeps for each monster.
type D1MonsterState struct {
	Goal, GoalVar1, GoalVar2, GoalVar3 int
	LastMode, StandTicks               int // _mVar1 (previous mode), _mVar2 (ticks spent standing)
	Squelch, PathCount                 int
	Last                               geometry.Point
	Dir                                int  // index into D1Dirs
	Enraged                            bool // the player attacked it: it fights back instead of idling or wandering off
	Flee                               int  // turns left of a fight-or-flight retreat
	Dormant                            bool // _mFlags & 4 on gargoyles: waiting in stone until the enemy comes close
	NoHeal                             bool // _mFlags & 8 (MFLAG_NOHEAL): hit by a weapon that stops healing
	Berserk                            bool // Hellfire's MFLAG_BERSERK | MFLAG_GOLEM: turned on its own kind
}

// D1 monster modes (MON_MODE) the AI compares against.
const (
	D1ModeStand     = 0
	D1ModeWalk      = 1
	D1ModeAttack    = 4
	D1ModeGotHit    = 5
	D1ModeSAttack   = 7
	D1ModeFadeIn    = 8
	D1ModeFadeOut   = 9
	D1ModeRAttack   = 10
	D1ModeSpStand   = 11
	D1ModeRSpAttack = 12
	D1ModeDelay     = 13
	D1ModeCharge    = 14
	D1ModeHeal      = 16
)

// D1Dirs lists the 8 directions in circular order, so d-1/d+1 are D1's left[]/right[] and d+4 is opposite[].
var D1Dirs = [8]geometry.Point{{X: 1, Y: 1}, {X: 0, Y: 1}, {X: -1, Y: 1}, {X: -1, Y: 0}, {X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 0}}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// D1DirTo ports GetDirection.
func D1DirTo(from, to geometry.Point) int {
	d := geometry.Point{X: sign(to.X - from.X), Y: sign(to.Y - from.Y)}
	for i, v := range D1Dirs {
		if v == d {
			return i
		}
	}
	return 0
}

func (a *Actor) D1Monster() *D1Monster { return a.d1Monster }

// D1MonsterSoundCue builds a monster-specific sound event (e.g. "monster.nzombie.attack", from
// assets/data/sounds.rec) for category "attack"/"hit"/"death". Empty if this actor isn't a D1 monster.
func (a *Actor) D1MonsterSoundCue(category string) string {
	if a.d1Monster == nil || a.d1Monster.Data == nil {
		return ""
	}
	return defs.IconID("monster", a.d1Monster.Data.ID) + "." + category
}

// Decision 8: one sheet icon per D1 monster type ("monster.<ID>" in icons/actors.rec, seeded from its graphics
// family), variants by tint. Monster variants are the only tinted tiles; everything else gets its own icon.
var d1VariantTints = []color.RGBA{{255, 255, 255, 255}, {160, 200, 255, 255}, {255, 190, 120, 255}, {255, 110, 110, 255}}

// NewD1Monster ports InitMonster for monster type mt.
func (g *Factory) NewD1Monster(mt int) *Actor {
	return g.newD1Monster(mt, nil)
}

// newD1Monster is NewD1Monster's shared core. Given um, it builds the actor as that unique right away, with the
// unique's own "unique.<name>" icon and untinted color from the start, instead of building a plain monster and
// overriding its icon/color/name afterward.
func (g *Factory) newD1Monster(mt int, um *d1.UniqueMonsterData) *Actor {
	d := g.Generator.D
	m := &d.Monsters[mt]
	name, icon, tint := m.Name, defs.IconID("monster", m.ID), g.d1VariantTint(mt, m)
	if um != nil {
		name, icon, tint = um.Name, defs.IconID("unique", um.Name), defs.White
	}
	a := NewActor(defs.Icon(icon))
	a.SetName(name)
	a.SetInternalName(m.ID)
	a.SetHostile()
	a.SetAsciiIcon(rune(strings.ToLower(name)[0]))
	a.SetColor(tint)
	if um != nil {
		a.SetUnique()
	}
	a.d1Monster = &D1Monster{Type: mt, Data: m, Unique: um, Level: m.Level, Exp: m.Exp, Int: m.Int, Hit: m.Hit, MinDam: m.MinDam,
		MaxDam: m.MaxDam, Hit2: m.Hit2, MinDam2: m.MinDam2, MaxDam2: m.MaxDam2, AC: m.AC, MagicRes: m.MagicRes, AI: m.AI, D1MonsterState: D1MonsterState{Goal: 1}}
	st := a.GetStats()
	st.SetAttribute(stats.MaxHealth, g.Generator.MonsterHP(m))
	st.GetResource(stats.Health).SetToMax()
	st.SetAttribute(stats.Level, m.Level)
	st.SetAttribute(stats.Experience, m.Exp)
	st.SetAttributeAsFloat(stats.MoveSpeedFactor, d1.SpeedTier(d1.RefWalkTicks, m.Frames[1]))
	st.SetAttributeAsFloat(stats.AttackSpeedFactor, d1.SpeedTier(d1.RefAttackTicks, m.Frames[2]))
	a.d1Monster.Dormant = m.AI == "AI_GARG"
	if m.Flags&d1.MFlagHidden != 0 {
		a.AddStatusEffect(StatusInvisible, -1)
	}
	return a
}

// d1VariantTint is InitMonster's tint pick for monster type mt: the Nth color in d1VariantTints, N being how
// many earlier types share its graphics family.
func (g *Factory) d1VariantTint(mt int, m *d1.MonsterData) color.RGBA {
	variant := 0
	for i := 0; i < mt; i++ {
		if g.Generator.D.Monsters[i].Family == m.Family {
			variant++
		}
	}
	return d1VariantTints[variant%len(d1VariantTints)]
}

// NewD1UniqueMonster ports PlaceUniqueMonst's stat changes for UniqMonst[u].
func (g *Factory) NewD1UniqueMonster(u int) *Actor {
	um := &g.Generator.D.UniqueMonsters[u]
	a := g.newD1Monster(g.Generator.D.MonsterByID(um.Type), um)
	mon := a.d1Monster
	if um.Level != 0 {
		mon.Level = 2 * um.Level
	} else {
		mon.Level += 5
	}
	mon.Exp *= 2
	mon.AI, mon.Int, mon.MagicRes = um.AI, um.Int, um.MagicRes
	mon.MinDam, mon.MaxDam, mon.MinDam2, mon.MaxDam2 = um.MinDam, um.MaxDam, um.MinDam, um.MaxDam
	if um.UnqAttr&4 != 0 {
		mon.Hit, mon.Hit2 = um.UnqVar1, um.UnqVar1
	}
	if um.UnqAttr&8 != 0 {
		mon.AC = um.UnqVar1
	}
	st := a.GetStats()
	st.SetAttribute(stats.MaxHealth, max(um.MaxHP>>1, 1))
	st.GetResource(stats.Health).SetToMax()
	st.SetAttribute(stats.Level, mon.Level)
	st.SetAttribute(stats.Experience, mon.Exp)
	return a
}

// NewD1MonsterFromLevel builds an Actor from a monster the exact-RNG level generator already placed (d1.Level's
// Monsters), using its precomputed HP/AI/damage/resist instead of rerolling them: mon.HP is 1/64 units like D1's
// _mmaxhp, so >>6 gives whole hit points.
func (g *Factory) NewD1MonsterFromLevel(mon *d1.Monster) *Actor {
	var um *d1.UniqueMonsterData
	if mon.Unique >= 0 {
		um = &g.Generator.D.UniqueMonsters[mon.Unique]
	}
	a := g.newD1Monster(mon.Type, um)
	dm := a.d1Monster
	dm.AI, dm.Int, dm.MinDam, dm.MaxDam, dm.MagicRes, dm.Level = mon.AI, mon.Int, mon.MinDam, mon.MaxDam, mon.MagicRes, mon.Level
	if um != nil {
		dm.TalkMsg = um.TalkMsg
	}
	dm.Dir = mon.Dir
	hp := max(mon.HP>>6, 1)
	st := a.GetStats()
	st.SetAttribute(stats.MaxHealth, hp)
	st.GetResource(stats.Health).SetToMax()
	st.SetAttribute(stats.Level, dm.Level)
	return a
}

// MakeD1Minion ports PlaceGroup's leader handling (leaderf bit 1: stronger, bit 2: follows the leader's AI).
func MakeD1Minion(minion, leader *Actor, leaderf int) {
	if leaderf&1 == 0 {
		return
	}
	mon, lead := minion.d1Monster, leader.d1Monster
	hp := minion.GetStats()
	hp.SetAttribute(stats.MaxHealth, hp.GetAttribute(stats.MaxHealth)*2)
	hp.GetResource(stats.Health).SetToMax()
	mon.Int = lead.Int
	if leaderf&2 != 0 {
		mon.Leader = leader
		mon.AI = lead.AI
	}
}

func (a *Actor) d1MonsterInfoText() []string {
	mon := a.d1Monster
	class := strings.ToLower(strings.TrimPrefix(mon.Data.Class, "MC_"))
	title := fmt.Sprintf("Level %d %s", mon.Level, class)
	if mon.Unique != nil {
		title = "Unique " + title
	}
	lines := []string{a.name, title, "",
		fmt.Sprintf("HP: %s", a.GetStats().GetResource(stats.Health).ToString()),
		fmt.Sprintf("Armor class: %d", mon.AC),
		fmt.Sprintf("Damage: %d-%d", mon.MinDam, mon.MaxDam),
	}
	if mon.MaxDam2 > 0 && (mon.MinDam2 != mon.MinDam || mon.MaxDam2 != mon.MaxDam) {
		lines = append(lines, fmt.Sprintf("Secondary attack: %d-%d", mon.MinDam2, mon.MaxDam2))
	}
	for _, r := range []struct {
		bit  int
		text string
	}{{d1.ResistMagic, "Resists magic"}, {d1.ResistFire, "Resists fire"}, {d1.ResistLightning, "Resists lightning"},
		{d1.ImmuneMagic, "Immune to magic"}, {d1.ImmuneFire, "Immune to fire"}, {d1.ImmuneLightning, "Immune to lightning"}} {
		if mon.MagicRes&r.bit != 0 {
			lines = append(lines, r.text)
		}
	}
	if behavior := d1MonsterAIDescriptions[mon.AI]; behavior != "" {
		lines = append(lines, "", behavior)
	}
	return lines
}

// d1MonsterAIDescriptions is a plain-English gloss of each AI_* brain in game/ai_d1.go's think(), kept here
// (common can't import game, which imports common) so the actor description popup can show it. Not code-shared
// with the AI itself: if a brain's behavior changes, update its line here too.
var d1MonsterAIDescriptions = map[string]string{
	"AI_ZOMBIE": "Shambles slowly toward you.", "AI_FALLEN": "Weak alone but rallies in packs; flees and may mutate when panicked.",
	"AI_SCAV": "Feeds on nearby corpses to heal itself.", "AI_SKELBOW": "Keeps its distance and fires arrows.",
	"AI_GOATBOW": "Keeps its distance and fires arrows.", "AI_SNEAK": "Lurks at the edge of sight and strikes from stealth.",
	"AI_GOATMC": "Charges in for melee.", "AI_BAT": "Flies erratically.",
	"AI_SUCC": "Keeps its distance, hurling magic bolts.", "AI_ACIDUNIQ": "Keeps its distance, spitting acid.",
	"AI_LICH": "Keeps its distance, hurling magic bolts.", "AI_ARCHLICH": "Keeps its distance, hurling magic bolts.",
	"AI_PSYCHORB": "Keeps its distance, hurling magic bolts.", "AI_NECROMORB": "Keeps its distance, hurling magic bolts.",
	"AI_FIREBAT": "Flies at range, firing bolts of fire.", "AI_HELLBAT": "Flies at range, hurling fireballs.",
	"AI_HORKDEMON": "Fights in melee.", "AI_BONED": "Circles at range, launching bone-spirit bolts with an occasional special attack.",
	"AI_MAGMA": "Circles at range, hurling magma balls.", "AI_STORM": "Circles at range, casting lightning.",
	"AI_ACID": "Circles at range, spitting corrosive acid.", "AI_MEGA": "Circles at range, breathing gouts of flame.",
	"AI_GARG": "Lies dormant until approached, then takes flight to attack.", "AI_RHINO": "Charges in a straight line.",
	"AI_COUNSLR": "Keeps its distance, casting spells.", "AI_CLEAVER": "Waits until provoked, then charges relentlessly.",
	"AI_SKELKING": "Occasionally raises a skeleton from the ground; otherwise fights in melee.",
	"AI_ZHAR": "Speaks first, then fights as a spellcaster.", "AI_LAZURUS": "Speaks first, then fights as a spellcaster.",
	"AI_LAZHELP": "Joins the fight as a ranged caster once Lazarus has spoken.",
	"AI_WARLORD": "Speaks first, then attacks with a blade.", "AI_GARBUD": "Speaks first, then charges in for melee.",
	"AI_SNOTSPIL": "Speaks first, then fights like a Fallen One.", "AI_LACHDAN": "Never fights.",
	"AI_DIABLO": "Unleashes devastating Apocalypse spells from a distance.",
}

func (a *Actor) RemoveStatusEffect(effect StatusEffect) {
	a.statusEffects.SetFalse(string(effect))
}
