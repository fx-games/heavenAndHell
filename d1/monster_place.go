package d1

// monster.cpp: monster types and placement on level creation.

const (
	placeScatter = 1
	placeSpecial = 2
	placeUnique  = 4

	maxMonsters   = 200
	maxLvlMTypes  = 16
	numMTypes     = 111 // D1's NUM_MTYPES, which leaves out MT_DARKMAGE
	umtGarbud     = 0
	umtSkelKing   = 1
	umtZhar       = 2
	umtSnotspil   = 3
	umtLazarus    = 4
	umtRedVex     = 5
	umtBlackJade  = 6
	umtLachdanan  = 7
	umtWarlord    = 8
	umtButcher    = 9
	aiGargoyleStr = "AI_GARG"
)

// Monster is a monster D1 placed on the level (MonsterStruct's lasting fields), in piece coordinates.
type Monster struct {
	Type       int // row of Data.Monsters
	Unique     int // row of Data.UniqueMonsters, -1 = none
	X, Y, Dir  int
	HP         int // hit points in 1/64
	AI         string
	Int, Level int
	Leader     int // slot of the pack leader in Level.Monsters, -1 = none
	LeaderFlag int
	PackSize   int
	Reserved   bool // one of the four slots D1 keeps for golems
	Hidden     bool // pre-spawned skeleton waiting in a sarcophagus or barrel
	Seed       int32
	AISeed     int32
	UniqTrans  int
	MinDam     int
	MaxDam     int
	MagicRes   int
	TalkMsg    string
}

type levelMType struct{ mtype, placeFlags int }

// population holds what LoadGameLevel fills after the layout: monsters, objects, items and theme rooms.
type population struct {
	data *Data
	igen *Generator // item rolls on the level RNG
	sol  []byte

	trigs       []Trigger
	dMonster    [112][112]int
	dObject     [112][112]int8
	dItem       [112][112]int8
	mtypes      []levelMType
	monstimgtot int
	monsters    []*Monster
	uniquetrans int
	totalmon    int

	objects      [maxObjects]*Object
	objectavail  [maxObjects]int
	objectactive [maxObjects]int
	nobjects     int
	trapid       int
	trapdir      int
	leverid      int
	initObjFlag  bool

	itemSlots  [maxItems]*PlacedItem
	itemavail  [maxItems]int
	itemactive [maxItems]int
	numitems   int

	rooms                                                                   []theme // themes.cpp's themes
	zharlib                                                                 int
	themex, themey, themeVar1                                               int
	armorFlag, bFountain, cauldron, mFountain, pFountain, tFountain, treasr bool
	weaponFlag, bCrossFlag                                                  bool
}

func (g *gen) dMon(x, y int) int {
	if x, y, ok := flat(x, y); ok {
		return g.dMonster[x][y]
	}
	return 0
}

func (g *gen) dObj(x, y int) int8 {
	if x, y, ok := flat(x, y); ok {
		return g.dObject[x][y]
	}
	return 0
}

func (g *gen) dIt(x, y int) int8 {
	if x, y, ok := flat(x, y); ok {
		return g.dItem[x][y]
	}
	return 0
}

func isSkel(mt int) bool {
	return mt >= MtWSKELAX && mt <= MtXSKELAX || mt >= MtWSKELBW && mt <= MtXSKELBW || mt >= MtWSKELSD && mt <= MtXSKELSD
}

func isGoat(mt int) bool {
	return mt >= MtNGOATMC && mt <= MtGGOATMC || mt >= MtNGOATBW && mt <= MtGGOATBW
}

// initLevelMonsters ports InitLevelMonsters; ClrAllMonsters rolls a direction and an enemy for all 200 slots.
func (g *gen) initLevelMonsters() {
	g.mtypes, g.monstimgtot, g.monsters, g.uniquetrans = nil, 0, nil, 0
	g.totalmon = maxMonsters
	for i := 0; i < maxMonsters; i++ {
		g.rng.Random(8)
		g.rng.Random(1)
	}
}

// addMonsterType ports AddMonsterType: the level's index of a monster type.
func (g *gen) addMonsterType(mtype, placeFlag int) int {
	for i, t := range g.mtypes {
		if t.mtype == mtype {
			g.mtypes[i].placeFlags |= placeFlag
			return i
		}
	}
	g.mtypes = append(g.mtypes, levelMType{mtype, placeFlag})
	g.monstimgtot += g.data.Monsters[mtype].ImgSize
	return len(g.mtypes) - 1
}

func (g *gen) uniqueType(u int) int { return g.data.MonsterByID(g.data.UniqueMonsters[u].Type) }

// getLevelMTypes ports GetLevelMTypes.
func (g *gen) getLevelMTypes(setLevel int) {
	g.addMonsterType(MtGOLEM, placeSpecial)
	if g.currlevel == 16 {
		g.addMonsterType(MtADVOCATE, placeScatter)
		g.addMonsterType(MtRBLACK, placeScatter)
		g.addMonsterType(MtDIABLO, placeSpecial)
		return
	}
	if g.setlevel {
		if setLevel == SLSkelKing {
			g.addMonsterType(MtSKING, placeUnique)
		}
		return
	}
	if g.available(QButcher) {
		g.addMonsterType(MtCLEAVER, placeSpecial)
	}
	for _, qu := range [][2]int{{QGarbud, umtGarbud}, {QZhar, umtZhar}, {QLtBanner, umtSnotspil}, {QVeil, umtLachdanan}, {QWarlord, umtWarlord}} {
		if g.available(qu[0]) {
			g.addMonsterType(g.uniqueType(qu[1]), placeUnique)
		}
	}
	var typelist []int
	for i := 0; i < numMTypes; i++ {
		m := &g.data.Monsters[i]
		if minl, maxl := 15*m.MinDLvl/30+1, 15*m.MaxDLvl/30+1; g.currlevel >= minl && g.currlevel <= maxl && m.Available {
			typelist = append(typelist, i)
		}
	}
	for len(typelist) > 0 && len(g.mtypes) < maxLvlMTypes && g.monstimgtot < 4000 {
		for i := 0; i < len(typelist); {
			if g.data.Monsters[typelist[i]].ImgSize > 4000-g.monstimgtot {
				typelist[i] = typelist[len(typelist)-1]
				typelist = typelist[:len(typelist)-1]
				continue
			}
			i++
		}
		if len(typelist) != 0 {
			i := g.rnd(len(typelist))
			g.addMonsterType(typelist[i], placeScatter)
			typelist[i] = typelist[len(typelist)-1]
			typelist = typelist[:len(typelist)-1]
		}
	}
}

// initMonster ports InitMonster's rolls and stats (single player, normal difficulty).
func (g *gen) initMonster(slot, dir, mt, x, y int) *Monster {
	d := &g.data.Monsters[g.mtypes[mt].mtype]
	g.rnd(d.Rate[0] - 1)   // _mAnimCnt
	g.rnd(d.Frames[0] - 1) // _mAnimFrame
	var hp int
	if g.mtypes[mt].mtype == MtDIABLO {
		hp = (g.rnd(1) + 3333) << 6 // Hellfire's Diablo
	} else {
		hp = (d.MinHP + g.rnd(d.MaxHP-d.MinHP+1)) << 6
	}
	hp = max(hp>>1, 64)
	m := &Monster{Type: g.mtypes[mt].mtype, Unique: -1, X: x, Y: y, Dir: dir, HP: hp, AI: d.AI, Int: d.Int, Level: d.Level,
		Leader: -1, MinDam: d.MinDam, MaxDam: d.MaxDam, MagicRes: d.MagicRes}
	m.Seed = g.rng.Seed()
	m.AISeed = g.rng.Seed()
	for len(g.monsters) <= slot {
		g.monsters = append(g.monsters, nil)
	}
	g.monsters[slot] = m
	return m
}

// addMonster ports AddMonster.
func (g *gen) addMonster(x, y, dir, mt int, inMap bool) int {
	if len(g.monsters) >= maxMonsters {
		return -1
	}
	slot := len(g.monsters)
	if inMap {
		if x, y, ok := flat(x, y); ok {
			g.dMonster[x][y] = slot + 1
		}
	}
	g.initMonster(slot, dir, mt, x, y)
	return slot
}

// monstPlace ports MonstPlace.
func (g *gen) monstPlace(x, y int) bool {
	if x < 0 || x >= 112 || y < 0 || y >= 112 || g.dMonster[x][y] != 0 {
		return false
	}
	return g.flags[x][y]&(bflagVisible|bflagPopulated) == 0 && !g.solidLoc(x, y)
}

// placeMonster ports PlaceMonster.
func (g *gen) placeMonster(slot, mt, x, y int) {
	if x, y, ok := flat(x, y); ok {
		g.dMonster[x][y] = slot + 1
	}
	g.initMonster(slot, g.rnd(8), mt, x, y)
}

var offsetX = [8]int{1, 0, -1, -1, -1, 0, 1, 1}
var offsetY = [8]int{1, 1, 1, 0, -1, -1, -1, 0}

// placeUniqueMonst ports PlaceUniqueMonst.
func (g *gen) placeUniqueMonst(u, minionType, bosspacksize int) {
	uniq := &g.data.UniqueMonsters[u]
	if (g.uniquetrans+19)<<8 >= 6912 { // LIGHTSIZE: no palette left for another unique
		return
	}
	uniqtype := len(g.mtypes)
	for i, t := range g.mtypes {
		if t.mtype == g.uniqueType(u) {
			uniqtype = i
			break
		}
	}
	var xp, yp int
	for count := 0; ; {
		xp, yp = g.rnd(80)+16, g.rnd(80)+16
		count2 := 0
		for x := xp - 3; x < xp+3; x++ {
			for y := yp - 3; y < yp+3; y++ {
				if y >= 0 && y < 112 && x >= 0 && x < 112 && g.monstPlace(x, y) {
					count2++
				}
			}
		}
		if count2 < 9 {
			if count++; count < 1000 {
				continue
			}
		}
		if g.monstPlace(xp, yp) {
			break
		}
	}
	switch u {
	case umtSnotspil:
		xp, yp = 2*g.setPiece.x+24, 2*g.setPiece.y+28
	case umtWarlord:
		xp, yp = 2*g.setPiece.x+22, 2*g.setPiece.y+23
	case umtZhar:
		if g.zharlib >= 0 && g.zharlib < len(g.themes) {
			xp, yp = 2*g.themes[g.zharlib].room.x+20, 2*g.themes[g.zharlib].room.y+20
		}
	case umtLazarus:
		xp, yp = 32, 46
	case umtRedVex:
		xp, yp = 40, 45
	case umtBlackJade:
		xp, yp = 38, 49
	case umtSkelKing:
		xp, yp = 35, 47
	case umtButcher:
		done := false
		for yp = 0; yp < 112 && !done; yp++ {
			for xp = 0; xp < 112 && !done; xp++ {
				done = g.piece[xp][yp]+1 == 367
			}
		}
	}
	slot := len(g.monsters)
	g.placeMonster(slot, uniqtype, xp, yp)
	m := g.monsters[slot]
	m.Unique = u
	if uniq.Level != 0 {
		m.Level = 2 * uniq.Level
	} else {
		m.Level += 5
	}
	m.HP = max((uniq.MaxHP<<6)>>1, 64)
	m.AI, m.Int, m.MinDam, m.MaxDam, m.MagicRes, m.TalkMsg = uniq.AI, uniq.Int, uniq.MinDam, uniq.MaxDam, uniq.MagicRes, uniq.TalkMsg
	m.UniqTrans = g.uniquetrans
	g.uniquetrans++
	if uniq.UnqAttr&1 != 0 {
		g.placeGroup(minionType, bosspacksize, uniq.UnqAttr, slot)
	}
	if m.AI != aiGargoyleStr {
		g.rnd(g.data.Monsters[m.Type].Frames[0] - 1) // _mAnimFrame
	}
}

// placeUniques ports PlaceUniques: the uniques of this level whose monster type is on it.
func (g *gen) placeUniques() {
	for u := range g.data.UniqueMonsters {
		if g.data.UniqueMonsters[u].Level != g.currlevel {
			continue
		}
		mt, done := 0, false
		for mt = 0; mt < len(g.mtypes) && !done; mt++ {
			done = g.mtypes[mt].mtype == g.uniqueType(u)
		}
		mt--
		switch {
		case u == umtGarbud && g.quests[QGarbud].Active == QuestNotAvail,
			u == umtZhar && g.quests[QZhar].Active == QuestNotAvail,
			u == umtSnotspil && g.quests[QLtBanner].Active == QuestNotAvail,
			u == umtLachdanan && g.quests[QVeil].Active == QuestNotAvail,
			u == umtWarlord && g.quests[QWarlord].Active == QuestNotAvail:
			done = false
		}
		if done {
			g.placeUniqueMonst(u, mt, 8)
		}
	}
}

// placeQuestMonsters ports PlaceQuestMonsters.
func (g *gen) placeQuestMonsters(setLevel int) {
	if g.setlevel {
		if setLevel == SLSkelKing {
			g.placeUniqueMonst(umtSkelKing, 0, 0)
		}
		return
	}
	if g.available(QButcher) {
		g.placeUniqueMonst(umtButcher, 0, 0)
	}
	sp := g.setPiece
	if g.available(QLtBanner) {
		g.setMapMonsters(g.loadDun("l1/banner1"), 2*sp.x, 2*sp.y)
	}
	if g.available(QBlood) {
		g.setMapMonsters(g.loadDun("l2/blood2"), 2*sp.x, 2*sp.y)
	}
	if g.available(QBlind) {
		g.setMapMonsters(g.loadDun("l2/blind2"), 2*sp.x, 2*sp.y)
	}
	if g.available(QAnvil) {
		g.setMapMonsters(g.loadDun("l3/anvil"), 2*sp.x+2, 2*sp.y+2)
	}
	if g.available(QWarlord) {
		g.setMapMonsters(g.loadDun("l4/warlord"), 2*sp.x, 2*sp.y)
		g.addMonsterType(g.uniqueType(umtWarlord), placeScatter)
	}
	if g.available(QVeil) {
		g.addMonsterType(g.uniqueType(umtLachdanan), placeScatter)
	}
	if g.available(QZhar) && g.zharlib == -1 {
		g.quests[QZhar].Active = QuestNotAvail
	}
}

// placeGroup ports PlaceGroup.
func (g *gen) placeGroup(mt, num, leaderf, leader int) {
	placed := 0
	for try1 := 0; try1 < 10; try1++ {
		for ; placed > 0; placed-- {
			last := g.monsters[len(g.monsters)-1]
			g.monsters = g.monsters[:len(g.monsters)-1]
			if x, y, ok := flat(last.X, last.Y); ok {
				g.dMonster[x][y] = 0
			}
		}
		var xp, yp int
		if leaderf&1 != 0 {
			o := g.rnd(8)
			xp, yp = g.monsters[leader].X+offsetX[o], g.monsters[leader].Y+offsetY[o]
		} else {
			for {
				xp, yp = g.rnd(80)+16, g.rnd(80)+16
				if g.monstPlace(xp, yp) {
					break
				}
			}
		}
		x1, y1 := xp, yp
		if num+len(g.monsters) > g.totalmon {
			num = g.totalmon - len(g.monsters)
		}
		for j, try2 := 0, 0; j < num && try2 < 100; xp, yp = xp+offsetX[g.rnd(8)], yp+offsetX[g.rnd(8)] { // D1 bug: offset_x for y
			if !g.monstPlace(xp, yp) || g.transAt(xp, yp) != g.transAt(x1, y1) ||
				leaderf&2 != 0 && (iabs(xp-x1) >= 4 || iabs(yp-y1) >= 4) {
				try2++
				continue
			}
			slot := len(g.monsters)
			g.placeMonster(slot, mt, xp, yp)
			if leaderf&1 != 0 {
				m, l := g.monsters[slot], g.monsters[leader]
				m.HP *= 2
				m.Int = l.Int
				if leaderf&2 != 0 {
					m.Leader, m.LeaderFlag, m.AI = leader, 1, l.AI
				}
				if m.AI != aiGargoyleStr {
					g.rnd(g.data.Monsters[m.Type].Frames[0] - 1)
				}
			}
			placed++
			j++
		}
		if placed >= num {
			break
		}
	}
	if leaderf&2 != 0 {
		g.monsters[leader].PackSize = placed
	}
}

// loadDiabMonsts ports LoadDiabMonsts.
func (g *gen) loadDiabMonsts() {
	for i, name := range [4]string{"l4/diab1", "l4/diab2a", "l4/diab3a", "l4/diab4a"} {
		g.setMapMonsters(g.loadDun(name), 2*g.l4.quads[i].x, 2*g.l4.quads[i].y)
	}
}

// initMonsters ports InitMonsters.
func (g *gen) initMonsters(setLevel int) {
	if !g.setlevel {
		for i := 0; i < 4; i++ {
			g.monsters[g.addMonster(1, 0, 0, 0, false)].Reserved = true
		}
		if g.currlevel == 16 {
			g.loadDiabMonsts()
		}
	}
	nt := len(g.trigs)
	if g.currlevel == 15 {
		nt = min(nt, 1)
	}
	saved := g.flags
	for i := 0; i < nt; i++ {
		for s := -2; s < 2; s++ {
			for t := -2; t < 2; t++ {
				g.doVision(s+g.trigs[i].X, t+g.trigs[i].Y, 15)
			}
		}
	}
	g.placeQuestMonsters(setLevel)
	if !g.setlevel {
		g.placeUniques()
		na := 0
		for s := 16; s < 96; s++ {
			for t := 16; t < 96; t++ {
				if !g.solidLoc(s, t) {
					na++
				}
			}
		}
		n := na / 30
		if len(g.monsters)+n > maxMonsters-10 {
			n = maxMonsters - 10 - len(g.monsters)
		}
		g.totalmon = len(g.monsters) + n
		var scatter []int
		for i, t := range g.mtypes {
			if t.placeFlags&placeScatter != 0 {
				scatter = append(scatter, i)
			}
		}
		for len(g.monsters) < g.totalmon {
			mt := scatter[g.rnd(len(scatter))]
			na := 1
			if g.currlevel != 1 && g.rnd(2) != 0 {
				if g.currlevel == 2 {
					na = g.rnd(2) + 2
				} else {
					na = g.rnd(3) + 3
				}
			}
			g.placeGroup(mt, na, 0, 0)
		}
	}
	// DoUnVision clears the marks again; only BFLAG_VISIBLE was added
	for x := range g.flags {
		for y := range g.flags[x] {
			g.flags[x][y] = g.flags[x][y]&^bflagVisible | saved[x][y]&bflagVisible
		}
	}
}

// setMapMonsters ports SetMapMonsters: the monsters a .DUN places.
func (g *gen) setMapMonsters(d dun, startx, starty int) {
	g.addMonsterType(MtGOLEM, placeSpecial)
	for i := 0; i < 4; i++ {
		g.monsters[g.addMonster(1, 0, 0, 0, false)].Reserved = true
	}
	if g.setlevel && g.setLvlNum == SLVileBetrayer {
		for _, u := range []int{umtLazarus, umtRedVex, umtBlackJade} {
			g.addMonsterType(g.uniqueType(u), placeUnique)
		}
		for _, u := range []int{umtLazarus, umtRedVex, umtBlackJade} {
			g.placeUniqueMonst(u, 0, 0)
		}
	}
	for j := 0; j < 2*d.h(); j++ {
		for i := 0; i < 2*d.w(); i++ {
			if v := d.layer(1, i, j); v != 0 {
				mt := g.addMonsterType(monstConvTbl[v-1], placeSpecial)
				g.placeMonster(len(g.monsters), mt, i+startx+16, j+starty+16)
			}
		}
	}
}
