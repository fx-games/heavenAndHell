package d1

import "math/rand"

// Cell is one cell of a generated level. D1 megatiles (40×40) become 2×2 cells (80×80);
// walls sit on a megatile's top/left edge, like D1 draws them.
type Cell byte

const (
	CellWall Cell = iota
	CellFloor
	CellDoor
)

const LevelSize = 80

type Level struct {
	Cells              [LevelSize][LevelSize]Cell
	StairsUp, StairsDn [2]int                     // x, y
	Trans              [LevelSize][LevelSize]int  // D1's dTransVal: room regions (0 = none)
	Held               [LevelSize][LevelSize]bool // dFlags & 8: reserved for a theme room
}

func (l *Level) Walkable(x, y int) bool {
	return x >= 0 && y >= 0 && x < LevelSize && y < LevelSize && l.Cells[x][y] != CellWall
}

// l1 holds drlg_l1.cpp's globals.
type l1 struct {
	rng           *rand.Rand
	dungeon       [40][40]byte
	flags         [40][40]byte
	fine          [80][80]byte // L5dungeon
	hr1, hr2, hr3 int
	vr1, vr2, vr3 int
	trans         [112][112]int // dTransVal in dPiece coordinates (megatile i covers 16+2i..17+2i)
	transVal      int
}

func (g *l1) random(v int) int {
	if v <= 0 {
		return 0
	}
	return g.rng.Intn(v)
}

// at reads dungeon with D1's out-of-bounds reads turned into void.
func (g *l1) at(i, j int) byte {
	if i < 0 || j < 0 || i >= 40 || j >= 40 {
		return 0
	}
	return g.dungeon[i][j]
}

func (g *l1) set(i, j int, v byte) {
	if i >= 0 && j >= 0 && i < 40 && j < 40 {
		g.dungeon[i][j] = v
	}
}

func (g *l1) flag(i, j int) byte {
	if i < 0 || j < 0 || i >= 40 || j >= 40 {
		return 0
	}
	return g.flags[i][j]
}

func (g *l1) drawRoom(x, y, w, h int) {
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			g.set(x+i, y+j, 1)
		}
	}
}

func (g *l1) checkRoom(x, y, w, h int) bool {
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			if i+x < 0 || i+x >= 40 || j+y < 0 || j+y >= 40 || g.dungeon[i+x][j+y] != 0 {
				return false
			}
		}
	}
	return true
}

func (g *l1) roomGen(x, y, w, h, dir int) {
	dirProb := g.random(4)
	horiz := dirProb == 0
	if dir == 1 {
		horiz = dirProb != 0
	}
	if !horiz {
		var cw, ch, cy1, cx1 int
		ran := false
		for num := 0; !ran && num < 20; num++ {
			cw = (g.random(5) + 2) &^ 1
			ch = (g.random(5) + 2) &^ 1
			cy1 = h/2 + y - ch/2
			cx1 = x - cw
			ran = g.checkRoom(cx1-1, cy1-1, ch+2, cw+1) // D1 bug: swapped args kept
		}
		if ran {
			g.drawRoom(cx1, cy1, cw, ch)
		}
		cx2 := x + w
		ran2 := g.checkRoom(cx2, cy1-1, cw+1, ch+2)
		if ran2 {
			g.drawRoom(cx2, cy1, cw, ch)
		}
		if ran {
			g.roomGen(cx1, cy1, cw, ch, 1)
		}
		if ran2 {
			g.roomGen(cx2, cy1, cw, ch, 1)
		}
		return
	}
	var width, height, rx, ry int
	ran := false
	for num := 0; !ran && num < 20; num++ {
		width = (g.random(5) + 2) &^ 1
		height = (g.random(5) + 2) &^ 1
		rx = w/2 + x - width/2
		ry = y - height
		ran = g.checkRoom(rx-1, ry-1, width+2, height+1)
	}
	if ran {
		g.drawRoom(rx, ry, width, height)
	}
	ry2 := y + h
	ran2 := g.checkRoom(rx-1, ry2, width+2, height+1)
	if ran2 {
		g.drawRoom(rx, ry2, width, height)
	}
	if ran {
		g.roomGen(rx, ry, width, height, 0)
	}
	if ran2 {
		g.roomGen(rx, ry2, width, height, 0)
	}
}

func (g *l1) firstRoom() {
	if g.random(2) == 0 {
		ys, ye := 1, 39
		g.vr1, g.vr2, g.vr3 = g.random(2), g.random(2), g.random(2)
		if g.vr1+g.vr3 <= 1 {
			g.vr2 = 1
		}
		if g.vr1 != 0 {
			g.drawRoom(15, 1, 10, 10)
		} else {
			ys = 18
		}
		if g.vr2 != 0 {
			g.drawRoom(15, 15, 10, 10)
		}
		if g.vr3 != 0 {
			g.drawRoom(15, 29, 10, 10)
		} else {
			ye = 22
		}
		for y := ys; y < ye; y++ {
			for x := 17; x <= 22; x++ {
				g.dungeon[x][y] = 1
			}
		}
		if g.vr1 != 0 {
			g.roomGen(15, 1, 10, 10, 0)
		}
		if g.vr2 != 0 {
			g.roomGen(15, 15, 10, 10, 0)
		}
		if g.vr3 != 0 {
			g.roomGen(15, 29, 10, 10, 0)
		}
		g.hr1, g.hr2, g.hr3 = 0, 0, 0
		return
	}
	xs, xe := 1, 39
	g.hr1, g.hr2, g.hr3 = g.random(2), g.random(2), g.random(2)
	if g.hr1+g.hr3 <= 1 {
		g.hr2 = 1
	}
	if g.hr1 != 0 {
		g.drawRoom(1, 15, 10, 10)
	} else {
		xs = 18
	}
	if g.hr2 != 0 {
		g.drawRoom(15, 15, 10, 10)
	}
	if g.hr3 != 0 {
		g.drawRoom(29, 15, 10, 10)
	} else {
		xe = 22
	}
	for x := xs; x < xe; x++ {
		for y := 17; y <= 22; y++ {
			g.dungeon[x][y] = 1
		}
	}
	if g.hr1 != 0 {
		g.roomGen(1, 15, 10, 10, 1)
	}
	if g.hr2 != 0 {
		g.roomGen(15, 15, 10, 10, 1)
	}
	if g.hr3 != 0 {
		g.roomGen(29, 15, 10, 10, 1)
	}
	g.vr1, g.vr2, g.vr3 = 0, 0, 0
}

func (g *l1) area() (rv int) {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.dungeon[i][j] == 1 {
				rv++
			}
		}
	}
	return
}

var l5ConvTbl = [16]byte{22, 13, 1, 13, 2, 13, 13, 13, 4, 13, 1, 13, 2, 13, 16, 13}

func (g *l1) makeDungeon() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			v := g.dungeon[i][j]
			g.fine[2*i][2*j], g.fine[2*i][2*j+1], g.fine[2*i+1][2*j], g.fine[2*i+1][2*j+1] = v, v, v, v
		}
	}
}

func (g *l1) makeDmt() {
	for j, dy := 0, 1; dy <= 77; j, dy = j+1, dy+2 {
		for i, dx := 0, 1; dx <= 77; i, dx = i+1, dx+2 {
			val := int(g.fine[dx+1][dy+1])
			val = 2*val + int(g.fine[dx][dy+1])
			val = 2*val + int(g.fine[dx+1][dy])
			val = 2*val + int(g.fine[dx][dy])
			g.dungeon[i][j] = l5ConvTbl[val]
		}
	}
	for i := 0; i < 40; i++ { // row/column 39 is never sampled by the loop above
		g.dungeon[i][39], g.dungeon[39][i] = 22, 22
	}
}

func (g *l1) gChamber(sx, sy int, top, bottom, left, right bool) {
	if top {
		g.set(sx+2, sy, 12)
		g.set(sx+3, sy, 12)
		g.set(sx+4, sy, 3)
		g.set(sx+7, sy, 9)
		g.set(sx+8, sy, 12)
		g.set(sx+9, sy, 2)
	}
	if bottom {
		sy += 11
		g.set(sx+2, sy, 10)
		g.set(sx+3, sy, 12)
		g.set(sx+4, sy, 8)
		g.set(sx+7, sy, 5)
		g.set(sx+8, sy, 12)
		if g.at(sx+9, sy) != 4 {
			g.set(sx+9, sy, 21)
		}
		sy -= 11
	}
	if left {
		g.set(sx, sy+2, 11)
		g.set(sx, sy+3, 11)
		g.set(sx, sy+4, 3)
		g.set(sx, sy+7, 8)
		g.set(sx, sy+8, 11)
		g.set(sx, sy+9, 1)
	}
	if right {
		sx += 11
		g.set(sx, sy+2, 14)
		g.set(sx, sy+3, 11)
		g.set(sx, sy+4, 9)
		g.set(sx, sy+7, 5)
		g.set(sx, sy+8, 11)
		if g.at(sx, sy+9) != 4 {
			g.set(sx, sy+9, 21)
		}
		sx -= 11
	}
	for j := 1; j < 11; j++ {
		for i := 1; i < 11; i++ {
			g.set(i+sx, j+sy, 13)
			if i+sx < 40 && j+sy < 40 {
				g.flags[i+sx][j+sy] |= 0x40
			}
		}
	}
	g.set(sx+4, sy+4, 15)
	g.set(sx+7, sy+4, 15)
	g.set(sx+4, sy+7, 15)
	g.set(sx+7, sy+7, 15)
}

func (g *l1) gHall(x1, y1, x2, y2 int) {
	if y1 == y2 {
		for i := x1; i < x2; i++ {
			g.set(i, y1, 12)
			g.set(i, y1+3, 12)
		}
		return
	}
	for i := y1; i < y2; i++ {
		g.set(x1, i, 11)
		g.set(x1+3, i, 11)
	}
}

func (g *l1) fillChambers() {
	b := func(v int) bool { return v != 0 }
	if b(g.hr1) {
		g.gChamber(0, 14, false, false, false, true)
	}
	if b(g.hr2) {
		g.gChamber(14, 14, false, false, b(g.hr1), b(g.hr3))
	}
	if b(g.hr3) {
		g.gChamber(28, 14, false, false, true, false)
	}
	if b(g.hr1) && b(g.hr2) {
		g.gHall(12, 18, 14, 18)
	}
	if b(g.hr2) && b(g.hr3) {
		g.gHall(26, 18, 28, 18)
	}
	if b(g.hr1) && !b(g.hr2) && b(g.hr3) {
		g.gHall(12, 18, 28, 18)
	}
	if b(g.vr1) {
		g.gChamber(14, 0, false, true, false, false)
	}
	if b(g.vr2) {
		g.gChamber(14, 14, b(g.vr1), b(g.vr3), false, false)
	}
	if b(g.vr3) {
		g.gChamber(14, 28, true, false, false, false)
	}
	if b(g.vr1) && b(g.vr2) {
		g.gHall(18, 12, 18, 14)
	}
	if b(g.vr2) && b(g.vr3) {
		g.gHall(18, 26, 18, 28)
	}
	if b(g.vr1) && !b(g.vr2) && b(g.vr3) {
		g.gHall(18, 12, 18, 28)
	}
	// ponytail: set pieces (Butcher, Leoric entrance) hook in here once milestone 7 needs them.
}

// tileFix ports L5tileFix: neighbour rules, each (a at [i][j], b at offset) → new b.
type fixRule struct {
	a, b   byte
	dx, dy int
	nb     byte
}

var tileFix1 = []fixRule{
	{2, 22, 1, 0, 23}, {13, 22, 1, 0, 18}, {13, 2, 1, 0, 7}, {6, 22, 1, 0, 24},
	{1, 22, 0, 1, 24}, {13, 1, 0, 1, 6}, {13, 22, 0, 1, 19},
}
var tileFix2 = []fixRule{
	{13, 19, 1, 0, 21}, {13, 22, 1, 0, 20}, {7, 22, 1, 0, 23}, {13, 24, 1, 0, 21}, {19, 22, 1, 0, 20},
	{2, 19, 1, 0, 21}, {19, 1, 1, 0, 6}, {7, 19, 1, 0, 21}, {2, 1, 1, 0, 6}, {3, 22, 1, 0, 24},
	{21, 1, 1, 0, 6}, {7, 1, 1, 0, 6}, {7, 24, 1, 0, 21}, {4, 16, 1, 0, 17}, {7, 13, 1, 0, 17},
	{2, 24, 1, 0, 21}, {2, 13, 1, 0, 17}, {23, 22, -1, 0, 19}, {19, 23, -1, 0, 21}, {6, 22, -1, 0, 24},
	{6, 23, -1, 0, 21}, {1, 2, 0, 1, 7}, {6, 18, 0, 1, 21}, {18, 2, 0, 1, 7}, {6, 2, 0, 1, 7},
	{21, 2, 0, 1, 7}, {6, 22, 0, 1, 24}, {6, 13, 0, 1, 16}, {1, 13, 0, 1, 16}, {13, 16, 0, 1, 17},
	{6, 22, 0, -1, 7}, {6, 22, 0, -1, 24}, {7, 24, 0, -1, 21}, {18, 24, 0, -1, 21},
}
var tileFix3 = []fixRule{{4, 2, 0, 1, 7}, {2, 19, 1, 0, 21}, {18, 22, 0, 1, 20}}

func (g *l1) tileFix() {
	for _, pass := range [][]fixRule{tileFix1, tileFix2, tileFix3} {
		for j := 0; j < 40; j++ {
			for i := 0; i < 40; i++ {
				for _, r := range pass {
					if g.at(i, j) == r.a && g.at(i+r.dx, j+r.dy) == r.b {
						g.set(i+r.dx, j+r.dy, r.nb)
					}
				}
			}
		}
	}
}

func wallEnd(c byte) bool { return c >= 3 && c <= 7 || c >= 16 && c <= 24 && c != 22 }

func (g *l1) hWallOk(i, j int) int {
	x := 1
	for ; g.at(i+x, j) == 13; x++ {
		if g.at(i+x, j-1) != 13 || g.at(i+x, j+1) != 13 || g.flag(i+x, j) != 0 {
			break
		}
	}
	if x == 1 || !wallEnd(g.at(i+x, j)) {
		return -1
	}
	return x
}

func (g *l1) vWallOk(i, j int) int {
	y := 1
	for ; g.at(i, j+y) == 13; y++ {
		if g.at(i-1, j+y) != 13 || g.at(i+1, j+y) != 13 || g.flag(i, j+y) != 0 {
			break
		}
	}
	if y == 1 || !wallEnd(g.at(i, j+y)) {
		return -1
	}
	return y
}

func (g *l1) horizWall(i, j int, p byte, dx int) {
	var dt byte
	switch g.random(4) {
	case 0, 1:
		dt = 2
	case 2:
		dt = 12
		if p == 2 {
			p = 12
		}
		if p == 4 {
			p = 10
		}
	case 3:
		dt = 36
		if p == 2 {
			p = 36
		}
		if p == 4 {
			p = 27
		}
	}
	wt := byte(26)
	if g.random(6) == 5 || dt == 12 {
		wt = 12
	}
	g.dungeon[i][j] = p
	for xx := 1; xx < dx; xx++ {
		g.dungeon[i+xx][j] = dt
	}
	xx := g.random(dx-1) + 1
	if wt == 12 {
		g.dungeon[i+xx][j] = wt
	} else {
		g.dungeon[i+xx][j] = 2
		g.flags[i+xx][j] |= 1
	}
}

func (g *l1) vertWall(i, j int, p byte, dy int) {
	var dt byte
	switch g.random(4) {
	case 0, 1:
		dt = 1
	case 2:
		dt = 11
		if p == 1 {
			p = 11
		}
		if p == 4 {
			p = 14
		}
	case 3:
		dt = 35
		if p == 1 {
			p = 35
		}
		if p == 4 {
			p = 37
		}
	}
	wt := byte(25)
	if g.random(6) == 5 || dt == 11 {
		wt = 11
	}
	g.dungeon[i][j] = p
	for yy := 1; yy < dy; yy++ {
		g.dungeon[i][j+yy] = dt
	}
	yy := g.random(dy-1) + 1
	if wt == 11 {
		g.dungeon[i][j+yy] = wt
	} else {
		g.dungeon[i][j+yy] = 1
		g.flags[i][j+yy] |= 2
	}
}

func (g *l1) addWall() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.flags[i][j] != 0 {
				continue
			}
			// D1 rolls random(0,100) < 100 before each check; always true, but it consumes RNG.
			if g.dungeon[i][j] == 3 && g.random(100) < 100 {
				if x := g.hWallOk(i, j); x != -1 {
					g.horizWall(i, j, 2, x)
				}
			}
			if g.dungeon[i][j] == 3 && g.random(100) < 100 {
				if y := g.vWallOk(i, j); y != -1 {
					g.vertWall(i, j, 1, y)
				}
			}
			if g.dungeon[i][j] == 6 && g.random(100) < 100 {
				if x := g.hWallOk(i, j); x != -1 {
					g.horizWall(i, j, 4, x)
				}
			}
			if g.dungeon[i][j] == 7 && g.random(100) < 100 {
				if y := g.vWallOk(i, j); y != -1 {
					g.vertWall(i, j, 4, y)
				}
			}
			if g.dungeon[i][j] == 2 && g.random(100) < 100 {
				if x := g.hWallOk(i, j); x != -1 {
					g.horizWall(i, j, 2, x)
				}
			}
			if g.dungeon[i][j] == 1 && g.random(100) < 100 {
				if y := g.vWallOk(i, j); y != -1 {
					g.vertWall(i, j, 1, y)
				}
			}
		}
	}
}

var (
	l5StairsUp = []byte{4, 4, 22, 22, 22, 22, 2, 2, 2, 2, 13, 13, 13, 13, 13, 13, 13, 13, 0, 66, 23, 0, 63, 64, 65, 0, 0, 67, 68, 0, 0, 0, 0, 0}
	stairsDown = []byte{4, 3, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 13, 62, 57, 58, 0, 61, 59, 60, 0, 0, 0, 0, 0}
)

// placeMiniSet ports DRLG_PlaceMiniSet for a single placement without quadrant rules; returns sx, sy or -1.
func (g *l1) placeMiniSet(m []byte) (int, int) {
	sw, sh := int(m[0]), int(m[1])
	sx, sy := g.random(40-sw), g.random(40-sh)
	for found := 0; ; {
		ok := true
		ii := 2
		for yy := 0; yy < sh && ok; yy++ {
			for xx := 0; xx < sw && ok; xx++ {
				if m[ii] != 0 && g.dungeon[xx+sx][sy+yy] != m[ii] || g.flags[xx+sx][sy+yy] != 0 {
					ok = false
				}
				ii++
			}
		}
		if ok {
			break
		}
		if sx++; sx == 40-sw {
			sx = 0
			if sy++; sy == 40-sh {
				sy = 0
			}
		}
		if found++; found > 4000 {
			return -1, -1
		}
	}
	ii := sw*sh + 2
	for yy := 0; yy < sh; yy++ {
		for xx := 0; xx < sw; xx++ {
			if m[ii] != 0 {
				g.dungeon[xx+sx][sy+yy] = m[ii]
			}
			ii++
		}
	}
	return sx, sy
}

func (g *l1) setTrans(x, y int) {
	if x >= 0 && y >= 0 && x < 112 && y < 112 {
		g.trans[x][y] = g.transVal
	}
}

// ftvr ports DRLG_L5FTVR: flood a room region over floor megatiles, marking the wall edges it touches.
func (g *l1) ftvr(i, j, x, y, d int) {
	if x < 0 || y < 0 || x > 110 || y > 110 {
		return
	}
	if g.trans[x][y] != 0 || g.at(i, j) != 13 {
		switch d {
		case 1:
			g.setTrans(x, y)
			g.setTrans(x, y+1)
		case 2:
			g.setTrans(x+1, y)
			g.setTrans(x+1, y+1)
		case 3:
			g.setTrans(x, y)
			g.setTrans(x+1, y)
		case 4:
			g.setTrans(x, y+1)
			g.setTrans(x+1, y+1)
		case 5:
			g.setTrans(x+1, y+1)
		case 6:
			g.setTrans(x, y+1)
		case 7:
			g.setTrans(x+1, y)
		case 8:
			g.setTrans(x, y)
		}
		return
	}
	g.setTrans(x, y)
	g.setTrans(x+1, y)
	g.setTrans(x, y+1)
	g.setTrans(x+1, y+1)
	g.ftvr(i+1, j, x+2, y, 1)
	g.ftvr(i-1, j, x-2, y, 2)
	g.ftvr(i, j+1, x, y+2, 3)
	g.ftvr(i, j-1, x, y-2, 4)
	g.ftvr(i-1, j-1, x-2, y-2, 5)
	g.ftvr(i+1, j-1, x+2, y-2, 6)
	g.ftvr(i-1, j+1, x-2, y+2, 7)
	g.ftvr(i+1, j+1, x+2, y+2, 8)
}

// floodTVal ports DRLG_L5FloodTVal.
func (g *l1) floodTVal() {
	g.transVal = 1
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.dungeon[i][j] == 13 && g.trans[16+2*i][16+2*j] == 0 {
				g.ftvr(i, j, 16+2*i, 16+2*j, 0)
				g.transVal++
			}
		}
	}
}

// transFix ports the stair tile copy and DRLG_L5TransFix.
func (g *l1) transFix() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			xx, yy := 16+2*i, 16+2*j
			t := &g.trans
			switch c := g.dungeon[i][j]; {
			case c == 64:
				t[xx][yy], t[xx+1][yy] = t[xx][yy+1], t[xx+1][yy+1]
			case c == 23 && g.at(i, j-1) == 18, c == 18:
				t[xx+1][yy], t[xx+1][yy+1] = t[xx][yy], t[xx][yy]
			case c == 24 && g.at(i+1, j) == 19, c == 19:
				t[xx][yy+1], t[xx+1][yy+1] = t[xx][yy], t[xx][yy]
			case c == 20:
				t[xx+1][yy], t[xx][yy+1], t[xx+1][yy+1] = t[xx][yy], t[xx][yy], t[xx][yy]
			}
		}
	}
}

// overlayWalls: cells (top-left, top-right, bottom-left, bottom-right) that a megatile's own walls/arches block
// on top of the fine room mask. Walls sit on a megatile's top/left edge, like D1 draws them.
func overlayWalls(c byte) [4]bool {
	switch c {
	case 2, 7, 26, 36: // top edge wall (26 = door, 36 = wall variant)
		return [4]bool{true, true, false, false}
	case 1, 6, 25, 35: // left edge wall (25 = door, 35 = wall variant)
		return [4]bool{true, false, true, false}
	case 3, 4, 27, 37: // top + left (corner posts)
		return [4]bool{true, true, true, false}
	case 5, 8, 9, 10, 11, 12, 14, 15, 16, 17: // pillars, arches, wall ends
		return [4]bool{true, false, false, false}
	}
	return [4]bool{}
}

// GenerateL1 ports DRLG_L5 (Cathedral) down to the floor/wall/door grid.
func GenerateL1(rng *rand.Rand, dlvl int) *Level {
	minarea := map[int]int{1: 533, 2: 693, 3: 761, 4: 761}[dlvl]
	if minarea == 0 {
		minarea = 761
	}
	for {
		g := &l1{rng: rng}
		for {
			g.dungeon = [40][40]byte{}
			g.flags = [40][40]byte{}
			g.firstRoom()
			if g.area() >= minarea {
				break
			}
		}
		g.makeDungeon()
		g.makeDmt()
		g.fillChambers()
		g.tileFix()
		g.addWall()
		for j := range g.flags {
			for i := range g.flags[j] {
				g.flags[j][i] &= 0xBF
			}
		}
		g.trans = [112][112]int{}
		g.floodTVal()
		ux, uy := g.placeMiniSet(l5StairsUp)
		if ux < 0 {
			continue
		}
		dx, dy := g.placeMiniSet(stairsDown)
		if dx < 0 {
			continue
		}
		g.transFix()
		lvl := &Level{}
		for x := 0; x < LevelSize; x++ {
			for y := 0; y < LevelSize; y++ {
				lvl.Trans[x][y] = g.trans[x+16][y+16]
			}
		}
		// megatile (i,j) samples L5dungeon[2i+1..2i+2][2j+1..2j+2]; its walls sit on its top/left edge, so a room's
		// floor reaches one cell past the L5dungeon room on the right/bottom: a cell is floor if any of
		// L5dungeon[x..x+1][y..y+1] is, and the megatile overlays below put the walls back.
		for x := 0; x < LevelSize-1; x++ {
			for y := 0; y < LevelSize-1; y++ {
				if g.fine[x][y]|g.fine[x+1][y]|g.fine[x][y+1]|g.fine[x+1][y+1] != 0 {
					lvl.Cells[x][y] = CellFloor
				}
			}
		}
		for j := 0; j < 40; j++ {
			for i := 0; i < 40; i++ {
				c := g.dungeon[i][j]
				for n, w := range overlayWalls(c) {
					if w {
						lvl.Cells[2*i+n%2][2*j+n/2] = CellWall
					}
				}
				if g.flags[i][j]&3 != 0 && (c == 1 || c == 2) {
					lvl.Cells[2*i][2*j] = CellDoor
				}
			}
		}
		lvl.StairsUp = [2]int{2*ux + 3, 2*uy + 3}
		lvl.StairsDn = [2]int{2*dx + 3, 2*dy + 3}
		return lvl
	}
}
