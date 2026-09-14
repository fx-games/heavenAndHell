package common

import (
	"fmt"
	"image/color"
	"rpg/d1"
	"rpg/geometry"
	"rpg/stats"
	"strings"
)

// D1Monster holds the MonsterStruct fields D1's AI and combat need.
type D1Monster struct {
	Type                               int
	Data                               *d1.MonsterData
	Unique                             *d1.UniqueMonsterData
	Level, Exp, Int                    int
	Hit, MinDam, MaxDam                int
	Hit2, MinDam2, MaxDam2             int
	AC, MagicRes                       int
	AI                                 string
	Goal, GoalVar1, GoalVar2, GoalVar3 int
	LastMode, StandTicks               int // _mVar1 (previous mode), _mVar2 (ticks spent standing)
	Squelch, PathCount                 int
	Last                               geometry.Point
	Dir                                int // index into D1Dirs
	Leader                             *Actor
	SpawnIndex                         int     // order of creation on its level, identifies the monster in saves
	Golem                              bool    // the player's golem (AI_GOLUM), fights for the player
	Dormant                            bool    // _mFlags & 4 on gargoyles: waiting in stone until the enemy comes close
	HitWith                            *[3]int // set for one melee attack: to-hit, min and max damage of a special attack
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

// Decision 8: one sheet icon per D1 graphics family, variants by tint. First guesses; tune freely.
var d1FamilyIcons = map[string]int32{
	"Zombie": 485, "FalSpear": 428, "SkelAxe": 397, "FalSword": 427, "Scav": 540, "SkelBow": 401, "SkelSd": 400,
	"TSneak": 411, "Sneak": 411, "GoatLord": 95, "GoatMace": 95, "Bat": 93, "GoatBow": 97, "Acid": 90,
	"SKing": 259, "FatC": 258, "Fat": 99, "Worm": 91, "Magma": 102, "Rhino": 100, "Demskel": 96, "Thin": 101,
	"Fireman": 98, "BigFall": 429, "Gargoyle": 88, "Mega": 487, "Snake": 91, "Black": 412, "Unrav": 413,
	"Succ": 454, "Mage": 453, "Golem": 96, "Diablo": 261, "DarkMage": 453,
}

var (
	d1VariantTints = []color.RGBA{{255, 255, 255, 255}, {160, 200, 255, 255}, {255, 190, 120, 255}, {255, 110, 110, 255}}
	d1UniqueTint   = color.RGBA{R: 255, G: 215, B: 80, A: 255}
)

// NewD1Monster ports InitMonster for monster type mt.
func (g *Factory) NewD1Monster(mt int) *Actor {
	d := g.d1.D
	m := &d.Monsters[mt]
	variant := 0
	for i := 0; i < mt; i++ {
		if d.Monsters[i].Family == m.Family {
			variant++
		}
	}
	a := NewActor(d1FamilyIcons[m.Family])
	a.SetName(m.Name)
	a.SetInternalName(m.ID)
	a.SetHostile()
	a.SetAsciiIcon(rune(strings.ToLower(m.Name)[0]))
	a.SetColor(d1VariantTints[variant%len(d1VariantTints)])
	a.d1Monster = &D1Monster{Type: mt, Data: m, Level: m.Level, Exp: m.Exp, Int: m.Int, Hit: m.Hit, MinDam: m.MinDam,
		MaxDam: m.MaxDam, Hit2: m.Hit2, MinDam2: m.MinDam2, MaxDam2: m.MaxDam2, AC: m.AC, MagicRes: m.MagicRes, AI: m.AI, Goal: 1}
	st := a.GetStats()
	st.SetAttribute(stats.MaxHealth, g.d1.MonsterHP(m))
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

// NewD1UniqueMonster ports PlaceUniqueMonst's stat changes for UniqMonst[u].
func (g *Factory) NewD1UniqueMonster(u int) *Actor {
	um := &g.d1.D.UniqueMonsters[u]
	a := g.NewD1Monster(g.d1.D.MonsterByID(um.Type))
	mon := a.d1Monster
	mon.Unique = um
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
	a.SetName(um.Name)
	a.SetUnique()
	a.SetColor(d1UniqueTint)
	st := a.GetStats()
	st.SetAttribute(stats.MaxHealth, max(um.MaxHP>>1, 1))
	st.GetResource(stats.Health).SetToMax()
	st.SetAttribute(stats.Level, mon.Level)
	st.SetAttribute(stats.Experience, mon.Exp)
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
	lines := []string{a.name, fmt.Sprintf("Level %d %s", mon.Level, class), "",
		fmt.Sprintf("HP: %s", a.GetStats().GetResource(stats.Health).ToString()),
		fmt.Sprintf("Armor class: %d", mon.AC),
		fmt.Sprintf("Damage: %d-%d", mon.MinDam, mon.MaxDam),
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
	return lines
}

func (a *Actor) RemoveStatusEffect(effect StatusEffect) {
	a.statusEffects.SetFalse(string(effect))
}
