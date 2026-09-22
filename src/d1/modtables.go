package d1

// tables are the mod's rule tables that Load reads from its data/*.rec files, embedded in Data so every game
// (and every test) has its own.
type tables struct {
	// ClassNames are the class names, in the mod's own order (hero.rec).
	ClassNames []string
	classAttr  []classAttributes
	// MaxStats is the attribute cap per class and ATTRIB_ (CharacterAttribute), derived from classAttr.
	MaxStats [][4]int
	// ExpLvls is the experience total each character level needs.
	ExpLvls [51]int
	// SpellFrames: casting animation length per class (each class' animations.tsv castingFrames).
	SpellFrames   []int
	WeaponBonuses []WeaponBonus
	// heroTablesLoaded records whether the mod supplied hero.rec. It is NOT optional in the way the xl_* tables
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
	shrineMax                                   []int // deepest dungeon level a shrine may be generated on
	shrineAvail                                 []int // 2 = multiplayer only
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
	// MonsterAIDescriptions is a plain-English gloss of each AI_* brain, shown in the actor description popup
	// (the ai section of monsters.rec).
	MonsterAIDescriptions map[string]string
}
