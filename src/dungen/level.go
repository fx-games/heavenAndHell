package dungen

import (
	"fmt"
	"rpg/src/hhm"
	"rpg/src/rules"
)

var (
	tilFiles = [5]string{"", `Levels\L1Data\L1.TIL`, `Levels\L2Data\L2.TIL`, `Levels\L3Data\L3.TIL`, `Levels\L4Data\L4.TIL`}
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
	PieceFlags           []hhm.Flags // by piece: what its tile in the level type's megatile map parts blocks
	View                 [2]int      // where the player enters
	Triggers             []Trigger
	SetPiece             [4]int // x, y, w, h in megatiles
	Monsters             []*Monster
	Objects              []*rules.Object
	Items                []*PlacedItem
	DiabloQuads          [4][2]int // level 16: megatile origins of diab1-4
	Uber, Cornerstone    [2]int    // the Crypt: Na-Krul's door (UberRow/UberCol) and the Cornerstone of the World
	til                  []byte
	parts                *hhm.Level // the level type's megatile map parts (CellTile)
}

// OpenNakrulRoom ports SyncNakrulRoom: Na-Krul's door opens. It reports whether anything changed.
func (l *Level) OpenNakrulRoom() bool {
	x, y := l.Uber[0], l.Uber[1]
	if x == 0 || y == 0 || l.Piece[x][y] == 297 {
		return false
	}
	l.Piece[x][y], l.Piece[x][y-1], l.Piece[x][y-2], l.Piece[x][y+1] = 297, 300, 299, 298
	return true
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

// Solid is D1's nSolidTable at piece (x, y): its map-part tile blocks movement.
func (l *Level) Solid(x, y int) bool { return l.pieceFlags(x, y).BlocksMovement }

// CellSolid and CellFlags read the piece at cell (x, y) (0..79) instead of piece coordinates (16..95).
func (l *Level) CellSolid(x, y int) bool      { return l.Solid(x+CellOrigin, y+CellOrigin) }
func (l *Level) CellFlags(x, y int) hhm.Flags { return l.pieceFlags(x+CellOrigin, y+CellOrigin) }

// CellTile is the tile cell (x, y) is drawn with: its quadrant of its megatile in the level type's map parts.
func (l *Level) CellTile(x, y int) int {
	return l.parts.TileAt(megatileCell(int(l.CellMega(x, y)), x%2+2*(y%2)))
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

func (l *Level) pieceFlags(x, y int) hhm.Flags {
	if x < 0 || y < 0 || x >= 112 || y >= 112 || l.Piece[x][y] >= len(l.PieceFlags) {
		return hhm.Flags{}
	}
	return l.PieceFlags[l.Piece[x][y]]
}

// CreateLevel builds, with gnr's tables and unique bookkeeping, dungeon level dlvl, or with setLevel the set level entered from it, the way LoadGameLevel does
// on a first visit: layout, triggers, monster types, themes, monsters, objects, items, theme rooms. It runs on D1's
// RNG from the level's seed and leaves the generator's own RNG alone.
func (l *Levels) CreateLevel(gnr *rules.Generator, q *rules.Quests, seeds [rules.NumLevels]int32, dlvl, setLevel int, entry Entry) (lvl *Level, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("creating level %d (set level %d): %v", dlvl, setLevel, r)
		}
	}()
	cfg := l.cfg[dlvl]
	st := strategyNamed(cfg.strategy)
	g := &gen{quests: q, currlevel: dlvl, ltype: st.ltype, setlevel: setLevel != rules.SLNone, setLvlNum: setLevel,
		strat: st, params: cfg.params}
	g.data, g.levels = gnr.D, l
	// a level rebuilds from its seed (save/load), so it can't see uniques dropped since its first visit: it rolls with
	// none dropped and only reports back the ones it placed. A unique already found can reappear on a new level.
	g.igen = &rules.Generator{D: gnr.D, R: &g.rng, UniqueDropped: make([]bool, len(gnr.UniqueDropped))}
	defer func() {
		for u, d := range g.igen.UniqueDropped {
			gnr.UniqueDropped[u] = gnr.UniqueDropped[u] || d
		}
	}()
	seed := seeds[dlvl]
	g.rng.SetSeed(seed)
	g.initLevelMonsters()
	if g.setlevel {
		g.ltype = setLevelType(q, setLevel)
		g.til = g.levels.til[g.ltype]
		g.sol = g.levels.pieceFlags[g.ltype]
		g.loadSetMap(setLevel, seeds)
		g.getLevelMTypes(setLevel)
		g.initMonsters(setLevel)
		g.initItems()
		g.resyncQuests()
	} else {
		g.createDungeon(seed, entry)
		g.initTriggers()
		g.freeUpstairs()
		g.sol = g.levels.pieceFlags[g.ltype]
		g.rng.SetSeed(seed)
		g.getLevelMTypes(rules.SLNone)
		g.initThemes()
		g.rng.SetSeed(seed)
		g.holdThemes()
		g.rng.Seed()
		g.initMonsters(rules.SLNone)
		g.rng.Seed()
		g.initObjects(seeds)
		g.hangTorchesOnWalls()
		g.moveMonstersOffObjects()
		g.initItems()
		g.createThemeRooms()
		g.resyncQuests()
	}
	return g.output(setLevel), nil
}

func setLevelType(q *rules.Quests, sl int) int {
	for i := range q {
		if q[i].SetLevel == sl {
			return q[i].LvlType
		}
	}
	return rules.DTypeCathedral
}

// createDungeon ports CreateDungeon for dungeon levels 1-16: it runs whichever generation strategy the mod named
// for this level.
func (g *gen) createDungeon(seed int32, entry Entry) {
	g.flags = [112][112]byte{}
	g.initTrans()
	g.setPiece, g.setPieceRoom = rect{}, rect{}
	g.til = g.levels.til[g.ltype]
	g.rng.SetSeed(seed)
	g.strat.build(g, entry)
	g.lPass3(g.strat.fill)
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

// tileFlags is what the tile of the piece at (x, y) blocks: nSolidTable, nBlockTable, nMissileTable and nTrapTable
// read off the megatile map parts.
func (g *gen) tileFlags(x, y int) hhm.Flags {
	if p := g.pieceAt(x, y); p < len(g.sol) {
		return g.sol[p]
	}
	return hhm.Flags{}
}

func (g *gen) solid(x, y int) bool { return g.tileFlags(x, y).BlocksMovement }

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
			case rules.DTypeCathedral:
				if p == 129 {
					add(i, j, TrigPrevLevel)
				}
				if p == 115 {
					add(i, j, TrigNextLevel)
				}
			case rules.DTypeCatacombs:
				if p == 267 && (i != g.quests[rules.QSchamb].X || j != g.quests[rules.QSchamb].Y) {
					add(i, j, TrigPrevLevel)
				}
				if p == 559 {
					add(i, j, TrigTownWarpUp)
				}
				if p == 271 {
					add(i, j, TrigNextLevel)
				}
			case rules.DTypeCaves:
				if p == 171 {
					add(i, j, TrigPrevLevel)
				}
				if p == 168 {
					add(i, j, TrigNextLevel)
				}
				if p == 549 {
					add(i, j, TrigTownWarpUp)
				}
			case rules.DTypeHell:
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
	if g.ltype == rules.DTypeHell && g.quests[rules.QBetrayer].Active == rules.QuestDone {
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
				cx, cy := rules.VCrawlTable[j][k], rules.VCrawlTable[j][k+1]
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
				blocked = g.tileFlags(x, y).BlocksView
				if !g.tileFlags(x+x1adj, y+y1adj).BlocksView || !g.tileFlags(x+x2adj, y+y2adj).BlocksView {
					visible(x, y)
				}
			}
		}
	}
}

func (g *gen) output(setLevel int) *Level {
	l := &Level{Dlvl: g.currlevel, Type: g.ltype, SetLevel: setLevel, Piece: g.piece, Trans: g.trans, Flags: g.flags,
		PieceFlags: g.sol, parts: g.levels.parts[g.ltype], til: g.til, View: [2]int{g.view.x, g.view.y}, Triggers: g.trigs,
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
