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
	return g.scaleD1NewGamePlus(g.newD1Monster(mt, nil))
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
	a := g.scaleD1NewGamePlus(g.newD1Monster(g.Generator.D.MonsterByID(um.Type), um))
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
	return g.scaleD1NewGamePlus(a)
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

// D1MonsterSections describes a monster for the info window: what it is, its numbers, what it shrugs off and
// how it fights.
func (a *Actor) D1MonsterSections() []D1SheetSection {
	mon := a.d1Monster
	class := strings.ToLower(strings.TrimPrefix(mon.Data.Class, "MC_"))
	kind := fmt.Sprintf("Level %d %s", mon.Level, class)
	if mon.Unique != nil {
		kind = "Unique " + kind
	}
	rows := [][2]string{
		{"Kind", kind},
		{"Life", a.GetStats().GetResource(stats.Health).ToString()},
		{"Armor class", fmt.Sprintf("%d", mon.AC)},
		{"Damage", fmt.Sprintf("%d-%d", mon.MinDam, mon.MaxDam)},
	}
	if mon.MaxDam2 > 0 && (mon.MinDam2 != mon.MinDam || mon.MaxDam2 != mon.MaxDam) {
		rows = append(rows, [2]string{"Secondary attack", fmt.Sprintf("%d-%d", mon.MinDam2, mon.MaxDam2)})
	}
	sections := []D1SheetSection{{Title: "Creature", Rows: rows}}
	var resists [][2]string
	for _, r := range []struct {
		element      string
		resist, immn int
	}{{"Fire", d1.ResistFire, d1.ImmuneFire}, {"Lightning", d1.ResistLightning, d1.ImmuneLightning},
		{"Magic", d1.ResistMagic, d1.ImmuneMagic}} {
		switch {
		case mon.MagicRes&r.immn != 0:
			resists = append(resists, [2]string{r.element, "immune"})
		case mon.MagicRes&r.resist != 0:
			resists = append(resists, [2]string{r.element, "75%"}) // every damage path quarters a resisted hit (dam >>= 2)
		}
	}
	if len(resists) > 0 {
		sections = append(sections, D1SheetSection{Title: "Resistances", Rows: resists})
	}
	if behavior := d1.MonsterAIDescriptions[mon.AI]; behavior != "" {
		sections = append(sections, D1SheetSection{Title: "Behavior", Notes: []string{behavior}})
	}
	return sections
}

func (a *Actor) RemoveStatusEffect(effect StatusEffect) {
	a.statusEffects.SetFalse(string(effect))
}

// D1NewGamePlus is on once Diablo has been beaten: monsters that spawn and loot that drops from then on use D1's
// own Nightmare numbers, so the dungeon Gillian's donation refills is a match for a hero who already cleared it.
func (s *State) D1NewGamePlus() bool { return s.QuestActive(d1.QDiablo) == d1.QuestDone }

// D1ItemLevel is the level loot rolls at on dungeon level dlvl.
func (s *State) D1ItemLevel(dlvl int) int {
	if s.D1NewGamePlus() {
		return dlvl + d1.NightmareItemLevelBonus
	}
	return dlvl
}

// scaleD1NewGamePlus ports InitMonster's DIFF_NIGHTMARE scaling onto a monster that was just built. The golem is
// the player's own and is created through newD1Monster, which skips this.
func (g *Factory) scaleD1NewGamePlus(a *Actor) *Actor {
	if a.d1Monster == nil || g.State == nil || !g.State.D1NewGamePlus() {
		return a
	}
	m, st := a.d1Monster, a.GetStats()
	st.SetAttribute(stats.MaxHealth, st.GetAttribute(stats.MaxHealth)*3+100)
	st.GetResource(stats.Health).SetToMax()
	m.MinDam, m.MaxDam = 2*(m.MinDam+2), 2*(m.MaxDam+2)
	m.MinDam2, m.MaxDam2 = 2*(m.MinDam2+2), 2*(m.MaxDam2+2)
	m.AC += d1.NightmareAcBonus
	m.Hit, m.Hit2 = m.Hit+d1.NightmareToHitBonus, m.Hit2+d1.NightmareToHitBonus
	m.Level += d1.NightmareLevelBonus
	m.Exp = 2 * (m.Exp + 1000)
	st.SetAttribute(stats.Level, m.Level)
	st.SetAttribute(stats.Experience, m.Exp)
	return a
}
