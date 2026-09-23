package rules

import (
	"rpg/src/defs"
	"text/template"
)

// tables are the mod's rule tables that Load reads from its data/*.rec files, embedded in Data so every game
// (and every test) has its own.
type tables struct {
	// classes are classes.rec's classes by internal_name.
	classes map[string]*defs.LoadoutDefinition
	// questRows are each engine quest's level, set level and set level type (questlist), from quests.rec and
	// levels.rec.
	questRows [NumQuests]Quest
	// ExpLvls is the experience total each character level needs.
	ExpLvls       [51]int
	WeaponBonuses []WeaponBonus
	// heroTablesLoaded records whether the mod supplied classes.rec and hero.rec. It is NOT optional in the way the xl_* tables
	// are: those describe features a mod may not have, while this one is the hero's own numbers, and a game with
	// no hero numbers has no hero. It is tolerated only so that the rest of a mod - its monsters, items, vendors
	// and levels - can be loaded and worked on, and HeroTablesLoaded exists so nothing quietly builds a hero out
	// of a table of zeroes. Zeroed class tables have silently broken this engine once already.
	heroTablesLoaded bool
	// ObjectDisplays is keyed by object kind (Obj*), from the object section of objects.rec.
	ObjectDisplays map[int]ObjectDisplay
	// ShrineNames are D1's shrinestrs; ShrineMessages the MsgStrings each shrine shows, with a note on what the
	// shrine does; StoryBookNames D1's StoryBookName, indexed by a story book's Var3 (objects.rec).
	ShrineNames, ShrineMessages, StoryBookNames []string
	ShrineMax                                   []int // deepest dungeon level a shrine may be generated on
	ShrineAvail                                 []int // 2 = multiplayer only
	// SfxFiles is every speech and quest sound id D1 has (_sfx_id, effects.cpp's sgSFX), mapped to the file it
	// was in Diablo (the .wav path lowercased, as .ogg), from sfx.rec. The file names are the key into
	// reference/d1_voice_transcripts.tsv; voices.rec plays them from diablo_sfx/.
	SfxFiles map[string]string
	// itemCAnimTbl, itemDropSnds and itemInvSnds are item_sounds.rec: a "sound" record per sound class holding
	// its drop and inventory sound, and an "anim" record whose repeated curs fields are ItemCAnimTbl.
	itemCAnimTbl              []int
	itemDropSnds, itemInvSnds []string
	oils                      []oil
	// propertyDefs is properties.rec. The wording lives there; the four ways a value turns into a line stay in
	// properties.go, because they are code.
	propertyDefs map[string]propertyDef
	// RuneTraps is the rune section of spells.rec.
	RuneTraps map[string]RuneTrap
	// MonsterAIs is the ai section of monsters.rec, by AI_* name.
	MonsterAIs map[string]MonsterAI
	// itemNames are item_names.rec's templates, by where the name shows (ItemNameTemplate).
	itemNames map[string]*template.Template
}
