package dungen

import "rpg/src/rules"

type point struct{ x, y int }

type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool { return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h }

type themeLoc struct {
	room  rect
	ttval int8
}

// gen holds gendung.cpp's globals while a laid-out level is populated: 112x112 pieces (dMonster etc.), the
// playable cells at 16..95, and D1's 40x40 megatiles as the 2x2 cells they cover.
type gen struct {
	rng           rules.Rng
	quests        *rules.Quests
	currlevel     int
	ltype         int
	libraryStands int      // Population.LibraryStands
	only          []string // Population.Monsters
	setlevel      bool
	setLvlNum     int
	cells         *DungeonMap // the level as the passes laid it out

	population

	trans    [112][112]int8
	flags    [112][112]byte
	transVal int8

	setPiece rect
	themes   []themeLoc
}

func (g *gen) rnd(v int) int { return g.rng.Random(v) }

func (g *gen) setTrans(x, y int, v int8) {
	if x >= 0 && y >= 0 && x < 112 && y < 112 {
		g.trans[x][y] = v
	}
}

func (g *gen) getTrans(x, y int) int8 {
	if x >= 0 && y >= 0 && x < 112 && y < 112 {
		return g.trans[x][y]
	}
	return 0
}

func (g *gen) initTrans() {
	g.trans = [112][112]int8{}
	g.transVal = 1
}

// rectTrans ports DRLG_RectTrans (inclusive piece rectangle).
func (g *gen) rectTrans(x1, y1, x2, y2 int) {
	for j := y1; j <= y2; j++ {
		for i := x1; i <= x2; i++ {
			g.setTrans(i, j, g.transVal)
		}
	}
	g.transVal++
}

// mRectTrans ports DRLG_MRectTrans (megatile corners).
func (g *gen) mRectTrans(x1, y1, x2, y2 int) {
	g.rectTrans(2*x1+17, 2*y1+17, 2*x2+16, 2*y2+16)
}

// makeSetPC ports Make_SetPC: mark a megatile rectangle populated (monsters stay out).
func (g *gen) makeSetPC(r rect) {
	for j := 0; j < 2*r.h; j++ {
		for i := 0; i < 2*r.w; i++ {
			if x, y := 2*r.x+16+i, 2*r.y+16+j; x >= 0 && y >= 0 && x < 112 && y < 112 {
				g.flags[x][y] |= bflagPopulated
			}
		}
	}
}

const (
	bflagMissile   = 0x01
	bflagVisible   = 0x02
	bflagDeadPlr   = 0x04
	bflagPopulated = 0x08
	bflagMonstLr   = 0x10
	bflagPlayerLr  = 0x20
	bflagLit       = 0x40
	bflagExplored  = 0x80
)

func (g *gen) nearThemeRoom(x, y int) bool {
	for _, t := range g.themes {
		if (rect{t.room.x - 2, t.room.y - 2, t.room.w + 5, t.room.h + 5}).contains(x, y) {
			return true
		}
	}
	return false
}

// holdThemeRooms ports DRLG_HoldThemeRooms.
func (g *gen) holdThemeRooms() {
	for _, t := range g.themes {
		for y := t.room.y; y < t.room.y+t.room.h-1; y++ {
			for x := t.room.x; x < t.room.x+t.room.w-1; x++ {
				xx, yy := 2*x+16, 2*y+16
				g.flags[xx][yy] |= bflagPopulated
				g.flags[xx+1][yy] |= bflagPopulated
				g.flags[xx][yy+1] |= bflagPopulated
				g.flags[xx+1][yy+1] |= bflagPopulated
			}
		}
	}
}

// findTrans ports FindTransparencyValues' span fill.
func (g *gen) findTrans(fx, fy int) {
	type seed struct{ scanStart, scanEnd, y, dy int }
	stack := []seed{{fx, fx + 1, fy, 1}}
	inside := func(x, y int) bool { return g.getTrans(x, y) == 0 && g.cellFloor(x, y) }
	set := func(x, y int) {
		for _, d := range [8]point{{0, 1}, {0, -1}, {1, 0}, {-1, 0}, {1, -1}, {-1, -1}, {1, 1}, {-1, 1}} {
			if !g.cellFloor(x+d.x, y+d.y) {
				g.setTrans(x+d.x, y+d.y, g.transVal)
			}
		}
		g.setTrans(x, y, g.transVal)
	}
	checkDiagonals := func(x, y, dir int) {
		if !inside(x, y-1) && inside(x+dir, y-1) {
			stack = append(stack, seed{x + dir, x + dir + 1, y - 1, -1})
		}
		if !inside(x, y+1) && inside(x+dir, y+1) {
			stack = append(stack, seed{x + dir, x + dir + 1, y + 1, 1})
		}
	}
	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		scanLeft := s.scanStart
		if inside(scanLeft, s.y) {
			for inside(scanLeft-1, s.y) {
				set(scanLeft-1, s.y)
				scanLeft--
			}
			checkDiagonals(scanLeft, s.y, -1)
		}
		if scanLeft < s.scanStart {
			stack = append(stack, seed{scanLeft, s.scanStart - 1, s.y - s.dy, -s.dy})
		}
		scanRight := s.scanStart
		for scanRight < s.scanEnd {
			for inside(scanRight, s.y) {
				set(scanRight, s.y)
				scanRight++
			}
			stack = append(stack, seed{scanLeft, scanRight - 1, s.y + s.dy, s.dy})
			if scanRight-1 > s.scanEnd {
				stack = append(stack, seed{s.scanEnd + 1, scanRight - 1, s.y - s.dy, -s.dy})
			}
			if scanLeft < scanRight {
				checkDiagonals(scanRight-1, s.y, 1)
			}
			for scanRight < s.scanEnd && !inside(scanRight, s.y) {
				scanRight++
			}
			scanLeft = scanRight
			if scanLeft < s.scanEnd {
				checkDiagonals(scanLeft, s.y, -1)
			}
		}
	}
}
