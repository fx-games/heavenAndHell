package d1

import "fmt"

var (
	tilFiles = [5]string{"", `Levels\L1Data\L1.TIL`, `Levels\L2Data\L2.TIL`, `Levels\L3Data\L3.TIL`, `Levels\L4Data\L4.TIL`}
	solFiles = [5]string{"", `Levels\L1Data\L1.SOL`, `Levels\L2Data\L2.SOL`, `Levels\L3Data\L3.SOL`, `Levels\L4Data\L4.SOL`}
)

// Trigger messages (_tmsg).
const (
	TrigNextLevel   = iota // stairs down
	TrigPrevLevel          // stairs up
	TrigTownWarpUp         // up to town
	TrigReturnLevel        // out of a set level
)

type Trigger struct{ X, Y, Msg int }

// LevelSize is the playable width/height in cells: D1's piece coordinates 16..95, shifted to 0..79.
const LevelSize = 80

// CellOrigin is where cell (0,0) sits in piece coordinates.
const CellOrigin = 16

// Level is a dungeon level as D1 creates it on the first visit, in D1's 112×112 piece coordinates (the playable
// area is 16..95, exposed as cells 0..79 by the Cell* helpers).
type Level struct {
	Dlvl, Type, SetLevel int
	Dungeon, PDungeon    [40][40]byte // megatiles; PDungeon is the layout objects switch parts of the level to
	Piece                [112][112]int
	Trans                [112][112]int8
	Flags                [112][112]byte
	SOL                  []byte // per piece: 1 solid, 2 blocks light, 4 blocks missiles, 0x80 wall that can hold a trap
	View                 [2]int // where the player enters
	Triggers             []Trigger
	SetPiece             [4]int // x, y, w, h in megatiles
	Monsters             []*Monster
	Objects              []*Object
	Items                []*PlacedItem
	DiabloQuads          [4][2]int // level 16: megatile origins of diab1-4
	til                  []byte
}

// ChangeMap ports ObjChangeMap on the built level: megatiles (x1, y1)..(x2, y2) switch to PDungeon (ObjSetMini).
// ObjL1Special/ObjL2Special only change how walls are drawn and are skipped; the doors AddL1Objs/AddL2Objs add
// are reported by CellDoor. It reports whether anything changed.
func (l *Level) ChangeMap(x1, y1, x2, y2 int) bool {
	changed := false
	for j := max(y1, 0); j <= min(y2, 39); j++ {
		for i := max(x1, 0); i <= min(x2, 39); i++ {
			t := l.PDungeon[i][j]
			m := megaPieces(l.til, int(t)-1)
			xx, yy := 16+2*i, 16+2*j
			p := [4]int{l.Piece[xx][yy], l.Piece[xx+1][yy], l.Piece[xx][yy+1], l.Piece[xx+1][yy+1]}
			if t != l.Dungeon[i][j] || p != m {
				l.Dungeon[i][j], changed = t, true
				l.Piece[xx][yy], l.Piece[xx+1][yy], l.Piece[xx][yy+1], l.Piece[xx+1][yy+1] = m[0], m[1], m[2], m[3]
			}
		}
	}
	return changed
}

// CellDoor reports a door piece at cell (x, y) (AddL1Objs/AddL2Objs/AddL3Objs).
func (l *Level) CellDoor(x, y int) bool { return doorKind(l.Type, l.CellPiece(x, y)) >= 0 }

func (l *Level) Solid(x, y int) bool   { return l.sol(x, y)&1 != 0 }
func (l *Level) Blocks(x, y int) bool  { return l.sol(x, y)&2 != 0 }
func (l *Level) Missile(x, y int) bool { return l.sol(x, y)&4 != 0 }

// CellSolid/CellBlocks/CellMissile/CellTrap read the SOL flags by cell (0..79) instead of piece (16..95).
func (l *Level) CellSolid(x, y int) bool   { return l.Solid(x+CellOrigin, y+CellOrigin) }
func (l *Level) CellBlocks(x, y int) bool  { return l.Blocks(x+CellOrigin, y+CellOrigin) }
func (l *Level) CellMissile(x, y int) bool { return l.Missile(x+CellOrigin, y+CellOrigin) }
func (l *Level) CellTrap(x, y int) bool {
	px, py := x+CellOrigin, y+CellOrigin
	if px < 0 || py < 0 || px >= 112 || py >= 112 || l.Piece[px][py] >= len(l.SOL) {
		return false
	}
	return l.SOL[l.Piece[px][py]]&0x80 != 0
}

// CellPiece is the dPiece value (1-based, 0 = none) at cell (x, y).
func (l *Level) CellPiece(x, y int) int {
	px, py := x+CellOrigin, y+CellOrigin
	if px < 0 || py < 0 || px >= 112 || py >= 112 {
		return 0
	}
	return l.Piece[px][py] + 1
}

// CellMega is the megatile's raw D1 tile number under cell (x, y) (Dungeon before lPass3), for terrain that's
// easier to read at megatile resolution than by piece ID: L3's lava and fences.
func (l *Level) CellMega(x, y int) byte {
	mx, my := x/2, y/2
	if mx < 0 || my < 0 || mx >= 40 || my >= 40 {
		return 0
	}
	return l.Dungeon[mx][my]
}

func (l *Level) sol(x, y int) byte {
	if x < 0 || y < 0 || x >= 112 || y >= 112 || l.Piece[x][y] >= len(l.SOL) {
		return 0
	}
	return l.SOL[l.Piece[x][y]]
}

// CreateLevel builds dungeon level dlvl, or with setLevel the set level entered from it, the way LoadGameLevel does
// on a first visit: layout, triggers, monster types, themes, monsters, objects, items, theme rooms. It runs on D1's
// RNG from the level's seed and leaves the generator's own RNG alone.
func (gnr *Generator) CreateLevel(q *Quests, seeds [17]int32, dlvl, setLevel int, entry Entry) (lvl *Level, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("creating level %d (set level %d): %v", dlvl, setLevel, r)
		}
	}()
	g := &gen{quests: q, currlevel: dlvl, ltype: LevelType(dlvl), setlevel: setLevel != SLNone, setLvlNum: setLevel}
	g.data = gnr.D
	// a level rebuilds from its seed (save/load), so it can't see uniques dropped since its first visit: it rolls with
	// none dropped and only reports back the ones it placed. A unique already found can reappear on a new level.
	g.igen = &Generator{D: gnr.D, R: &g.rng, UniqueDropped: make([]bool, len(gnr.UniqueDropped))}
	defer func() {
		for u, d := range g.igen.UniqueDropped {
			gnr.UniqueDropped[u] = gnr.UniqueDropped[u] || d
		}
	}()
	seed := seeds[dlvl]
	g.rng.SetSeed(seed)
	g.initLevelMonsters()
	if g.setlevel {
		g.ltype = q.setLevelType(setLevel)
		g.til = g.data.levels.til[g.ltype]
		g.sol = g.data.levels.sol[g.ltype]
		g.loadSetMap(setLevel, seeds)
		g.getLevelMTypes(setLevel)
		g.initMonsters(setLevel)
		g.initItems()
		g.resyncQuests()
	} else {
		g.createDungeon(seed, entry)
		g.initTriggers()
		g.freeUpstairs()
		g.sol = g.data.levels.sol[g.ltype]
		g.rng.SetSeed(seed)
		g.getLevelMTypes(SLNone)
		g.initThemes()
		g.rng.SetSeed(seed)
		g.holdThemes()
		g.rng.Seed()
		g.initMonsters(SLNone)
		g.rng.Seed()
		g.initObjects(seeds)
		g.initItems()
		g.createThemeRooms()
		g.resyncQuests()
	}
	return g.output(setLevel), nil
}

func (q *Quests) setLevelType(sl int) int {
	for i := range q {
		if q[i].SetLevel == sl {
			return q[i].LvlType
		}
	}
	return DTypeCathedral
}

// createDungeon ports CreateDungeon for dungeon levels 1-16.
func (g *gen) createDungeon(seed int32, entry Entry) {
	g.flags = [112][112]byte{}
	g.initTrans()
	g.setPiece, g.setPieceRoom = rect{}, rect{}
	g.til = g.data.levels.til[g.ltype]
	switch g.ltype {
	case DTypeCathedral:
		g.createL1(seed, entry)
	case DTypeCatacombs:
		g.createL2(seed, entry)
	case DTypeCaves:
		g.createL3(seed, entry)
	case DTypeHell:
		g.createL4(seed, entry)
	}
	g.makeSetPC(g.setPiece)
}

// updateL4Trans ports UpdateL4Trans (CreateThemeRooms on Hell levels with theme rooms).
func (g *gen) updateL4Trans() {
	for i := range g.trans {
		for j := range g.trans[i] {
			if g.trans[i][j] != 0 {
				g.trans[i][j] = 1
			}
		}
	}
}

// flat reports whether piece coordinates fall inside D1's 112×112 arrays read as one block (x one past a column
// reads the next one), and the in-range coordinates.
func flat(x, y int) (int, int, bool) {
	i := x*112 + y
	if i < 0 || i >= 112*112 {
		return 0, 0, false
	}
	return i / 112, i % 112, true
}

func (g *gen) pieceAt(x, y int) int {
	if x, y, ok := flat(x, y); ok {
		return g.piece[x][y]
	}
	return 0
}

func (g *gen) solFlag(x, y int, bit byte) bool {
	p := g.pieceAt(x, y)
	return p < len(g.sol) && g.sol[p]&bit != 0
}

func (g *gen) solid(x, y int) bool { return g.solFlag(x, y, 1) }

// solidLoc ports SolidLoc: positions off the map are not solid.
func (g *gen) solidLoc(x, y int) bool {
	return x >= 0 && y >= 0 && x < 112 && y < 112 && g.solid(x, y)
}

func (g *gen) transAt(x, y int) int8 {
	if x, y, ok := flat(x, y); ok {
		return g.trans[x][y]
	}
	return 0
}

func (g *gen) flagAt(x, y int) byte {
	if x, y, ok := flat(x, y); ok {
		return g.flags[x][y]
	}
	return 0
}

// initTriggers ports InitL1-4Triggers (D1's piece numbers are one more than ours).
func (g *gen) initTriggers() {
	g.trigs = nil
	add := func(x, y, msg int) { g.trigs = append(g.trigs, Trigger{x, y, msg}) }
	for j := 0; j < 112; j++ {
		for i := 0; i < 112; i++ {
			switch p := g.piece[i][j] + 1; g.ltype {
			case DTypeCathedral:
				if p == 129 {
					add(i, j, TrigPrevLevel)
				}
				if p == 115 {
					add(i, j, TrigNextLevel)
				}
			case DTypeCatacombs:
				if p == 267 && (i != g.quests[QSchamb].X || j != g.quests[QSchamb].Y) {
					add(i, j, TrigPrevLevel)
				}
				if p == 559 {
					add(i, j, TrigTownWarpUp)
				}
				if p == 271 {
					add(i, j, TrigNextLevel)
				}
			case DTypeCaves:
				if p == 171 {
					add(i, j, TrigPrevLevel)
				}
				if p == 168 {
					add(i, j, TrigNextLevel)
				}
				if p == 549 {
					add(i, j, TrigTownWarpUp)
				}
			case DTypeHell:
				if p == 83 {
					add(i, j, TrigPrevLevel)
				}
				if p == 422 {
					add(i, j, TrigTownWarpUp)
				}
				if p == 120 {
					add(i, j, TrigNextLevel)
				}
			}
		}
	}
	if g.ltype == DTypeHell && g.quests[QBetrayer].Active == QuestDone {
		for j := 0; j < 112; j++ {
			for i := 0; i < 112; i++ {
				if g.piece[i][j]+1 == 370 {
					add(i, j, TrigNextLevel)
				}
			}
		}
	}
}

// freeUpstairs ports Freeupstairs: nothing is placed within 2 cells of a trigger.
func (g *gen) freeUpstairs() {
	for _, t := range g.trigs {
		for yy := -2; yy <= 2; yy++ {
			for xx := -2; xx <= 2; xx++ {
				if x, y, ok := flat(t.X+xx, t.Y+yy); ok {
					g.flags[x][y] |= bflagPopulated
				}
			}
		}
	}
}

var (
	vCrawlTable = [23][30]int{
		{1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6, 0, 7, 0, 8, 0, 9, 0, 10, 0, 11, 0, 12, 0, 13, 0, 14, 0, 15, 0},
		{1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6, 0, 7, 0, 8, 1, 9, 1, 10, 1, 11, 1, 12, 1, 13, 1, 14, 1, 15, 1},
		{1, 0, 2, 0, 3, 0, 4, 1, 5, 1, 6, 1, 7, 1, 8, 1, 9, 1, 10, 1, 11, 1, 12, 2, 13, 2, 14, 2, 15, 2},
		{1, 0, 2, 0, 3, 1, 4, 1, 5, 1, 6, 1, 7, 1, 8, 2, 9, 2, 10, 2, 11, 2, 12, 2, 13, 3, 14, 3, 15, 3},
		{1, 0, 2, 1, 3, 1, 4, 1, 5, 1, 6, 2, 7, 2, 8, 2, 9, 3, 10, 3, 11, 3, 12, 3, 13, 4, 14, 4, 0, 0},
		{1, 0, 2, 1, 3, 1, 4, 1, 5, 2, 6, 2, 7, 3, 8, 3, 9, 3, 10, 4, 11, 4, 12, 4, 13, 5, 14, 5, 0, 0},
		{1, 0, 2, 1, 3, 1, 4, 2, 5, 2, 6, 3, 7, 3, 8, 3, 9, 4, 10, 4, 11, 5, 12, 5, 13, 6, 14, 6, 0, 0},
		{1, 1, 2, 1, 3, 2, 4, 2, 5, 3, 6, 3, 7, 4, 8, 4, 9, 5, 10, 5, 11, 6, 12, 6, 13, 7, 0, 0, 0, 0},
		{1, 1, 2, 1, 3, 2, 4, 2, 5, 3, 6, 4, 7, 4, 8, 5, 9, 6, 10, 6, 11, 7, 12, 7, 12, 8, 13, 8, 0, 0},
		{1, 1, 2, 2, 3, 2, 4, 3, 5, 4, 6, 5, 7, 5, 8, 6, 9, 7, 10, 7, 10, 8, 11, 8, 12, 9, 0, 0, 0, 0},
		{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 5, 7, 6, 8, 7, 9, 8, 10, 9, 11, 9, 11, 10, 0, 0, 0, 0, 0, 0},
		{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 0, 0, 0, 0, 0, 0, 0, 0},
		{1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 9, 11, 10, 11, 0, 0, 0, 0, 0, 0},
		{1, 1, 2, 2, 2, 3, 3, 4, 4, 5, 5, 6, 5, 7, 6, 8, 7, 9, 7, 10, 8, 10, 8, 11, 9, 12, 0, 0, 0, 0},
		{1, 1, 1, 2, 2, 3, 2, 4, 3, 5, 4, 6, 4, 7, 5, 8, 6, 9, 6, 10, 7, 11, 7, 12, 8, 12, 8, 13, 0, 0},
		{1, 1, 1, 2, 2, 3, 2, 4, 3, 5, 3, 6, 4, 7, 4, 8, 5, 9, 5, 10, 6, 11, 6, 12, 7, 13, 0, 0, 0, 0},
		{0, 1, 1, 2, 1, 3, 2, 4, 2, 5, 3, 6, 3, 7, 3, 8, 4, 9, 4, 10, 5, 11, 5, 12, 6, 13, 6, 14, 0, 0},
		{0, 1, 1, 2, 1, 3, 1, 4, 2, 5, 2, 6, 3, 7, 3, 8, 3, 9, 4, 10, 4, 11, 4, 12, 5, 13, 5, 14, 0, 0},
		{0, 1, 1, 2, 1, 3, 1, 4, 1, 5, 2, 6, 2, 7, 2, 8, 3, 9, 3, 10, 3, 11, 3, 12, 4, 13, 4, 14, 0, 0},
		{0, 1, 0, 2, 1, 3, 1, 4, 1, 5, 1, 6, 1, 7, 2, 8, 2, 9, 2, 10, 2, 11, 2, 12, 3, 13, 3, 14, 3, 15},
		{0, 1, 0, 2, 0, 3, 1, 4, 1, 5, 1, 6, 1, 7, 1, 8, 1, 9, 1, 10, 1, 11, 2, 12, 2, 13, 2, 14, 2, 15},
		{0, 1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6, 0, 7, 1, 8, 1, 9, 1, 10, 1, 11, 1, 12, 1, 13, 1, 14, 1, 15},
		{0, 1, 0, 2, 0, 3, 0, 4, 0, 5, 0, 6, 0, 7, 0, 8, 0, 9, 0, 10, 0, 11, 0, 12, 0, 13, 0, 14, 0, 15},
	}
	radiusAdj = [23]int{0, 0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 4, 3, 2, 2, 2, 1, 1, 1, 0, 0, 0, 0}
)

// doVision ports DoVision's BFLAG_VISIBLE marking (InitMonsters keeps monsters out of what the stairs see).
func (g *gen) doVision(nx, ny, radius int) {
	visible := func(x, y int) {
		if x, y, ok := flat(x, y); ok {
			g.flags[x][y] |= bflagVisible
		}
	}
	if nx >= 0 && nx <= 112 && ny >= 0 && ny <= 112 {
		visible(nx, ny)
	}
	for v := 0; v < 4; v++ {
		for j := 0; j < 23; j++ {
			blocked := false
			for k := 0; k < 2*(radius-radiusAdj[j]) && !blocked; k += 2 {
				cx, cy := vCrawlTable[j][k], vCrawlTable[j][k+1]
				x1adj, x2adj, y1adj, y2adj := 0, 0, 0, 0
				diag := cx > 0 && cy > 0
				var x, y int
				switch v {
				case 0:
					x, y = nx+cx, ny+cy
					if diag {
						x1adj, y2adj = -1, -1
					}
				case 1:
					x, y = nx-cx, ny-cy
					if diag {
						y1adj, x2adj = 1, 1
					}
				case 2:
					x, y = nx+cx, ny-cy
					if diag {
						x1adj, y2adj = -1, 1
					}
				case 3:
					x, y = nx-cx, ny+cy
					if diag {
						y1adj, x2adj = -1, 1
					}
				}
				if x < 0 || x > 112 || y < 0 || y > 112 {
					continue
				}
				blocked = g.solFlag(x, y, 2)
				if !g.solFlag(x+x1adj, y+y1adj, 2) || !g.solFlag(x+x2adj, y+y2adj, 2) {
					visible(x, y)
				}
			}
		}
	}
}

func (g *gen) output(setLevel int) *Level {
	l := &Level{Dlvl: g.currlevel, Type: g.ltype, SetLevel: setLevel, Piece: g.piece, Trans: g.trans, Flags: g.flags,
		SOL: g.sol, til: g.til, View: [2]int{g.view.x, g.view.y}, Triggers: g.trigs,
		SetPiece: [4]int{g.setPiece.x, g.setPiece.y, g.setPiece.w, g.setPiece.h}}
	for i := 0; i < 40; i++ {
		for j := 0; j < 40; j++ {
			l.Dungeon[i][j], l.PDungeon[i][j] = g.d(i, j), g.pd(i, j)
		}
	}
	for i, q := range g.l4.quads {
		l.DiabloQuads[i] = [2]int{q.x, q.y}
	}
	l.Monsters = g.monsters
	for _, oi := range g.objectactive[:g.nobjects] {
		l.Objects = append(l.Objects, g.objects[oi])
	}
	for _, ii := range g.itemactive[:g.numitems] {
		l.Items = append(l.Items, g.itemSlots[ii])
	}
	return l
}
