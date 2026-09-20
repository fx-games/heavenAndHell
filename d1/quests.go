package d1

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

// QuestNames are questlist's _qlstr.
var QuestNames = [NumQuests]string{"The Magic Rock", "Black Mushroom", "Gharbad The Weak", "Zhar the Mad", "Lachdanan",
	"Diablo", "The Butcher", "Ogden's Sign", "Halls of the Blind", "Valor", "Anvil of Fury", "Warlord of Blood",
	"The Curse of King Leoric", "Poisoned Water Supply", "The Chamber of Bone", "Archbishop Lazarus"}

// QuestMsgs are questlist's _qdmsg: the text the quest log shows.
var QuestMsgs = [NumQuests]string{"TEXT_INFRA5", "TEXT_MUSH8", "TEXT_GARBUD1", "TEXT_ZHAR1", "TEXT_VEIL9", "TEXT_VILE3",
	"TEXT_BUTCH9", "TEXT_BANNER2", "TEXT_BLINDING", "TEXT_BLOODY", "TEXT_ANVIL5", "TEXT_BLOODWAR", "TEXT_KING2",
	"TEXT_POISON3", "TEXT_BONER", "TEXT_VILE1"}

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

// Quest items (item_indexes): rows of Data.Items.
const (
	IdiRock      = 9
	IdiOptAmulet = 10
	IdiBanner    = 12
	IdiGldnElix  = 15
	IdiAnvil     = 16
	IdiMushroom  = 17
	IdiBrain     = 18
	IdiFungalTm  = 19
	IdiSpecElix  = 20
	IdiBldStone  = 21
	IdiArmOfVal  = 28
	IdiLazStaff  = 33
)

// Quest rewards (unique_item_indexes): rows of Data.UniqueItems.
const (
	UItemInfraRing = 2
	UItemOptAmulet = 3
	UItemTRing     = 4
	UItemHarCrest  = 5
	UItemSteelVeil = 6
	UItemArmOfVal  = 7
	UItemGriswold  = 8
)

// questlist: _qdlvl, _qslvl, _qlvlt.
var questList = [NumQuests][3]int{{5, 0, 0}, {9, 0, 0}, {4, 0, 0}, {8, 0, 0}, {14, 0, 0}, {15, 0, 0}, {2, 0, 0},
	{4, 0, 0}, {7, 0, 0}, {5, 0, 0}, {10, 0, 0}, {13, 0, 0}, {3, SLSkelKing, DTypeCathedral},
	{2, SLPoisonWater, DTypeCaves}, {6, SLBoneChamb, DTypeCatacombs}, {15, SLVileBetrayer, DTypeCathedral}}

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
// Leoric and one quest of each group.
func InitQuests(seeds [NumLevels]int32) (q Quests) {
	for i := range q {
		q[i] = Quest{Active: QuestInit, Level: questList[i][0], SetLevel: questList[i][1], LvlType: questList[i][2]}
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

// available ports QuestStatus.
func (g *gen) available(i int) bool {
	q := &g.quests[i]
	return !g.setlevel && g.currlevel == q.Level && q.Active != QuestNotAvail
}
