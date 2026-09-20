package d1

import (
	"encoding/binary"
	"fmt"
)

// Level types (dungeon_type).
const (
	DTypeTown = iota
	DTypeCathedral
	DTypeCatacombs
	DTypeCaves
	DTypeHell
)

// Entry is how the player enters a level (lvl_entry); it decides where D1 puts them.
type Entry int

const (
	EntryMain    Entry = iota // from the level above
	EntryPrev                 // from the level below
	EntrySetLvl               // into a set level
	EntryRtnLvl               // back from a set level
	EntryLoad                 // loading a save
	EntryWarpLvl              // through a portal
	EntryTWarpDn              // from town through a warp
	EntryTWarpUp              // up to town through a warp
)

func LevelType(dlvl int) int {
	switch {
	case dlvl == 0:
		return DTypeTown
	case dlvl <= 4:
		return DTypeCathedral
	case dlvl <= 8:
		return DTypeCatacombs
	case dlvl <= 12:
		return DTypeCaves
	}
	return DTypeHell
}

// tileset is the level type whose til/sol a level draws with.
func (g *gen) tileset() int { return g.ltype }

type point struct{ x, y int }

type rect struct{ x, y, w, h int }

func (r rect) contains(x, y int) bool { return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h }

type themeLoc struct {
	room  rect
	ttval int8
}

// miniset is a tile pattern and its replacement, indexed [y][x].
type miniset struct {
	w, h            int
	search, replace [6][6]byte
}

// gen holds gendung.cpp's globals while one level is built. D1 levels are 40×40 megatiles (dungeon) and
// 112×112 pieces (dPiece etc.), megatile (i, j) covering pieces 16+2i..17+2i.
type gen struct {
	rng       Rng
	quests    *Quests
	currlevel int
	ltype     int
	setlevel  bool
	setLvlNum int

	population

	// dungeon and pdungeon share one block like D1's globals, so the generators' reads one past an edge see
	// what D1 sees (dungeon[40][y] is pdungeon[0][y]).
	mem       [3200]byte
	protected [1600]bool // Protected / L*dflags, index y*40+x
	mask      [1600]bool // DungeonMask, index y*40+x

	piece    [112][112]int // dPiece: 0-based micro index
	trans    [112][112]int8
	flags    [112][112]byte
	transVal int8

	setPiece, setPieceRoom rect
	view                   point
	themes                 []themeLoc
	til                    []byte

	l1 struct {
		vertical, ch1, ch2, ch3 bool
		chamber                 [1600]bool
	}
	l2 struct {
		rooms    [81]struct{ tl, br point }
		nRoomCnt int
		halls    []l2Hall
		pre      [40][40]byte
	}
	l3 struct{ lockoutcnt int }
	l4 struct {
		hallok [20]bool
		hold   point
		quads  [4]point
	}
}

func (g *gen) d(x, y int) byte {
	if i := x*40 + y; i >= 0 && i < len(g.mem) {
		return g.mem[i]
	}
	return 0
}

func (g *gen) sd(x, y int, v byte) {
	if i := x*40 + y; i >= 0 && i < len(g.mem) {
		g.mem[i] = v
	}
}

func (g *gen) pd(x, y int) byte { return g.d(x+40, y) }

func (g *gen) setPD(x, y int, v byte) {
	if x >= 0 && x < 40 {
		g.sd(x+40, y, v)
	}
}

func (g *gen) fillDungeon(v byte) {
	for i := 0; i < 1600; i++ {
		g.mem[i] = v
	}
}

func (g *gen) savePdungeon() { copy(g.mem[1600:], g.mem[:1600]) }

func bitIndex(x, y int) int {
	if x < 0 || y < 0 || x >= 40 || y >= 40 {
		return -1
	}
	return y*40 + x
}

func (g *gen) prot(x, y int) bool { i := bitIndex(x, y); return i >= 0 && g.protected[i] }

func (g *gen) setProt(x, y int) {
	if i := bitIndex(x, y); i >= 0 {
		g.protected[i] = true
	}
}

func (g *gen) masked(x, y int) bool { i := bitIndex(x, y); return i >= 0 && g.mask[i] }

func (g *gen) setMask(x, y int, v bool) {
	if i := bitIndex(x, y); i >= 0 {
		g.mask[i] = v
	}
}

func (g *gen) maskCount() (n int) {
	for _, b := range g.mask {
		if b {
			n++
		}
	}
	return
}

func (g *gen) rnd(v int) int   { return g.rng.Random(v) }
func (g *gen) flip(f int) bool { return g.rnd(f) == 0 }
func (g *gen) discard(n int) {
	for ; n > 0; n-- {
		g.rng.Seed()
	}
}
func (g *gen) pick(a, b int) int {
	if g.rnd(2) == 0 {
		return a
	}
	return b
}

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

func (g *gen) copyTrans(sx, sy, dx, dy int) { g.setTrans(dx, dy, g.getTrans(sx, sy)) }

func (m *miniset) matches(g *gen, x, y int, respectProtected bool) bool {
	for yy := 0; yy < m.h; yy++ {
		for xx := 0; xx < m.w; xx++ {
			if s := m.search[yy][xx]; s != 0 && g.d(xx+x, yy+y) != s {
				return false
			}
			if respectProtected && g.prot(xx+x, yy+y) {
				return false
			}
		}
	}
	return true
}

func (m *miniset) place(g *gen, x, y int, protect bool) {
	for yy := 0; yy < m.h; yy++ {
		for xx := 0; xx < m.w; xx++ {
			if r := m.replace[yy][xx]; r != 0 {
				g.sd(xx+x, yy+y, r)
				if protect {
					g.setProt(xx+x, yy+y)
				}
			}
		}
	}
}

// placeMiniSet ports PlaceMiniSet (gendung.cpp); ok is false when it doesn't fit anywhere.
func (g *gen) placeMiniSet(m *miniset, tries int, drlg1Quirk bool) (x, y int, ok bool) {
	sw, sh := m.w, m.h
	x, y = g.rnd(40-sw), g.rnd(40-sh)
	for i := 0; i < tries; i, x = i+1, x+1 {
		if x == 40-sw {
			x = 0
			if y++; y == 40-sh {
				y = 0
			}
		}
		if drlg1Quirk {
			valid := true
			if x <= 12 {
				x++
				valid = false
			}
			if y <= 12 {
				y++
				valid = false
			}
			if !valid {
				continue
			}
		}
		if g.setPieceRoom.contains(x, y) || !m.matches(g, x, y, true) {
			continue
		}
		m.place(g, x, y, false)
		return x, y, true
	}
	return 0, 0, false
}

// dun is a .DUN file: size, then layers of little-endian words (tiles; items, monsters, objects and
// transparency at double resolution).
type dun []uint16

func (d dun) w() int { return int(d[0]) }
func (d dun) h() int { return int(d[1]) }

func (d dun) tile(i, j int) byte { return byte(d[2+j*d.w()+i]) }

// layer returns a double-resolution layer value: 1 monsters, 2 objects, 3 transparency (items are layer 0).
func (d dun) layer(n, i, j int) int {
	off := 2 + d.w()*d.h() + n*4*d.w()*d.h()
	if idx := off + j*2*d.w() + i; idx < len(d) {
		return int(d[idx])
	}
	return 0
}

// loadDun returns a set piece by its name under assets/data/d1/levels, e.g. "l1/sklkng1".
func (g *gen) loadDun(name string) dun {
	if readDunHook != nil {
		if b := readDunHook(name); b != nil {
			return binDun(b)
		}
	}
	d, ok := g.data.levels.dun[name]
	if !ok {
		panic(fmt.Errorf("D1 level data: no set piece %q", name))
	}
	return d
}

// readDunHook lets tests supply set pieces as raw .DUN bytes; nil or a nil result uses the imported text.
var readDunHook func(name string) []byte

// placeDunTiles ports PlaceDunTiles: a set piece's tiles, protected; empty cells become floorID if set.
func (g *gen) placeDunTiles(d dun, x, y int, floorID byte) {
	for j := 0; j < d.h(); j++ {
		for i := 0; i < d.w(); i++ {
			if t := d.tile(i, j); t != 0 {
				g.sd(x+i, y+j, t)
				g.setProt(x+i, y+j)
			} else if floorID != 0 {
				g.sd(x+i, y+j, floorID)
			}
		}
	}
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

// sizeForThemeRoom ports GetSizeForThemeRoom (DRLG_WillThemeRoomFit, with its broken bounds check).
func (g *gen) sizeForThemeRoom(floor byte, x, y, minSize, maxSize int) (int, int, bool) {
	if x+maxSize > 40 && y+maxSize > 40 {
		return 0, 0, false
	}
	if g.nearThemeRoom(x, y) {
		return 0, 0, false
	}
	maxWidth, maxHeight := min(maxSize, 40-x), min(maxSize, 40-y)
	rw, rh := maxWidth, maxHeight
	for i := 0; i < maxSize; i++ {
		width := 0
		if i < rh {
			width = i
		}
		if i < maxHeight {
			for width < rw && g.d(x+width, y+i) == floor {
				width++
			}
		}
		height := 0
		if i < rw {
			height = i
		}
		if i < maxWidth {
			for height < rh && g.d(x+i, y+height) == floor {
				height++
			}
		}
		if width < minSize || height < minSize {
			if i < minSize {
				return 0, 0, false
			}
			break
		}
		rw, rh = min(rw, width), min(rh, height)
	}
	return rw - 2, rh - 2, true
}

func (g *gen) nearThemeRoom(x, y int) bool {
	for _, t := range g.themes {
		if (rect{t.room.x - 2, t.room.y - 2, t.room.w + 5, t.room.h + 5}).contains(x, y) {
			return true
		}
	}
	return false
}

// createThemeRoom ports CreateThemeRoom.
func (g *gen) createThemeRoom(t themeLoc) {
	lx, ly := t.room.x, t.room.y
	hx, hy := lx+t.room.w, ly+t.room.h
	wall := map[int][3]byte{DTypeCatacombs: {2, 1, 3}, DTypeCaves: {134, 137, 7}, DTypeHell: {2, 1, 6}}[g.ltype]
	for yy := ly; yy < hy; yy++ {
		for xx := lx; xx < hx; xx++ {
			switch {
			case yy == ly || yy == hy-1:
				g.sd(xx, yy, wall[0])
			case xx == lx || xx == hx-1:
				g.sd(xx, yy, wall[1])
			default:
				g.sd(xx, yy, wall[2])
			}
		}
	}
	switch g.ltype {
	case DTypeCatacombs:
		g.sd(lx, ly, 8)
		g.sd(hx-1, ly, 7)
		g.sd(lx, hy-1, 9)
		g.sd(hx-1, hy-1, 6)
		if g.flip(2) {
			g.sd(hx-1, (ly+hy)/2, 4)
		} else {
			g.sd((lx+hx)/2, hy-1, 5)
		}
	case DTypeCaves:
		g.sd(lx, ly, 150)
		g.sd(hx-1, ly, 151)
		g.sd(lx, hy-1, 152)
		g.sd(hx-1, hy-1, 138)
		if g.flip(2) {
			g.sd(hx-1, (ly+hy)/2, 147)
		} else {
			g.sd((lx+hx)/2, hy-1, 146)
		}
	case DTypeHell:
		g.sd(lx, ly, 9)
		g.sd(hx-1, ly, 16)
		g.sd(lx, hy-1, 15)
		g.sd(hx-1, hy-1, 12)
		if g.flip(2) {
			yy := (ly + hy) / 2
			g.sd(hx-1, yy-1, 53)
			g.sd(hx-1, yy, 6)
			g.sd(hx-1, yy+1, 52)
			g.sd(hx-2, yy-1, 54)
		} else {
			xx := (lx + hx) / 2
			g.sd(xx-1, hy-1, 57)
			g.sd(xx, hy-1, 6)
			g.sd(xx+1, hy-1, 56)
			g.sd(xx, hy-2, 59)
			g.sd(xx-1, hy-2, 58)
		}
	}
}

// placeThemeRooms ports DRLG_PlaceThemeRooms.
func (g *gen) placeThemeRooms(minSize, maxSize int, floor byte, freq int, rndSize bool) {
	g.themes = nil
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) != floor || !g.flip(freq) {
				continue
			}
			w, h, ok := g.sizeForThemeRoom(floor, i, j, minSize, maxSize)
			if !ok {
				continue
			}
			if rndSize {
				lo, hi := minSize-2, maxSize-2
				if w = lo + g.rnd(g.rnd(w-lo+1)); w < lo || w > hi {
					w = lo
				}
				if h = lo + g.rnd(g.rnd(h-lo+1)); h < lo || h > hi {
					h = lo
				}
			}
			t := themeLoc{room: rect{i + 1, j + 1, w, h}}
			if g.ltype == DTypeCaves {
				g.rectTrans(2*i+20, 2*j+20, 2*(i+w)+15, 2*(j+h)+15)
			} else {
				g.mRectTrans(i+1, j+1, i+w, j+h)
			}
			t.ttval = g.transVal - 1
			g.createThemeRoom(t)
			g.themes = append(g.themes, t)
		}
	}
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

// lPass3 ports DRLG_LPass3: megatiles to pieces through the level type's .TIL (lv = the fill megatile).
func (g *gen) lPass3(lv int) {
	m := g.mega(lv)
	for j := 0; j < 112; j += 2 {
		for i := 0; i < 112; i += 2 {
			g.piece[i][j], g.piece[i+1][j], g.piece[i][j+1], g.piece[i+1][j+1] = m[0], m[1], m[2], m[3]
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) == 0 {
				continue // D1's assembly skips tile 0 and keeps the fill megatile
			}
			m := g.mega(int(g.d(i, j)) - 1)
			xx, yy := 16+2*i, 16+2*j
			g.piece[xx][yy], g.piece[xx+1][yy], g.piece[xx][yy+1], g.piece[xx+1][yy+1] = m[0], m[1], m[2], m[3]
		}
	}
}

// mega is megatile t's four pieces from the level type's .TIL.
func (g *gen) mega(t int) [4]int { return megaPieces(g.til, t) }

func megaPieces(til []byte, t int) [4]int {
	o := 8 * t
	if t < 0 || o+8 > len(til) {
		return [4]int{}
	}
	return [4]int{int(binary.LittleEndian.Uint16(til[o:])), int(binary.LittleEndian.Uint16(til[o+2:])),
		int(binary.LittleEndian.Uint16(til[o+4:])), int(binary.LittleEndian.Uint16(til[o+6:]))}
}

func (g *gen) isFloor(x, y int, floor byte) bool {
	i, j := (x-16)/2, (y-16)/2
	if x < 16 || y < 16 || i >= 40 || j >= 40 {
		return false
	}
	return g.d(i, j) == floor
}

// floodTrans ports FloodTransparencyValues (DRLG_L*FloodTVal): one room region per connected floor area.
func (g *gen) floodTrans(floor byte) {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if xx, yy := 16+2*i, 16+2*j; g.d(i, j) == floor && g.trans[xx][yy] == 0 {
				g.findTrans(xx, yy, floor)
				g.transVal++
			}
		}
	}
}

// findTrans ports FindTransparencyValues' span fill.
func (g *gen) findTrans(fx, fy int, floor byte) {
	type seed struct{ scanStart, scanEnd, y, dy int }
	stack := []seed{{fx, fx + 1, fy, 1}}
	inside := func(x, y int) bool { return g.getTrans(x, y) == 0 && g.isFloor(x, y, floor) }
	set := func(x, y int) {
		for _, d := range [8]point{{0, 1}, {0, -1}, {1, 0}, {-1, 0}, {1, -1}, {-1, -1}, {1, 1}, {-1, 1}} {
			if !g.isFloor(x+d.x, y+d.y, floor) {
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
