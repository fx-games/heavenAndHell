package common

import (
	"fmt"
	"image/color"
	"rpg/src/defs"
	"rpg/src/dungen"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
	"rpg/src/util"
	"strings"
)

// Monster holds the MonsterStruct fields D1's AI and combat need.
type Monster struct {
	Type                   int
	Data                   *rules.MonsterData
	Unique                 *rules.UniqueMonsterData
	Level, Exp, AILevel    int
	Hit, MinDam, MaxDam    int
	Hit2, MinDam2, MaxDam2 int
	AC, MagicRes           int
	AI                     string
	TalkMsg                string
	MonsterState
	SpawnIndex int     // order of creation on its level, identifies the monster in saves
	Golem      bool    // the player's golem (AI_GOLUM), fights for the player
	HitWith    *[3]int // set for one melee attack: to-hit, min and max damage of a special attack
}

// MonsterState is the AI state a save keeps for each monster.
type MonsterState struct {
	Goal, GoalVar1, GoalVar2, GoalVar3 int
	LastMode, StandTicks               int // _mVar1 (previous mode), _mVar2 (ticks spent standing)
	Squelch, PathCount                 int
	Last                               geometry.Point
	Dir                                int  // index into geometry.Dirs
	Dormant                            bool // _mFlags & 4 on gargoyles: waiting in stone until the enemy comes close
	NoHeal                             bool // _mFlags & 8 (MFLAG_NOHEAL): hit by a weapon that stops healing
	Berserk                            bool // Hellfire's MFLAG_BERSERK | MFLAG_GOLEM: turned on its own kind
	LeaderIdx, LeaderFlag, PackSize    int  // the pack ties (pack.go)
}

// D1 monster modes (MON_MODE) the AI compares against.
const (
	ModeStand     = 0
	ModeWalk      = 1
	ModeAttack    = 4
	ModeGotHit    = 5
	ModeSAttack   = 7
	ModeFadeIn    = 8
	ModeFadeOut   = 9
	ModeRAttack   = 10
	ModeSpStand   = 11
	ModeRSpAttack = 12
	ModeDelay     = 13
	ModeCharge    = 14
	ModeHeal      = 16
)

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// DirTo ports GetDirection.
func DirTo(from, to geometry.Point) int {
	d := geometry.Point{X: sign(to.X - from.X), Y: sign(to.Y - from.Y)}
	for i, v := range geometry.Dirs {
		if v == d {
			return i
		}
	}
	return 0
}

func (a *Actor) Monster() *Monster { return a.monster }

// MonsterSoundCue builds a monster-specific sound event (e.g. "monster.nzombie.attack", from
// assets/data/sounds.rec) for category "attack"/"hit"/"death"/"special": a D1 monster by its type, any other
// monster by its internal name ("monster.skeleton.death"). Empty for the hero.
func (a *Actor) MonsterSoundCue(category string) string {
	if a.hero != nil {
		return ""
	}
	if a.monster != nil && a.monster.Data != nil {
		return defs.IconID("monster", a.monster.Data.ID) + "." + category
	}
	return "monster." + a.GetInternalName() + "." + category
}

// Decision 8: one sheet icon per D1 monster type ("monster.<ID>" in icons/actors.rec, seeded from its graphics
// family), variants by tint. Monster variants are the only tinted tiles; everything else gets its own icon.

// NewMonster ports InitMonster for monster type mt.
func (g *Factory) NewMonster(mt int) *Actor {
	return g.scaleNewGamePlus(g.newMonster(mt, nil))
}

// newMonster is NewMonster's shared core. Given um, it builds the actor as that unique right away, with the
// unique's own "unique.<name>" icon and untinted color from the start, instead of building a plain monster and
// overriding its icon/color/name afterward.
func (g *Factory) newMonster(mt int, um *rules.UniqueMonsterData) *Actor {
	d := g.Generator.D
	m := &d.Monsters[mt]
	name, icon, tint := m.Name, defs.IconID("monster", m.ID), variantTint(m)
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
	a.monster = &Monster{Type: mt, Data: m, Unique: um, Level: m.Level, Exp: m.Exp, AILevel: m.AILevel, Hit: m.Hit, MinDam: m.MinDam(),
		MaxDam: m.MaxDam(), Hit2: m.Hit2, MinDam2: m.MinDam2(), MaxDam2: m.MaxDam2(), AC: m.AC, MagicRes: m.MagicRes, AI: m.AI, MonsterState: MonsterState{Goal: 1}}
	st := a.GetStats()
	st.SetAttribute(stats.MaxHealth, g.Generator.MonsterHP(m))
	st.GetResource(stats.Health).SetToMax()
	st.SetAttribute(stats.Level, m.Level)
	st.SetAttribute(stats.Experience, m.Exp)
	a.monster.Dormant = g.Generator.D.MonsterAIs[m.AI].Dormant
	if m.Has("hidden") {
		a.AddStatusEffect(StatusInvisible, -1)
	}
	g.applyMonsterRecord(a, m)
	return a
}

// applyMonsterRecord gives a monster what its record declares beyond D1's columns: its glyph and color, death
// lines, tags, speed, how far it notices the hero, its skills, its gear and what it says on sight. Diablo's
// records declare none of it.
func (g *Factory) applyMonsterRecord(a *Actor, m *rules.MonsterData) {
	if m.AsciiIcon != 0 {
		a.SetAsciiIcon(m.AsciiIcon)
	}
	if m.UnicodeIcon != 0 {
		a.SetUnicodeIcon(m.UnicodeIcon)
	}
	if m.Color.A != 0 {
		a.SetColor(m.Color)
	}
	if len(m.DeathStringFormats) > 0 {
		a.SetDeathStringFormats(m.DeathStringFormats)
	}
	for _, tag := range m.Tags {
		switch tag {
		case "unique":
			a.SetUnique()
		case "no_corpse":
			a.SetNoCorpse()
		case "flying":
			a.AddStatusEffect(StatusFlying, -1)
		}
	}
	if m.AggroRange > 0 {
		a.SetAggroRange(m.AggroRange)
	}
	a.speechOnSight = m.SpeechOnSight
	var targeted []*TargetedAction
	for _, name := range m.TargetedSkills {
		if s := g.NewTargetedActionFromName(name); s != nil {
			targeted = append(targeted, s)
		}
	}
	var untargeted []*UntargetedAction
	for _, name := range m.UntargetedSkills {
		if s := g.NewUntargetedActionFromName(name); s != nil {
			untargeted = append(untargeted, s)
		}
	}
	if targeted != nil || untargeted != nil {
		a.SetTargetedSkills(targeted)
		a.SetUntargetedSkills(untargeted)
	}
	for _, name := range m.Equipment {
		b, ok := g.Generator.D.ItemsByName[name]
		if !ok {
			g.Msg(fmt.Sprintf("%s is equipped with %q, which this mod's items.rec does not have", m.ID, name))
			continue
		}
		item := g.NewRolledItem(g.Generator.PlainItem(b))
		a.GetInventory().Add(item)
		a.GetEquipment().Equip(item)
	}
}

// variantTint is the monster's tint from enemies.rec: variants of one family share an icon and differ by color.
func variantTint(m *rules.MonsterData) color.RGBA {
	if m.Tint == "" {
		return color.RGBA{R: 255, G: 255, B: 255, A: 255}
	}
	return util.GetColorFromRGBString(m.Tint)
}

// NewUniqueMonster ports PlaceUniqueMonst's stat changes for UniqMonst[u].
func (g *Factory) NewUniqueMonster(u int) *Actor {
	um := &g.Generator.D.UniqueMonsters[u]
	a := g.scaleNewGamePlus(g.newMonster(g.Generator.D.MonsterByID(um.Type), um))
	mon := a.monster
	if um.Level != 0 {
		mon.Level = 2 * um.Level
	} else {
		mon.Level += 5
	}
	mon.Exp *= 2
	mon.AI, mon.AILevel, mon.MagicRes = um.AI, um.AILevel, um.MagicRes
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

// NewMonsterFromLevel builds an Actor from a monster the exact-RNG level generator already placed (dungen.Level's
// Monsters), using its precomputed HP/AI/damage/resist instead of rerolling them: mon.HP is 1/64 units like D1's
// _mmaxhp, so rules.Whole gives whole hit points.
func (g *Factory) NewMonsterFromLevel(mon *dungen.Monster) *Actor {
	var um *rules.UniqueMonsterData
	if mon.Unique >= 0 {
		um = &g.Generator.D.UniqueMonsters[mon.Unique]
	}
	a := g.newMonster(mon.Type, um)
	dm := a.monster
	dm.AI, dm.AILevel, dm.MinDam, dm.MaxDam, dm.MagicRes, dm.Level = mon.AI, mon.AILevel, mon.MinDam, mon.MaxDam, mon.MagicRes, mon.Level
	if um != nil {
		dm.TalkMsg = um.TalkMsg
	}
	dm.Dir = mon.Dir
	dm.LeaderIdx, dm.LeaderFlag, dm.PackSize = mon.Leader, mon.LeaderFlag, mon.PackSize
	hp := max(rules.Whole(mon.HP), 1)
	st := a.GetStats()
	st.SetAttribute(stats.MaxHealth, hp)
	st.GetResource(stats.Health).SetToMax()
	st.SetAttribute(stats.Level, dm.Level)
	return g.scaleNewGamePlus(a)
}

// MonsterSections describes a monster for the info window: what it is, its numbers, what it shrugs off and
// how it fights.
func (a *Actor) MonsterSections(d *rules.Data) []SheetSection {
	mon := a.monster
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
	sections := []SheetSection{{Title: "Creature", Rows: rows}}
	var resists [][2]string
	for _, r := range []struct {
		element      string
		resist, immn int
	}{{"Fire", rules.ResistFire, rules.ImmuneFire}, {"Lightning", rules.ResistLightning, rules.ImmuneLightning},
		{"Magic", rules.ResistMagic, rules.ImmuneMagic}} {
		switch {
		case mon.MagicRes&r.immn != 0:
			resists = append(resists, [2]string{r.element, "immune"})
		case mon.MagicRes&r.resist != 0:
			resists = append(resists, [2]string{r.element, "75%"}) // every damage path quarters a resisted hit (dam >>= 2)
		}
	}
	if len(resists) > 0 {
		sections = append(sections, SheetSection{Title: "Resistances", Rows: resists})
	}
	if behavior := d.MonsterAIs[mon.AI].Description; behavior != "" {
		sections = append(sections, SheetSection{Title: "Behavior", Notes: []string{behavior}})
	}
	return sections
}

func (a *Actor) RemoveStatusEffect(effect StatusEffect) {
	a.statusEffects.SetFalse(string(effect))
}

// NewGamePlus is on once Diablo has been beaten: monsters that spawn and loot that drops from then on use D1's
// own Nightmare numbers, so the dungeon Gillian's donation refills is a match for a hero who already cleared it.
func (s *State) NewGamePlus() bool {
	for q, def := range s.QuestDefs() {
		if def.NewGamePlus && s.QuestActive(q) == rules.QuestDone {
			return true
		}
	}
	return false
}

// ItemLevel is the level loot rolls at on dungeon level dlvl.
func (s *State) ItemLevel(dlvl int) int {
	if s.NewGamePlus() {
		return dlvl + rules.NightmareItemLevelBonus
	}
	return dlvl
}

// scaleNewGamePlus ports InitMonster's DIFF_NIGHTMARE scaling onto a monster that was just built. The golem is
// the player's own and is created through newMonster, which skips this.
func (g *Factory) scaleNewGamePlus(a *Actor) *Actor {
	if a.monster == nil || g.State == nil || !g.State.NewGamePlus() {
		return a
	}
	m, st := a.monster, a.GetStats()
	st.SetAttribute(stats.MaxHealth, st.GetAttribute(stats.MaxHealth)*3+100)
	st.GetResource(stats.Health).SetToMax()
	m.MinDam, m.MaxDam = 2*(m.MinDam+2), 2*(m.MaxDam+2)
	m.MinDam2, m.MaxDam2 = 2*(m.MinDam2+2), 2*(m.MaxDam2+2)
	m.AC += rules.NightmareAcBonus
	m.Hit, m.Hit2 = m.Hit+rules.NightmareToHitBonus, m.Hit2+rules.NightmareToHitBonus
	m.Level += rules.NightmareLevelBonus
	m.Exp = 2 * (m.Exp + 1000)
	st.SetAttribute(stats.Level, m.Level)
	st.SetAttribute(stats.Experience, m.Exp)
	return a
}

// FireWallBlocks ports PosOkMonst's fire check: a monster won't step into a fire wall unless it is immune to fire,
// and Diablo never does.
func FireWallBlocks(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point, mon *Monster) bool {
	o, _ := m.TryGetObjectAt(p)
	w, ok := o.(*DamageDealingObject)
	return ok && w.FireWall && (mon.MagicRes&rules.ImmuneFire == 0 || mon.Data.Has("fire_wall_walker"))
}
