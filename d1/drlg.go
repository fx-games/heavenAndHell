package d1

import (
	"math/rand"
	"strings"
)

// drlg holds the generator state gendung.cpp shares between the level types: the 40×40 megatile map, its flags,
// dTransVal and the theme rooms placed during generation.
type drlg struct {
	rng      *rand.Rand
	dungeon  [40][40]byte
	flags    [40][40]byte
	trans    [112][112]int // dTransVal in dPiece coordinates (megatile i covers 16+2i..17+2i)
	transVal int
	themes   []themeLoc
}

type themeLoc struct{ x, y, w, h, ttval int }

func newDrlg(rng *rand.Rand) drlg { return drlg{rng: rng, transVal: 1} }

func (g *drlg) random(v int) int {
	if v <= 0 {
		return 0
	}
	return g.rng.Intn(v)
}

// at reads dungeon with D1's out-of-bounds reads turned into 0.
func (g *drlg) at(i, j int) byte {
	if i < 0 || j < 0 || i >= 40 || j >= 40 {
		return 0
	}
	return g.dungeon[i][j]
}

func (g *drlg) set(i, j int, v byte) {
	if i >= 0 && j >= 0 && i < 40 && j < 40 {
		g.dungeon[i][j] = v
	}
}

func (g *drlg) flag(i, j int) byte {
	if i < 0 || j < 0 || i >= 40 || j >= 40 {
		return 0
	}
	return g.flags[i][j]
}

func (g *drlg) setTrans(x, y int) {
	if x >= 0 && y >= 0 && x < 112 && y < 112 {
		g.trans[x][y] = g.transVal
	}
}

// ftvr ports DRLG_L*FTVR: flood a room region over floor megatiles, marking the wall edges it touches.
func (g *drlg) ftvr(floor byte, i, j, x, y, d int) {
	if x < 0 || y < 0 || x > 110 || y > 110 {
		return
	}
	if g.trans[x][y] != 0 || g.at(i, j) != floor {
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
	g.ftvr(floor, i+1, j, x+2, y, 1)
	g.ftvr(floor, i-1, j, x-2, y, 2)
	g.ftvr(floor, i, j+1, x, y+2, 3)
	g.ftvr(floor, i, j-1, x, y-2, 4)
	g.ftvr(floor, i-1, j-1, x-2, y-2, 5)
	g.ftvr(floor, i+1, j-1, x+2, y-2, 6)
	g.ftvr(floor, i-1, j+1, x-2, y+2, 7)
	g.ftvr(floor, i+1, j+1, x+2, y+2, 8)
}

// floodTVal ports DRLG_L*FloodTVal.
func (g *drlg) floodTVal(floor byte) {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.dungeon[i][j] == floor && g.trans[16+2*i][16+2*j] == 0 {
				g.ftvr(floor, i, j, 16+2*i, 16+2*j, 0)
				g.transVal++
			}
		}
	}
}

// placeMiniSet ports DRLG_L*PlaceMiniSet for one placement anywhere (tmin = tmax = 1, cx = cy = -1).
// It returns the set's top-left megatile, or -1 after bailing out.
func (g *drlg) placeMiniSet(m []byte, tries int, flag byte) (int, int) {
	sw, sh := int(m[0]), int(m[1])
	sx, sy := g.random(40-sw), g.random(40-sh)
	for n := 0; ; n++ {
		if n >= tries {
			return -1, -1
		}
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
	}
	ii := sw*sh + 2
	for yy := 0; yy < sh; yy++ {
		for xx := 0; xx < sw; xx++ {
			if m[ii] != 0 {
				g.dungeon[xx+sx][sy+yy] = m[ii]
				g.flags[xx+sx][sy+yy] |= flag
			}
			ii++
		}
	}
	return sx, sy
}

func (g *drlg) skipThemeRoom(x, y int) bool {
	for _, t := range g.themes {
		if x >= t.x-2 && x <= t.x+t.w+2 && y >= t.y-2 && y <= t.y+t.h+2 {
			return false
		}
	}
	return true
}

// willThemeRoomFit ports DRLG_WillThemeRoomFit, including its absolute-coordinate size check.
func (g *drlg) willThemeRoomFit(floor byte, x, y, minSize, maxSize int) (int, int, bool) {
	if x > 40-maxSize && y > 40-maxSize || !g.skipThemeRoom(x, y) {
		return 0, 0, false
	}
	var xArray, yArray [20]int
	xFlag, yFlag := true, true
	xCount, yCount := 0, 0
	for ii := 0; ii < maxSize; ii++ {
		if xFlag {
			for xx := x; xx < x+maxSize; xx++ {
				if g.at(xx, y+ii) != floor {
					if xx >= minSize {
						break
					}
					xFlag = false
				} else {
					xCount++
				}
			}
			if xFlag {
				xArray[ii], xCount = xCount, 0
			}
		}
		if yFlag {
			for yy := y; yy < y+maxSize; yy++ {
				if g.at(x+ii, yy) != floor {
					if yy >= minSize {
						break
					}
					yFlag = false
				} else {
					yCount++
				}
			}
			if yFlag {
				yArray[ii], yCount = yCount, 0
			}
		}
	}
	for ii := 0; ii < minSize; ii++ {
		if xArray[ii] < minSize || yArray[ii] < minSize {
			return 0, 0, false
		}
	}
	xs, ys := xArray[0], yArray[0]
	for ii := 0; ii < maxSize; ii++ {
		if xArray[ii] < minSize || yArray[ii] < minSize {
			break
		}
		xs, ys = min(xs, xArray[ii]), min(ys, yArray[ii])
	}
	return xs - 2, ys - 2, true
}

// placeThemeRooms ports DRLG_PlaceThemeRooms; draw writes one theme room's megatiles (DRLG_CreateThemeRoom).
func (g *drlg) placeThemeRooms(caves bool, minSize, maxSize int, floor byte, freq int, rndSize bool, draw func(t themeLoc)) {
	g.themes = nil
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if g.dungeon[x][y] != floor || g.random(freq) != 0 {
				continue
			}
			w, h, ok := g.willThemeRoomFit(floor, x, y, minSize, maxSize)
			if !ok {
				continue
			}
			if rndSize {
				lo, hi := minSize-2, maxSize-2
				if w = lo + g.random(g.random(w-lo+1)); w < lo || w > hi {
					w = lo
				}
				if h = lo + g.random(g.random(h-lo+1)); h < lo || h > hi {
					h = lo
				}
			}
			t := themeLoc{x: x + 1, y: y + 1, w: w, h: h}
			if caves {
				g.rectTrans(2*x+20, 2*y+20, 2*(x+w)+15, 2*(y+h)+15)
			} else {
				g.rectTrans(2*(x+1)+17, 2*(y+1)+17, 2*(x+w)+16, 2*(y+h)+16) // DRLG_MRectTrans
			}
			t.ttval = g.transVal - 1
			draw(t)
			g.themes = append(g.themes, t)
		}
	}
}

// rectTrans ports DRLG_RectTrans (dPiece coordinates).
func (g *drlg) rectTrans(x1, y1, x2, y2 int) {
	for j := y1; j <= y2; j++ {
		for i := x1; i <= x2; i++ {
			g.setTrans(i, j)
		}
	}
	g.transVal++
}

// drawThemeRoom writes a theme room's border (walls hw/vw, corners c[4]: top-left, top-right, bottom-left,
// bottom-right) and floor, like DRLG_CreateThemeRoom.
func (g *drlg) drawThemeRoom(t themeLoc, hw, vw, floor byte, c [4]byte) {
	for yy := t.y; yy < t.y+t.h; yy++ {
		for xx := t.x; xx < t.x+t.w; xx++ {
			switch {
			case yy == t.y || yy == t.y+t.h-1:
				g.set(xx, yy, hw)
			case xx == t.x || xx == t.x+t.w-1:
				g.set(xx, yy, vw)
			default:
				g.set(xx, yy, floor)
			}
		}
	}
	g.set(t.x, t.y, c[0])
	g.set(t.x+t.w-1, t.y, c[1])
	g.set(t.x, t.y+t.h-1, c[2])
	g.set(t.x+t.w-1, t.y+t.h-1, c[3])
}

// level copies dTransVal and the theme rooms into a Level.
func (g *drlg) level() *Level {
	lvl := &Level{}
	for x := 0; x < LevelSize; x++ {
		for y := 0; y < LevelSize; y++ {
			lvl.Trans[x][y] = g.trans[x+16][y+16]
		}
	}
	for _, t := range g.themes {
		lvl.Themes = append(lvl.Themes, t.ttval)
	}
	return lvl
}

// megaCells sets the 2×2 cells of megatile (i, j).
func (l *Level) megaCells(i, j int, c Cell) {
	for n := 0; n < 4; n++ {
		l.Cells[2*i+n%2][2*j+n/2] = c
	}
}

// nearestFloor returns the walkable cell closest to (x, y), for putting stairs where D1 puts the player.
func (l *Level) nearestFloor(x, y int) [2]int {
	for r := 0; r < LevelSize; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(iabs(dx), iabs(dy)) == r && l.Cells[clamp(x+dx, 0, LevelSize-1)][clamp(y+dy, 0, LevelSize-1)] == CellFloor {
					return [2]int{clamp(x+dx, 0, LevelSize-1), clamp(y+dy, 0, LevelSize-1)}
				}
			}
		}
	}
	return [2]int{x, y}
}

func iabs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// Generate builds dungeon level dlvl (1-16) with the generator of its level type.
func Generate(rng *rand.Rand, dlvl int) *Level {
	switch (dlvl-1)/4 + 1 {
	case 1:
		return GenerateL1(rng, dlvl)
	case 2:
		return GenerateL2(rng, dlvl)
	case 3:
		return GenerateL3(rng, dlvl)
	}
	return GenerateL4(rng, dlvl)
}

// String draws the level: # wall, . floor, + door, < > stairs, W town warp.
func (l *Level) String() string {
	var b strings.Builder
	for y := 0; y < LevelSize; y++ {
		for x := 0; x < LevelSize; x++ {
			switch p := [2]int{x, y}; {
			case p == l.StairsUp:
				b.WriteByte('<')
			case p == l.StairsDn:
				b.WriteByte('>')
			case p == l.Warp && p != [2]int{}:
				b.WriteByte('W')
			default:
				b.WriteByte("#.+"[l.Cells[x][y]])
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}
