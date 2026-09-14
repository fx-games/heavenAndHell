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
// (2i+a, 2j+b) is floor map [i+a][j+b]. ponytail: lava pools, rivers, wooden fences, islands and cracks are
// skipped (pools and rivers would add unwalkable lava, fences extra walls); the Anvil set piece is a deferred quest.
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
		lvl := g.level()
		for x := 0; x < LevelSize; x++ {
			for y := 0; y < LevelSize; y++ {
				if mask[min((x+1)/2, 39)][min((y+1)/2, 39)] != 0 {
					lvl.Cells[x][y] = CellFloor
				}
			}
		}
		for n, t := range fences { // the theme rooms' fences are walls, their gate is open
			for j := t.y; j < t.y+t.h; j++ {
				for i := t.x; i < t.x+t.w; i++ {
					if i == t.x || j == t.y || i == t.x+t.w-1 || j == t.y+t.h-1 {
						lvl.megaCells(i, j, CellWall)
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
		return lvl
	}
}
