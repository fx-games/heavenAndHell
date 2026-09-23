package rules

import (
	"errors"
	"fmt"
	"io/fs"
	"rpg/src/defs"
	"slices"
)

// Quest ids (quest_id).
const (
	QRock = iota
	QMushroom
	QGarbud
	QZhar
	QVeil
	QDiablo
	QButcher
	QLtBanner
	QBlind
	QBlood
	QAnvil
	QWarlord
	QSkelKing
	QPWater
	QSchamb
	QBetrayer
	NumQuests
)

// Quest states (_qactive).
const (
	QuestNotAvail = iota
	QuestInit
	QuestActive
	QuestDone
)

// Set levels (_setlevels).
const (
	SLNone = iota
	SLSkelKing
	SLBoneChamb
	SLMaze
	SLPoisonWater
	SLVileBetrayer
)

// Quest is QuestStruct minus its text.
type Quest struct {
	Active     int
	Level      int
	SetLevel   int // _qslvl
	LvlType    int // _qlvltype: the set level's level type
	X, Y       int // _qtx, _qty (piece coordinates once a level placed them)
	Var1, Var2 int
	Log        bool
}

type Quests [NumQuests]Quest

// questlist's _qlstr and _qdmsg -- a quest's name and log text -- are the mod's, in assets/data/quests.rec.

// Black Mushroom's steps (quest_mush_state, Var1).
const (
	QSInit = iota
	QSTomeSpawned
	QSTomeGiven
	QSMushSpawned
	QSMushPicked
	QSMushGiven
	QSBrainSpawned
	QSBrainGiven
)

// Quest items (item_indexes), by internal_name.
const (
	ItemRock      = "magic_rock"
	ItemOptAmulet = "optic_amulet"
	ItemBanner    = "tavern_sign"
	ItemGldnElix  = "golden_elixir"
	ItemAnvil     = "anvil_of_fury"
	ItemMushroom  = "black_mushroom"
	ItemBrain     = "brain"
	ItemFungalTm  = "fungal_tome"
	ItemSpecElix  = "spectral_elixir"
	ItemBldStone  = "blood_stone"
	ItemArmOfVal  = "arkaine_s_valor"
	ItemLazStaff  = "staff_of_lazarus"
)

// Quest rewards (unique_item_indexes), by unique_items.rec internal_name.
const (
	UItemInfraRing = "empyrean_band"
	UItemOptAmulet = "optic_amulet"
	UItemTRing     = "ring_of_truth"
	UItemHarCrest  = "harlequin_crest"
	UItemSteelVeil = "veil_of_steel"
	UItemArmOfVal  = "arkaines_valor"
	UItemGriswold  = "griswolds_edge"
)

// QuestKeys are the quests.rec keys of the quests whose logic the engine carries, by quest id. A mod has any of
// them it declares, in any order; the rest of its quests are driven by its triggers alone.
var QuestKeys = [NumQuests]string{"rock", "mushroom", "garbud", "zhar", "veil", "diablo", "butcher", "banner",
	"blind", "blood", "anvil", "warlord", "leoric", "pwater", "schamb", "lazarus"}

// setPieces are the set levels the engine can build, by the quest whose set piece each one is (LoadSetMap).
var setPieces = map[string]int{"leoric": SLSkelKing, "pwater": SLPoisonWater, "schamb": SLBoneChamb,
	"lazarus": SLVileBetrayer}

// DTypeByName names the level types a levels.rec "tiles:" field gives.
var DTypeByName = map[string]int{"town": DTypeTown, "cathedral": DTypeCathedral, "catacombs": DTypeCatacombs,
	"caves": DTypeCaves, "hell": DTypeHell}

// NumLevels is D1's NUMLEVELS: town and its 16 dungeon levels.
const NumLevels = 17

// LevelSeeds ports multi.cpp's glSeedTbl: one seed per level drawn from the game seed.
func LevelSeeds(gameSeed int32) (t [NumLevels]int32) {
	var r Rng
	r.SetSeed(gameSeed)
	for i := range t {
		t[i] = r.Seed()
	}
	return
}

// InitQuests ports single-player InitQuests: every quest at its level, then D1 drops one of Poisoned Water and
// Leoric and one quest of each group. A quest's level, set level and set level type are the mod's (questList).
func (d *Data) InitQuests(seeds [NumLevels]int32) (q Quests) {
	for i := range q {
		q[i] = d.questRows[i]
		q[i].Active = QuestInit
	}
	var r Rng
	r.SetSeed(seeds[15])
	if r.Random(2) != 0 {
		q[QPWater].Active = QuestNotAvail
	} else {
		q[QSkelKing].Active = QuestNotAvail
	}
	for _, group := range [][]int{{QButcher, QLtBanner, QGarbud}, {QBlind, QRock, QBlood}, {QMushroom, QZhar, QAnvil}, {QVeil, QWarlord}} {
		if i := r.Random(len(group)); i >= 0 { // a negative roll (seed 988045466) wrote outside the table in D1: no effect
			q[group[i]].Active = QuestNotAvail
		}
	}
	if q[QSkelKing].Active == QuestNotAvail {
		q[QSkelKing].Var2 = 2
	}
	if q[QRock].Active == QuestNotAvail {
		q[QRock].Var2 = 2
	}
	q[QLtBanner].Var1 = 1
	return
}

// loadQuests reads quests.rec and puts the quests the engine knows at their quest ids, whatever order the file
// lists them in. Each one's level is the depth of its quests.rec "level"; its set level, if levels.rec has a
// d1_set record naming the quest, is that record's set piece and tile set.
func loadQuests(fsys fs.FS, d *Data) error {
	all, triggers, err := defs.ReadQuests(fsys)
	if err != nil {
		return err
	}
	d.QuestTriggers = triggers
	d.QuestDefs = make([]defs.Quest, NumQuests)
	for _, q := range all {
		if id := slices.Index(QuestKeys[:], q.Key); id >= 0 {
			d.QuestDefs[id] = q
		} else {
			d.QuestDefs = append(d.QuestDefs, q)
		}
	}
	levels, err := ReadRec(fsys, "levels.rec")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	depth, setTiles := map[string]int{}, map[string]string{}
	for _, r := range levels {
		m := r.ToMap("")
		depth[m.GetStringOrDefault("map", "")] = m.GetIntOrDefault("depth", 0)
		if m.GetStringOrDefault("generator", "") == "d1_set" {
			setTiles[m.GetStringOrDefault("quest", "")] = m.GetStringOrDefault("tiles", "")
		}
	}
	for id, key := range QuestKeys {
		row := &d.questRows[id]
		row.Level = depth[d.QuestDefs[id].Level]
		if tiles, ok := setTiles[key]; ok && d.QuestDefs[id].Key != "" {
			lt, known := DTypeByName[tiles]
			if !known || setPieces[key] == SLNone {
				return fmt.Errorf("levels.rec: set level of quest %q: tiles %q, set piece %d", key, tiles, setPieces[key])
			}
			row.SetLevel, row.LvlType = setPieces[key], lt
		}
	}
	return nil
}
