package d1

import "math/rand"

// l3 holds drlg_l3.cpp's globals.
type l3 struct {
	drlg
}

var (
	l3ConvTbl  = [16]byte{8, 11, 3, 10, 1, 9, 12, 12, 6, 13, 4, 13, 2, 14, 5, 7}
	l3UP       = []byte{3, 3, 8, 8, 0, 10, 10, 0, 7, 7, 0, 51, 50, 0, 48, 49, 0, 0, 0, 0}
	l3DOWN     = []byte{3, 3, 8, 9, 7, 8, 9, 7, 0, 0, 0, 0, 47, 0, 0, 46, 0, 0, 0, 0}
	l3HOLDWARP = []byte{3, 3, 8, 8, 0, 10, 10, 0, 7, 7, 0, 125, 125, 0, 125, 125, 0, 0, 0, 0}
)

func (g *l3) fillRoom(x1, y1, x2, y2 int) bool {
	if x1 <= 1 || x2 >= 34 || y1 <= 1 || y2 >= 38 {
		return false
	}
	for j := y1; j <= y2; j++ {
		for i := x1; i <= x2; i++ {
			if g.dungeon[i][j] != 0 {
				return false
			}
		}
	}
	for j := y1 + 1; j < y2; j++ {
		for i := x1 + 1; i < x2; i++ {
			g.dungeon[i][j] = 1
		}
	}
	for j := y1; j <= y2; j++ {
		if g.random(2) != 0 {
			g.dungeon[x1][j] = 1
		}
		if g.random(2) != 0 {
			g.dungeon[x2][j] = 1
		}
	}
	for i := x1; i <= x2; i++ {
		if g.random(2) != 0 {
			g.dungeon[i][y1] = 1
		}
		if g.random(2) != 0 {
			g.dungeon[i][y2] = 1
		}
	}
	return true
}

func (g *l3) createBlock(x, y, obs, dir int) {
	bx, by := g.random(2)+3, g.random(2)+3
	var x1, y1, x2, y2 int
	along := func(size, v int) int {
		switch {
		case size < obs:
			return g.random(size) + v
		case size == obs:
			return v
		}
		return v - g.random(size)
	}
	switch dir {
	case 0:
		y2 = y - 1
		y1 = y2 - by
		x1 = along(bx, x)
		x2 = bx + x1
	case 3:
		x2 = x - 1
		x1 = x2 - bx
		y1 = along(by, y)
		y2 = y1 + by
	case 2:
		y1 = y + 1
		y2 = y1 + by
		x1 = along(bx, x)
		x2 = bx + x1
	case 1:
		x1 = x + 1
		x2 = x1 + bx
		y1 = along(by, y)
		y2 = y1 + by
	}
	if g.fillRoom(x1, y1, x2, y2) {
		cont := g.random(4) != 0
		if cont && dir != 2 {
			g.createBlock(x1, y1, by, 0)
		}
		if cont && dir != 3 {
			g.createBlock(x2, y1, bx, 1)
		}
		if cont && dir != 0 {
			g.createBlock(x1, y2, by, 2)
		}
		if cont && dir != 1 {
			g.createBlock(x1, y1, bx, 3)
		}
	}
}

func (g *l3) corners(i, j int) int {
	return int(g.at(i+1, j+1)) + 2*int(g.at(i, j+1)) + 4*int(g.at(i+1, j)) + 8*int(g.at(i, j))
}

func (g *l3) fillDiags() {
	for j := 0; j < 39; j++ {
		for i := 0; i < 39; i++ {
			switch g.corners(i, j) {
			case 6:
				if g.random(2) == 0 {
					g.dungeon[i][j] = 1
				} else {
					g.dungeon[i+1][j+1] = 1
				}
			case 9:
				if g.random(2) == 0 {
					g.dungeon[i+1][j] = 1
				} else {
					g.dungeon[i][j+1] = 1
				}
			}
		}
	}
}

func (g *l3) fillSingles() {
	d := &g.dungeon
	for j := 1; j < 39; j++ {
		for i := 1; i < 39; i++ {
			if d[i][j] == 0 && d[i][j-1]+d[i-1][j-1]+d[i+1][j-1] == 3 && d[i+1][j]+d[i-1][j] == 2 &&
				d[i][j+1]+d[i-1][j+1]+d[i+1][j+1] == 3 {
				d[i][j] = 1
			}
		}
	}
}

// fillStraights ports DRLG_L3FillStraights: long straight cave edges get a ragged row of random floor.
func (g *l3) fillStraights() {
	d := &g.dungeon
	for _, pass := range []struct {
		vert       bool
		edge, fill byte
		off        int
	}{{false, 0, 1, 0}, {false, 1, 0, 1}, {true, 0, 1, 0}, {true, 1, 0, 1}} {
		for a := 0; a < 39; a++ {
			s, c := 0, 0
			for b := 0; b < 37; b++ {
				i, j, ni, nj := b, a, b, a+1
				if pass.vert {
					i, j, ni, nj = a, b, a+1, b
				}
				if d[i][j] == pass.edge && d[ni][nj] == pass.fill {
					if s == 0 {
						c = b
					}
					s++
					continue
				}
				if s > 3 && g.random(2) != 0 {
					for k := c; k < b; k++ {
						if pass.vert {
							d[a+pass.off][k] = byte(g.random(2))
						} else {
							d[k][a+pass.off] = byte(g.random(2))
						}
					}
				}
				s = 0
			}
		}
	}
}

func (g *l3) floorArea() (n int) {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			n += int(g.dungeon[i][j])
		}
	}
	return
}

// lockout ports DRLG_L3Lockout: every floor megatile is connected to the others.
func (g *l3) lockout() bool {
	var seen [40][40]bool
	total, fx, fy := 0, 0, 0
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.dungeon[i][j] != 0 {
				total, fx, fy = total+1, i, j
			}
		}
	}
	count := 0
	stack := [][2]int{{fx, fy}}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if p[0] < 0 || p[1] < 0 || p[0] >= 40 || p[1] >= 40 || seen[p[0]][p[1]] || g.dungeon[p[0]][p[1]] == 0 {
			continue
		}
		seen[p[0]][p[1]] = true
		count++
		stack = append(stack, [2]int{p[0], p[1] - 1}, [2]int{p[0], p[1] + 1}, [2]int{p[0] - 1, p[1]}, [2]int{p[0] + 1, p[1]})
	}
	return total == count
}

func (g *l3) makeMegas() {
	for j := 0; j < 39; j++ {
		for i := 0; i < 39; i++ {
			v := g.corners(i, j)
			if v == 6 {
				if g.random(2) == 0 {
					v = 12
				} else {
					v = 5
				}
			}
			if v == 9 {
				if g.random(2) == 0 {
					v = 13
				} else {
					v = 14
				}
			}
			g.dungeon[i][j] = l3ConvTbl[v]
		}
		g.dungeon[39][j] = 8
	}
	for i := 0; i < 40; i++ {
		g.dungeon[i][39] = 8
	}
}

// GenerateL3 ports DRLG_L3 (Caves) down to the grid. Megatile (i, j) shows the corners of the floor map, so cell
// (2i+a, 2j+b) is floor map [i+a][j+b]. Lava lakes, rivers and wooden fences become lava and fence cells (they block
// movement, not sight). ponytail: islands, cracks and the Hall of Heroes fix are skipped (they need the grid built
// from megatiles instead of the floor map); the Anvil set piece is a deferred quest.
func GenerateL3(rng *rand.Rand, dlvl int) *Level {
	for {
		g := &l3{drlg: newDrlg(rng)}
		for {
			g.dungeon = [40][40]byte{}
			x1, y1 := g.random(20)+10, g.random(20)+10
			x2, y2 := x1+2, y1+2
			g.fillRoom(x1, y1, x2, y2)
			g.createBlock(x1, y1, 2, 0)
			g.createBlock(x2, y1, 2, 1)
			g.createBlock(x1, y2, 2, 2)
			g.createBlock(x1, y1, 2, 3)
			g.fillDiags()
			g.fillSingles()
			g.fillStraights()
			g.fillDiags()
			for n := 0; n < 40; n++ {
				g.dungeon[39][n], g.dungeon[n][39] = 0, 0
			}
			if g.floorArea() >= 600 && g.lockout() {
				break
			}
		}
		mask := g.dungeon
		g.makeMegas()
		ux, uy := g.placeMiniSet(l3UP, 200, 0)
		if ux < 0 {
			continue
		}
		dx, dy := g.placeMiniSet(l3DOWN, 200, 0)
		if dx < 0 {
			continue
		}
		wx, wy := -1, -1
		if dlvl == 9 {
			if wx, wy = g.placeMiniSet(l3HOLDWARP, 200, 0); wx < 0 {
				continue
			}
		}
		if !g.pool() { // D1 regenerates until the level has a lava lake
			continue
		}
		g.poolFix()
		g.river()
		type door struct{ i, j int }
		var fences []themeLoc
		var doors []door
		g.placeThemeRooms(true, 5, 10, 7, 0, false, func(t themeLoc) {
			g.drawThemeRoom(t, 134, 137, 7, [4]byte{150, 151, 152, 138})
			if g.random(2) == 0 {
				g.set(t.x+t.w-1, t.y+t.h/2, 147)
				doors = append(doors, door{t.x + t.w - 1, t.y + t.h/2})
			} else {
				g.set(t.x+t.w/2, t.y+t.h-1, 146)
				doors = append(doors, door{t.x + t.w/2, t.y + t.h - 1})
			}
			fences = append(fences, t)
		})
		g.wood()
		lvl := g.level()
		for x := 0; x < LevelSize; x++ {
			for y := 0; y < LevelSize; y++ {
				floor := mask[min((x+1)/2, 39)][min((y+1)/2, 39)] != 0
				v := g.dungeon[x/2][y/2]
				switch {
				case floor && (between(v, 15, 24) || between(v, 38, 43)): // rivers flow over floor
					lvl.Cells[x][y] = CellLava
				case !floor && between(v, 25, 41): // lakes fill the rock
					lvl.Cells[x][y] = CellLava
				case floor && x%2 == 0 && (v == 131 || v == 133 || v == 135 || v == 137 || v == 139 || v == 138): // vertical fence
					lvl.Cells[x][y] = CellFence
				case floor && y%2 == 0 && (v == 130 || v == 132 || v == 134 || v == 136 || v == 140 || v == 138): // horizontal fence
					lvl.Cells[x][y] = CellFence
				case floor:
					lvl.Cells[x][y] = CellFloor
				}
			}
		}
		for n, t := range fences { // the theme rooms' fences are whole megatiles, their gate is open
			for j := t.y; j < t.y+t.h; j++ {
				for i := t.x; i < t.x+t.w; i++ {
					if i == t.x || j == t.y || i == t.x+t.w-1 || j == t.y+t.h-1 {
						lvl.megaCells(i, j, CellFence)
					}
				}
			}
			lvl.megaCells(doors[n].i, doors[n].j, CellFloor)
		}
		lvl.StairsUp = lvl.nearestFloor(2*ux+1, 2*uy+3)
		lvl.StairsDn = lvl.nearestFloor(2*dx+3, 2*dy+1)
		if wx >= 0 {
			lvl.Warp = lvl.nearestFloor(2*wx+1, 2*wy+3)
		}
		// ponytail: crossing rivers and fences can wall off parts D1 never checks; such levels are regenerated.
		if seen := lvl.reachable(lvl.StairsUp); !seen[lvl.StairsDn] || wx >= 0 && !seen[lvl.Warp] || len(seen) < lvl.walkableCount()*9/10 {
			continue
		}
		return lvl
	}
}

var (
	l3SpawnTbl     = [15]byte{0, 10, 3, 5, 12, 6, 9, 0, 0, 12, 3, 6, 9, 10, 5}
	l3SpawnEdgeTbl = [15]byte{0, 10, 67, 5, 44, 6, 9, 0, 0, 28, 131, 6, 9, 10, 5}
	l3PoolSub      = [15]byte{0, 35, 26, 36, 25, 29, 34, 7, 33, 28, 27, 37, 32, 31, 30}
	l3Dirs         = [4][2]int{{0, -1}, {0, 1}, {1, 0}, {-1, 0}} // the order of spawntable's bits 8, 4, 2, 1
)

// spawn ports DRLG_L3Spawn (edge = DRLG_L3SpawnEdge): flood a rock region, true if it isn't enclosed or too big.
func (g *l3) spawn(x, y int, tot *int, edge bool) bool {
	if *tot > 40 || x < 0 || y < 0 || x >= 40 || y >= 40 {
		return true
	}
	v := g.dungeon[x][y]
	if v&0x80 != 0 {
		return false
	}
	if v > 15 {
		return true
	}
	g.dungeon[x][y] |= 0x80
	*tot++
	if !edge && v == 8 {
		return g.spawn(x+1, y, tot, false) || g.spawn(x-1, y, tot, false) || g.spawn(x, y+1, tot, false) || g.spawn(x, y-1, tot, false)
	}
	t := l3SpawnTbl[v]
	if edge {
		t = l3SpawnEdgeTbl[v]
	}
	for k, d := range l3Dirs {
		if t&(8>>k) != 0 && g.spawn(x+d[0], y+d[1], tot, true) {
			return true
		}
	}
	for k, d := range l3Dirs {
		if edge && t&(0x80>>k) != 0 && g.spawn(x+d[0], y+d[1], tot, false) {
			return true
		}
	}
	return false
}

// pool ports DRLG_L3Pool: small enclosed rock regions become lava lakes. It reports whether any did.
func (g *l3) pool() (lava bool) {
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if g.dungeon[x][y] != 8 {
				continue
			}
			g.dungeon[x][y] |= 0x80
			tot, found := 1, true
			if x+1 < 40 {
				found = g.spawn(x+1, y, &tot, false)
			}
			// D1's else branches: found stays false only if every spawn ran and returned false.
			if x-1 > 0 && !found {
				found = g.spawn(x-1, y, &tot, false)
			} else {
				found = true
			}
			if y+1 < 40 && !found {
				found = g.spawn(x, y+1, &tot, false)
			} else {
				found = true
			}
			if y-1 > 0 && !found {
				found = g.spawn(x, y-1, &tot, false)
			} else {
				found = true
			}
			chance := g.random(100)
			for j := max(y-tot, 0); j < min(y+tot, 40); j++ {
				for i := max(x-tot, 0); i < min(x+tot, 40); i++ {
					if g.dungeon[i][j]&0x80 == 0 {
						continue
					}
					g.dungeon[i][j] &^= 0x80
					if tot > 4 && chance < 25 && !found {
						if k := l3PoolSub[g.dungeon[i][j]]; k != 0 && k <= 37 {
							g.dungeon[i][j] = k
						}
						lava = true
					}
				}
			}
		}
	}
	return
}

// poolFix ports DRLG_L3PoolFix: rock surrounded by lava becomes lava.
func (g *l3) poolFix() {
	isLava := func(i, j int) bool { v := g.at(i, j); return v >= 25 && v <= 41 }
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if g.dungeon[x][y] == 8 && isLava(x-1, y-1) && isLava(x-1, y) && isLava(x-1, y+1) && isLava(x, y-1) &&
				isLava(x, y+1) && isLava(x+1, y-1) && isLava(x+1, y) && isLava(x+1, y+1) {
				g.dungeon[x][y] = 33
			}
		}
	}
}

// river ports DRLG_L3River: up to 4 lava rivers flow from a lake's shore into a wall, each with one bridge.
func (g *l3) river() {
	var river [3][101]int
	rivercnt, trys, dir, pdir, nodir, amt := 0, 0, 0, 0, 0, 0
	for trys < 200 && rivercnt < 4 {
		bail := false
		for !bail && trys < 200 {
			trys++
			rx, ry, i := 0, 0, 0
			for (g.at(rx, ry) < 25 || g.at(rx, ry) > 28) && i < 100 {
				rx, ry = g.random(40), g.random(40)
				i++
				for (g.at(rx, ry) < 25 || g.at(rx, ry) > 28) && ry < 40 {
					if rx++; rx >= 40 {
						rx = 0
						ry++
					}
				}
			}
			if i >= 100 {
				return
			}
			switch g.at(rx, ry) {
			case 25:
				dir, nodir, river[2][0] = 3, 2, 40
			case 26:
				dir, nodir, river[2][0] = 0, 1, 38
			case 27:
				dir, nodir, river[2][0] = 1, 0, 41
			case 28:
				dir, nodir, river[2][0] = 2, 3, 39
			}
			river[0][0], river[1][0] = rx, ry
			amt = 1
			nodir2, dircheck := 4, 0
			for dircheck < 4 && amt < 100 {
				px, py := rx, ry
				if dircheck == 0 {
					dir = g.random(4)
				} else {
					dir = (dir + 1) & 3
				}
				dircheck++
				for dir == nodir || dir == nodir2 {
					dir = (dir + 1) & 3
					dircheck++
				}
				switch {
				case dir == 0 && ry > 0:
					ry--
				case dir == 1 && ry < 40:
					ry++
				case dir == 2 && rx < 40:
					rx++
				case dir == 3 && rx > 0:
					rx--
				}
				if g.at(rx, ry) != 7 {
					rx, ry = px, py
					continue
				}
				dircheck = 0
				if dir < 2 {
					river[2][amt] = g.random(2) + 17
				} else {
					river[2][amt] = g.random(2) + 15
				}
				river[0][amt], river[1][amt] = rx, ry
				amt++
				corner := func(tile, n2a, n2b int) {
					if amt > 2 {
						river[2][amt-2] = tile
					}
					if dir < 2 {
						nodir2 = n2a
					} else {
						nodir2 = n2b
					}
				}
				switch {
				case dir == 0 && pdir == 2 || dir == 3 && pdir == 1:
					corner(22, 1, 2)
				case dir == 0 && pdir == 3 || dir == 2 && pdir == 1:
					corner(21, 1, 3)
				case dir == 1 && pdir == 2 || dir == 3 && pdir == 0:
					corner(20, 0, 2)
				case dir == 1 && pdir == 3 || dir == 2 && pdir == 0:
					corner(19, 0, 3)
				}
				pdir = dir
			}
			end := func(x, y, tile int, p1, t1, p2, t2 int) {
				river[0][amt], river[1][amt], river[2][amt] = x, y, tile
				if pdir == p1 {
					river[2][amt-1] = t1
				}
				if pdir == p2 {
					river[2][amt-1] = t2
				}
				bail = true
			}
			switch {
			case dir == 0 && g.at(rx, ry-1) == 10 && g.at(rx, ry-2) == 8:
				end(rx, ry-1, 24, 2, 22, 3, 21)
			case dir == 1 && g.at(rx, ry+1) == 2 && g.at(rx, ry+2) == 8:
				end(rx, ry+1, 42, 2, 20, 3, 19)
			case dir == 2 && g.at(rx+1, ry) == 4 && g.at(rx+2, ry) == 8:
				end(rx+1, ry, 43, 0, 19, 1, 21)
			case dir == 3 && g.at(rx-1, ry) == 9 && g.at(rx-2, ry) == 8:
				end(rx-1, ry, 23, 0, 20, 1, 22)
			}
		}
		if !bail || amt < 7 {
			continue
		}
		found, bridge := 0, 0
		for lp := 0; found == 0 && lp < 30; lp++ {
			bridge = g.random(amt)
			bx, by, bt := river[0][bridge], river[1][bridge], river[2][bridge]
			if (bt == 15 || bt == 16) && g.at(bx, by-1) == 7 && g.at(bx, by+1) == 7 {
				found = 1
			}
			if (bt == 17 || bt == 18) && g.at(bx-1, by) == 7 && g.at(bx+1, by) == 7 {
				found = 2
			}
			for i := 0; i < amt && found != 0; i++ {
				if found == 1 && (by-1 == river[1][i] || by+1 == river[1][i]) && bx == river[0][i] ||
					found == 2 && (bx-1 == river[0][i] || bx+1 == river[0][i]) && by == river[1][i] {
					found = 0
				}
			}
		}
		if found == 0 {
			continue
		}
		river[2][bridge] = 43 + found // 44 / 45
		rivercnt++
		for b := 0; b <= amt; b++ {
			g.set(river[0][b], river[1][b], byte(river[2][b]))
		}
	}
}

func between(v byte, lo, hi byte) bool { return v >= lo && v <= hi }

// wood ports DRLG_L3Wood: wooden beams along cave walls, then fences across open floor with a gap in each.
func (g *l3) wood() {
	d := &g.dungeon
	run := func(i, j, di, dj int, v byte) (int, int) { // last megatile of the run of v from (i, j)
		for g.at(i, j) == v {
			i, j = i+di, j+dj
		}
		return i - di, j - dj
	}
	beams := func(i, j, di, dj, n int, a, b byte) {
		for k := 1; k < n; k++ {
			if g.random(2) != 0 {
				d[i+k*di][j+k*dj] = a
			} else {
				d[i+k*di][j+k*dj] = b
			}
		}
	}
	for j := 0; j < 39; j++ {
		for i := 0; i < 39; i++ {
			if d[i][j] == 10 && g.random(2) != 0 {
				if x, _ := run(i, j, 1, 0, 10); x-i > 0 {
					d[i][j] = 127
					beams(i, j, 1, 0, x-i, 126, 129)
					d[x][j] = 128
				}
			}
			if d[i][j] == 9 && g.random(2) != 0 {
				if _, y := run(i, j, 0, 1, 9); y-j > 0 {
					d[i][j] = 123
					beams(i, j, 0, 1, y-j, 121, 124)
					d[i][y] = 122
				}
			}
			if d[i][j] == 11 && d[i+1][j] == 10 && d[i][j+1] == 9 && g.random(2) != 0 {
				d[i][j] = 125
				x, _ := run(i+1, j, 1, 0, 10)
				beams(i, j, 1, 0, x-i, 126, 129)
				d[x][j] = 128
				_, y := run(i, j+1, 0, 1, 9)
				beams(i, j, 0, 1, y-j, 121, 124)
				d[i][y] = 122
			}
		}
	}

	isFence := func(i, j int) bool { return between(g.at(i, j), 130, 152) }
	extends := func(v byte, ok ...byte) bool {
		for _, o := range ok {
			if v == o {
				return true
			}
		}
		return false
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if d[i][j] != 7 || !g.skipThemeRoom(i, j) {
				continue
			}
			vert := g.random(2) == 0
			// a fence runs across floor from wall to wall, converting what it crosses (D1's conv tables)
			var lo, hi int
			if vert {
				for lo = j; !isFence(i+1, lo) && !isFence(i-1, lo) && extends(g.at(i, lo), 7, 10, 126, 129, 134, 136); lo-- {
				}
				for hi = j; !isFence(i+1, hi) && !isFence(i-1, hi) && extends(g.at(i, hi), 7, 2, 134, 136); hi++ {
				}
			} else {
				for lo = i; !isFence(lo, j+1) && !isFence(lo, j-1) && extends(g.at(lo, j), 7, 9, 121, 124, 135, 137); lo-- {
				}
				for hi = i; !isFence(hi, j+1) && !isFence(hi, j-1) && extends(g.at(hi, j), 7, 4, 135, 137); hi++ {
				}
			}
			lo, hi = lo+1, hi-1
			at := func(k int) *byte {
				if vert {
					return &d[i][k]
				}
				return &d[k][j]
			}
			if hi-lo <= 1 || *at(lo) == 7 || *at(hi) == 7 {
				continue
			}
			conv := map[byte]byte{10: 131, 126: 133, 129: 133, 2: 139, 134: 138, 136: 138}
			floor := [2]byte{135, 137}
			if !vert {
				conv = map[byte]byte{9: 130, 121: 132, 124: 132, 4: 140, 135: 138, 137: 138}
				floor = [2]byte{134, 136}
			}
			rp := g.random(hi-lo-1) + lo + 1
			for k := lo; k <= hi; k++ {
				if k == rp {
					continue
				}
				if p := at(k); *p == 7 {
					*p = floor[g.random(2)]
				} else if c, ok := conv[*p]; ok {
					*p = c
				}
			}
		}
	}

	// AddFenceDoors + FenceDoorFix: a floor megatile between two fences becomes a gate, kept only between fence pieces.
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if d[i][j] == 7 && isFence(i-1, j) && isFence(i+1, j) {
				d[i][j] = 146
			} else if d[i][j] == 7 && isFence(i, j-1) && isFence(i, j+1) {
				d[i][j] = 147
			}
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			switch v := d[i][j]; {
			case v == 146 && (!isFence(i-1, j) || !isFence(i+1, j) || !extends(g.at(i-1, j), 130, 132, 133, 134, 136, 138, 140) && !extends(g.at(i+1, j), 130, 132, 133, 134, 136, 138, 140)):
				d[i][j] = 7
			case v == 147 && (!isFence(i, j-1) || !isFence(i, j+1) || !extends(g.at(i, j-1), 131, 132, 133, 135, 137, 138, 139) && !extends(g.at(i, j+1), 131, 132, 133, 135, 137, 138, 139)):
				d[i][j] = 7
			}
		}
	}
}

// reachable floods the walkable cells from p (4 directions).
func (l *Level) reachable(p [2]int) map[[2]int]bool {
	seen := map[[2]int]bool{p: true}
	for queue := [][2]int{p}; len(queue) > 0; queue = queue[1:] {
		for _, d := range l3Dirs {
			if q := [2]int{queue[0][0] + d[0], queue[0][1] + d[1]}; l.Walkable(q[0], q[1]) && !seen[q] {
				seen[q] = true
				queue = append(queue, q)
			}
		}
	}
	return seen
}

func (l *Level) walkableCount() (n int) {
	for x := 0; x < LevelSize; x++ {
		for y := 0; y < LevelSize; y++ {
			if l.Walkable(x, y) {
				n++
			}
		}
	}
	return
}
