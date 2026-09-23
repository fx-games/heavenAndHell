package dungen

import (
	"rpg/src/hhm"
	"rpg/src/rules"
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

// Level is a laid-out dungeon or set level as D1's rules populated it (Populate), in D1's 112x112 piece
// coordinates: the playable cells 0..79 are pieces 16..95.
type Level struct {
	Dlvl, Type, SetLevel int
	Trans                [112][112]int8 // room regions (dTransVal), for the AIs' same-room checks
	Triggers             []Trigger
	SetPiece             [4]int // the first set piece: x, y, w, h in megatiles
	Monsters             []*Monster
	Objects              []*rules.Object
	Items                []*PlacedItem
	Pieces               []Stamped // its set pieces, which open cell by cell (OpenCells)
	opened               map[[2]int]bool
}

// OpenCell is a cell of a set piece that switched to the piece's open layout: its new tile, or a door.
type OpenCell struct {
	X, Y, Tile int
	Door       bool
	Object     string // an object the open layout places here (Blood2's torches), "" for none
}

// OpenCells ports ObjChangeMap on a laid-out level: the cells of megatiles (x1, y1)..(x2, y2) that a set piece
// draws differently in its open layout switch to it. Each cell switches once; the cells that did are returned.
func (l *Level) OpenCells(x1, y1, x2, y2 int) []OpenCell {
	var out []OpenCell
	for _, p := range l.Pieces {
		if p.Open == nil {
			continue
		}
		doors, objs := map[[2]int]bool{}, map[[2]int]string{}
		for _, e := range p.Open.Entities {
			switch e.Kind {
			case "Door":
				doors[[2]int{e.X, e.Y}] = true
			case "Object":
				objs[[2]int{e.X, e.Y}] = e.Str("ID")
			}
		}
		for y := max(2*y1, p.Y); y <= min(2*y2+1, p.Y+p.Open.Height-1); y++ {
			for x := max(2*x1, p.X); x <= min(2*x2+1, p.X+p.Open.Width-1); x++ {
				lx, ly := x-p.X, y-p.Y
				t, door, obj := p.Open.TileAt(lx, ly), doors[[2]int{lx, ly}], objs[[2]int{lx, ly}]
				if l.opened[[2]int{x, y}] || t == p.Map.TileAt(lx, ly) && !door && obj == "" {
					continue
				}
				if l.opened == nil {
					l.opened = map[[2]int]bool{}
				}
				l.opened[[2]int{x, y}] = true
				out = append(out, OpenCell{x, y, t, door, obj})
			}
		}
	}
	return out
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

// tileFlags is what the tile of the piece at (x, y) blocks: nSolidTable, nBlockTable, nMissileTable and nTrapTable
// read off the megatile map parts.
func (g *gen) tileFlags(x, y int) hhm.Flags { return g.cellFlags(x, y) }

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
	l := &Level{Dlvl: g.currlevel, Type: g.ltype, SetLevel: setLevel, Trans: g.trans, Triggers: g.trigs,
		SetPiece: [4]int{g.setPiece.x, g.setPiece.y, g.setPiece.w, g.setPiece.h}, Pieces: g.cells.pieces, Monsters: g.monsters}
	for _, oi := range g.objectactive[:g.nobjects] {
		l.Objects = append(l.Objects, g.objects[oi])
	}
	for _, ii := range g.itemactive[:g.numitems] {
		l.Items = append(l.Items, g.itemSlots[ii])
	}
	return l
}
