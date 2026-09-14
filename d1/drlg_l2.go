package d1

import "math/rand"

// l2 holds drlg_l2.cpp's globals.
type l2 struct {
	drlg
	pre   [40][40]byte // predungeon: ' ' void, '#' wall, '.' floor, 'D' door, ',' hall, 'A'-'E' room corners
	rooms []l2Room     // RoomList, 1-based like D1 (rooms[0] is unused)
	halls []l2Hall
}

type l2Room struct{ x1, y1, x2, y2 int }
type l2Hall struct{ x1, y1, x2, y2, dir int }

var (
	l2USTAIRS    = []byte{4, 4, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 72, 77, 0, 0, 76, 0, 0, 0, 0, 0, 0}
	l2DSTAIRS    = []byte{4, 4, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 48, 71, 0, 0, 50, 78, 0, 0, 0, 0, 0}
	l2WARPSTAIRS = []byte{4, 4, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 0, 0, 0, 0, 0, 158, 160, 0, 0, 159, 0, 0, 0, 0, 0, 0}
	dirXadd      = [5]int{0, 0, 1, 0, -1}
	dirYadd      = [5]int{0, -1, 0, 1, 0}
)

// l2Patterns ports Patterns: a 3×3 predungeon neighbourhood (0 any, 1 '#', 2 '.', 3 'D', 4 ' ', 5 'D'/'.',
// 6 'D'/'#', 7 ' '/'.', 8 'D'/'#'/'.') and the megatile it becomes. Later matches win.
var l2Patterns = [][10]byte{
	{0, 0, 0, 0, 0, 0, 0, 0, 0, 3},
	{0, 0, 0, 0, 2, 0, 0, 0, 0, 3},
	{0, 7, 0, 0, 1, 0, 0, 5, 0, 2},
	{0, 5, 0, 0, 1, 0, 0, 7, 0, 2},
	{0, 0, 0, 7, 1, 5, 0, 0, 0, 1},
	{0, 0, 0, 5, 1, 7, 0, 0, 0, 1},
	{0, 1, 0, 0, 3, 0, 0, 1, 0, 4},
	{0, 0, 0, 1, 3, 1, 0, 0, 0, 5},
	{0, 6, 0, 6, 1, 0, 0, 0, 0, 6},
	{0, 6, 0, 0, 1, 6, 0, 0, 0, 9},
	{0, 0, 0, 6, 1, 0, 0, 6, 0, 7},
	{0, 0, 0, 0, 1, 6, 0, 6, 0, 8},
	{0, 6, 0, 6, 6, 0, 8, 6, 0, 7},
	{0, 6, 8, 6, 6, 6, 0, 0, 0, 9},
	{0, 6, 0, 0, 6, 6, 0, 6, 8, 8},
	{6, 6, 6, 6, 6, 6, 0, 6, 0, 8},
	{2, 6, 6, 6, 6, 6, 0, 6, 0, 8},
	{7, 7, 7, 6, 6, 6, 0, 6, 0, 8},
	{6, 6, 2, 6, 6, 6, 0, 6, 0, 8},
	{6, 2, 6, 6, 6, 6, 0, 6, 0, 8},
	{2, 6, 6, 6, 6, 6, 0, 6, 0, 8},
	{6, 7, 7, 6, 6, 6, 0, 6, 0, 8},
	{4, 4, 6, 6, 6, 6, 2, 6, 2, 8},
	{2, 2, 2, 2, 6, 2, 2, 6, 2, 7},
	{2, 2, 2, 2, 6, 2, 6, 6, 6, 7},
	{2, 2, 6, 2, 6, 6, 2, 2, 6, 9},
	{2, 6, 2, 2, 6, 2, 2, 2, 2, 6},
	{2, 2, 2, 2, 6, 6, 2, 2, 2, 9},
	{2, 2, 2, 6, 6, 2, 2, 2, 2, 6},
	{2, 2, 0, 2, 6, 6, 2, 2, 0, 9},
	{0, 0, 0, 0, 4, 0, 0, 0, 0, 12},
	{0, 1, 0, 0, 1, 4, 0, 1, 0, 10},
	{0, 0, 0, 1, 1, 1, 0, 4, 0, 11},
	{0, 0, 0, 6, 1, 4, 0, 1, 0, 14},
	{0, 6, 0, 1, 1, 0, 0, 4, 0, 16},
	{0, 6, 0, 0, 1, 1, 0, 4, 0, 15},
	{0, 0, 0, 0, 1, 1, 0, 1, 4, 13},
	{8, 8, 8, 8, 1, 1, 0, 1, 1, 13},
	{8, 8, 4, 8, 1, 1, 0, 1, 1, 10},
	{0, 0, 0, 1, 1, 1, 1, 1, 1, 11},
	{1, 1, 1, 1, 1, 1, 2, 2, 8, 2},
	{0, 1, 0, 1, 1, 4, 1, 1, 0, 16},
	{0, 0, 0, 1, 1, 1, 1, 1, 4, 11},
	{1, 1, 4, 1, 1, 1, 0, 2, 2, 2},
	{1, 1, 1, 1, 1, 1, 6, 2, 6, 2},
	{4, 1, 1, 1, 1, 1, 6, 2, 6, 2},
	{2, 2, 2, 1, 1, 1, 4, 1, 1, 11},
	{4, 1, 1, 1, 1, 1, 2, 2, 2, 2},
	{1, 1, 4, 1, 1, 1, 2, 2, 1, 2},
	{4, 1, 1, 1, 1, 1, 1, 2, 2, 2},
	{2, 2, 6, 1, 1, 1, 4, 1, 1, 11},
	{4, 1, 1, 1, 1, 1, 2, 2, 6, 2},
	{1, 2, 2, 1, 1, 1, 4, 1, 1, 11},
	{0, 1, 1, 0, 1, 1, 0, 1, 1, 10},
	{2, 1, 1, 3, 1, 1, 2, 1, 1, 14},
	{1, 1, 0, 1, 1, 2, 1, 1, 0, 1},
	{0, 4, 0, 1, 1, 1, 0, 1, 1, 14},
	{4, 1, 0, 1, 1, 0, 1, 1, 0, 1},
	{0, 1, 0, 4, 1, 1, 0, 1, 1, 15},
	{1, 1, 1, 1, 1, 1, 0, 2, 2, 2},
	{0, 1, 1, 2, 1, 1, 2, 1, 4, 10},
	{2, 1, 1, 1, 1, 1, 0, 4, 0, 16},
	{1, 1, 4, 1, 1, 2, 0, 1, 2, 1},
	{2, 1, 1, 2, 1, 1, 1, 1, 4, 10},
	{1, 1, 2, 1, 1, 2, 4, 1, 8, 1},
	{2, 1, 4, 1, 1, 1, 4, 4, 1, 16},
	{2, 1, 1, 1, 1, 1, 1, 1, 1, 16},
	{1, 1, 2, 1, 1, 1, 1, 1, 1, 15},
	{1, 1, 1, 1, 1, 1, 2, 1, 1, 14},
	{4, 1, 1, 1, 1, 1, 2, 1, 1, 14},
	{1, 1, 1, 1, 1, 1, 1, 1, 2, 8}}

func (g *l2) p(x, y int) byte {
	if x < 0 || y < 0 || x >= 40 || y >= 40 {
		return 0
	}
	return g.pre[x][y]
}

func (g *l2) setP(x, y int, v byte) {
	if x >= 0 && y >= 0 && x < 40 && y < 40 {
		g.pre[x][y] = v
	}
}

func (g *l2) defineRoom(x1, y1, x2, y2 int) {
	g.setP(x1, y1, 'C')
	g.setP(x1, y2, 'E')
	g.setP(x2, y1, 'B')
	g.setP(x2, y2, 'A')
	g.rooms = append(g.rooms, l2Room{x1, y1, x2, y2})
	for i := x1 + 1; i <= x2-1; i++ {
		g.setP(i, y1, '#')
		g.setP(i, y2, '#')
	}
	for j := y1 + 1; j <= y2-1; j++ {
		g.setP(x1, j, '#')
		g.setP(x2, j, '#')
		for i := x1 + 1; i < x2; i++ {
			g.setP(i, j, '.')
		}
	}
}

func (g *l2) createDoorType(x, y int) {
	if g.p(x-1, y) == 'D' || g.p(x+1, y) == 'D' || g.p(x, y-1) == 'D' || g.p(x, y+1) == 'D' {
		return
	}
	if c := g.p(x, y); c == 'A' || c == 'B' || c == 'C' || c == 'E' {
		return
	}
	g.setP(x, y, 'D')
}

func (g *l2) placeHallExt(x, y int) {
	if g.p(x, y) == ' ' {
		g.setP(x, y, ',')
	}
}

// createRoom ports CreateRoom. ponytail: quest set rooms (ForceHW) come with their quests.
func (g *l2) createRoom(x1, y1, x2, y2, rdest, hdir int) {
	const areaMin, roomMax, roomMin = 2, 10, 4
	if len(g.rooms) > 80 {
		return
	}
	aw, ah := x2-x1, y2-y1
	if aw < areaMin || ah < areaMin {
		return
	}
	var rw, rh int
	switch {
	case aw > roomMax:
		rw = g.random(roomMax-roomMin) + roomMin
	case aw > roomMin:
		rw = g.random(aw-roomMin) + roomMin
	default:
		rw = aw
	}
	switch {
	case ah > roomMax:
		rh = g.random(roomMax-roomMin) + roomMin
	case ah > roomMin:
		rh = g.random(ah-roomMin) + roomMin
	default:
		rh = ah
	}
	rx1 := g.random(x2-x1) + x1
	ry1 := g.random(y2-y1) + y1
	rx2, ry2 := rw+rx1, rh+ry1
	if rx2 > x2 {
		rx2, rx1 = x2, x2-rw
	}
	if ry2 > y2 {
		ry2, ry1 = y2, y2-rh
	}
	rx1, ry1, rx2, ry2 = clamp(rx1, 1, 38), clamp(ry1, 1, 38), clamp(rx2, 1, 38), clamp(ry2, 1, 38)
	g.defineRoom(rx1, ry1, rx2, ry2)
	rid := len(g.rooms) - 1
	if rdest != 0 {
		d := g.rooms[rdest]
		var h l2Hall
		switch hdir {
		case 1:
			h.x1, h.y1 = g.random(rx2-rx1-2)+rx1+1, ry1
			h.x2, h.y2 = g.random(d.x2-d.x1-2)+d.x1+1, d.y2
		case 3:
			h.x1, h.y1 = g.random(rx2-rx1-2)+rx1+1, ry2
			h.x2, h.y2 = g.random(d.x2-d.x1-2)+d.x1+1, d.y1
		case 2:
			h.x1, h.y1 = rx2, g.random(ry2-ry1-2)+ry1+1
			h.x2, h.y2 = d.x1, g.random(d.y2-d.y1-2)+d.y1+1
		case 4:
			h.x1, h.y1 = rx1, g.random(ry2-ry1-2)+ry1+1
			h.x2, h.y2 = d.x2, g.random(d.y2-d.y1-2)+d.y1+1
		}
		h.dir = hdir
		g.halls = append(g.halls, h)
	}
	if rh > rw {
		g.createRoom(x1+2, y1+2, rx1-2, ry2-2, rid, 2)
		g.createRoom(rx2+2, ry1+2, x2-2, y2-2, rid, 4)
		g.createRoom(x1+2, ry2+2, rx2-2, y2-2, rid, 1)
		g.createRoom(rx1+2, y1+2, x2-2, ry1-2, rid, 3)
	} else {
		g.createRoom(x1+2, y1+2, rx2-2, ry1-2, rid, 3)
		g.createRoom(rx1+2, ry2+2, x2-2, y2-2, rid, 1)
		g.createRoom(x1+2, ry1+2, rx1-2, y2-2, rid, 2)
		g.createRoom(rx2+2, y1+2, x2-2, ry2-2, rid, 4)
	}
}

// connectHall ports ConnectHall: a random walk from one room's wall to the destination's. D1 can bounce off a room
// corner forever; after 2000 steps we give up and the level is generated again.
func (g *l2) connectHall(h l2Hall) bool {
	x1, y1, x2, y2, cur := h.x1, h.y1, h.x2, h.y2, h.dir
	minusFlag, plusFlag := g.random(100), g.random(100)
	origX1, origY1 := x1, y1
	g.createDoorType(x1, y1)
	g.createDoorType(x2, y2)
	x2 -= dirXadd[cur]
	y2 -= dirYadd[cur]
	g.setP(x2, y2, ',')
	inRoom := false
	for steps := 0; x1 != x2 || y1 != y2; steps++ {
		if steps == 2000 {
			return false
		}
		if x1 >= 38 && cur == 2 {
			cur = 4
		}
		if y1 >= 38 && cur == 3 {
			cur = 1
		}
		if x1 <= 1 && cur == 4 {
			cur = 2
		}
		if y1 <= 1 && cur == 1 {
			cur = 3
		}
		switch g.p(x1, y1) {
		case 'C':
			if cur == 1 || cur == 4 {
				cur = 2
			}
		case 'B':
			if cur == 1 || cur == 2 {
				cur = 3
			}
		case 'E':
			if cur == 4 || cur == 3 {
				cur = 1
			}
		case 'A':
			if cur == 2 || cur == 3 {
				cur = 4
			}
		}
		x1 += dirXadd[cur]
		y1 += dirYadd[cur]
		if g.p(x1, y1) == ' ' {
			if inRoom {
				g.createDoorType(x1-dirXadd[cur], y1-dirYadd[cur])
			} else {
				if minusFlag < 50 {
					if cur != 1 && cur != 3 {
						g.placeHallExt(x1, y1-1)
					} else {
						g.placeHallExt(x1-1, y1)
					}
				}
				if plusFlag < 50 {
					if cur != 1 && cur != 3 {
						g.placeHallExt(x1, y1+1)
					} else {
						g.placeHallExt(x1+1, y1)
					}
				}
			}
			g.setP(x1, y1, ',')
			inRoom = false
		} else {
			if !inRoom && g.p(x1, y1) == '#' {
				g.createDoorType(x1, y1)
			}
			if g.p(x1, y1) != ',' {
				inRoom = true
			}
		}
		dx, dy := iabs(x2-x1), iabs(y2-y1)
		horiz := func(far bool) {
			if x2 <= x1 || far {
				cur = 4
			} else {
				cur = 2
			}
		}
		vert := func(far bool) {
			if y2 <= y1 || far {
				cur = 1
			} else {
				cur = 3
			}
		}
		if dx > dy {
			if g.random(100) < min(2*dx, 30) {
				horiz(x1 >= 40)
			}
		} else if g.random(100) < min(5*dy, 80) {
			vert(y1 >= 40)
		}
		if dy < 10 && x1 == x2 && (cur == 2 || cur == 4) {
			vert(y1 >= 40)
		}
		if dx < 10 && y1 == y2 && (cur == 1 || cur == 3) {
			horiz(x1 >= 40)
		}
		if dy == 1 && dx > 1 && (cur == 1 || cur == 3) {
			horiz(x1 >= 40)
		}
		if dx == 1 && dy > 1 && (cur == 2 || cur == 4) {
			vert(x1 >= 40)
		}
		if dx == 0 && g.p(x1, y1) != ' ' && (cur == 2 || cur == 4) {
			if x2 <= origX1 || x1 >= 40 {
				cur = 1
			} else {
				cur = 3
			}
		}
		if dy == 0 && g.p(x1, y1) != ' ' && (cur == 1 || cur == 3) {
			if y2 <= origY1 || y1 >= 40 {
				cur = 4
			} else {
				cur = 2
			}
		}
	}
	return true
}

func (g *l2) patternCheck(i, j int) {
	for _, pat := range l2Patterns {
		ok := true
		for l := 0; l < 9 && ok; l++ {
			x, y := i-1+l%3, j-1+l/3
			if x < 0 || y < 0 || x >= 40 || y >= 40 {
				continue
			}
			c := g.pre[x][y]
			switch pat[l] {
			case 1:
				ok = c == '#'
			case 2:
				ok = c == '.'
			case 3:
				ok = c == 'D'
			case 4:
				ok = c == ' '
			case 5:
				ok = c == 'D' || c == '.'
			case 6:
				ok = c == 'D' || c == '#'
			case 7:
				ok = c == ' ' || c == '.'
			case 8:
				ok = c == 'D' || c == '#' || c == '.'
			}
		}
		if ok {
			g.dungeon[i][j] = pat[9]
		}
	}
}

func (g *l2) numNoChar() (t int) {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.pre[i][j] == ' ' {
				t++
			}
		}
	}
	return
}

func (g *l2) drawRoom(x1, y1, x2, y2 int) {
	for jj := y1; jj <= y2; jj++ {
		for ii := x1; ii <= x2; ii++ {
			g.setP(ii, jj, '.')
		}
	}
	for jj := y1; jj <= y2; jj++ {
		g.setP(x1, jj, '#')
		g.setP(x2, jj, '#')
	}
	for ii := x1; ii <= x2; ii++ {
		g.setP(ii, y1, '#')
		g.setP(ii, y2, '#')
	}
}

func (g *l2) knockWalls(x1, y1, x2, y2 int) {
	for ii := x1 + 1; ii < x2; ii++ {
		if g.p(ii, y1-1) == '.' && g.p(ii, y1+1) == '.' {
			g.setP(ii, y1, '.')
		}
		if g.p(ii, y2-1) == '.' && g.p(ii, y2+1) == '.' {
			g.setP(ii, y2, '.')
		}
		if g.p(ii, y1-1) == 'D' {
			g.setP(ii, y1-1, '.')
		}
		if g.p(ii, y2+1) == 'D' {
			g.setP(ii, y2+1, '.')
		}
	}
	for jj := y1 + 1; jj < y2; jj++ {
		if g.p(x1-1, jj) == '.' && g.p(x1+1, jj) == '.' {
			g.setP(x1, jj, '.')
		}
		if g.p(x2-1, jj) == '.' && g.p(x2+1, jj) == '.' {
			g.setP(x2, jj, '.')
		}
		if g.p(x1-1, jj) == 'D' {
			g.setP(x1-1, jj, '.')
		}
		if g.p(x2+1, jj) == 'D' {
			g.setP(x2+1, jj, '.')
		}
	}
}

// fillVoids ports DL2_FillVoids: grow extra rooms out of walls that face large void areas.
func (g *l2) fillVoids() bool {
	for to := 0; g.numNoChar() > 700 && to < 100; to++ {
		xx, yy := g.random(38)+1, g.random(38)+1
		if g.pre[xx][yy] != '#' {
			continue
		}
		var xf1, xf2, yf1, yf2 bool
		p := g.p
		switch {
		case p(xx-1, yy) == ' ' && p(xx+1, yy) == '.':
			if p(xx+1, yy-1) == '.' && p(xx+1, yy+1) == '.' && p(xx-1, yy-1) == ' ' && p(xx-1, yy+1) == ' ' {
				xf1, yf1, yf2 = true, true, true
			}
		case p(xx+1, yy) == ' ' && p(xx-1, yy) == '.':
			if p(xx-1, yy-1) == '.' && p(xx-1, yy+1) == '.' && p(xx+1, yy-1) == ' ' && p(xx+1, yy+1) == ' ' {
				xf2, yf1, yf2 = true, true, true
			}
		case p(xx, yy-1) == ' ' && p(xx, yy+1) == '.':
			if p(xx-1, yy+1) == '.' && p(xx+1, yy+1) == '.' && p(xx-1, yy-1) == ' ' && p(xx+1, yy-1) == ' ' {
				yf1, xf1, xf2 = true, true, true
			}
		case p(xx, yy+1) == ' ' && p(xx, yy-1) == '.':
			if p(xx-1, yy-1) == '.' && p(xx+1, yy-1) == '.' && p(xx-1, yy+1) == ' ' && p(xx+1, yy+1) == ' ' {
				yf2, xf1, xf2 = true, true, true
			}
		}
		// DL2_Cont
		if xf1 && xf2 && yf1 && yf2 || !(xf1 && xf2 && (yf1 || yf2)) && !(yf1 && yf2 && (xf1 || xf2)) {
			continue
		}
		x1, x2, y1, y2 := xx, xx, yy, yy
		if xf1 {
			x1--
		}
		if xf2 {
			x2++
		}
		if yf1 {
			y1--
		}
		if yf2 {
			y2++
		}
		switch {
		case !xf1:
			for yf1 || yf2 {
				if y1 == 0 {
					yf1 = false
				}
				if y2 == 39 {
					yf2 = false
				}
				if y2-y1 >= 14 {
					yf1, yf2 = false, false
				}
				if yf1 {
					y1--
				}
				if yf2 {
					y2++
				}
				if p(x2, y1) != ' ' {
					yf1 = false
				}
				if p(x2, y2) != ' ' {
					yf2 = false
				}
			}
			y1 += 2
			y2 -= 2
			if y2-y1 > 5 {
				for xf2 {
					if x2 == 39 || x2-x1 >= 12 {
						xf2 = false
					}
					for jj := y1; jj <= y2; jj++ {
						if p(x2, jj) != ' ' {
							xf2 = false
						}
					}
					if xf2 {
						x2++
					}
				}
				x2 -= 2
				if x2-x1 > 5 {
					g.drawRoom(x1, y1, x2, y2)
					g.knockWalls(x1, y1, x2, y2)
				}
			}
		case !xf2:
			for yf1 || yf2 {
				if y1 == 0 {
					yf1 = false
				}
				if y2 == 39 {
					yf2 = false
				}
				if y2-y1 >= 14 {
					yf1, yf2 = false, false
				}
				if yf1 {
					y1--
				}
				if yf2 {
					y2++
				}
				if p(x1, y1) != ' ' {
					yf1 = false
				}
				if p(x1, y2) != ' ' {
					yf2 = false
				}
			}
			y1 += 2
			y2 -= 2
			if y2-y1 > 5 {
				for xf1 {
					if x1 == 0 || x2-x1 >= 12 {
						xf1 = false
					}
					for jj := y1; jj <= y2; jj++ {
						if p(x1, jj) != ' ' {
							xf1 = false
						}
					}
					if xf1 {
						x1--
					}
				}
				x1 += 2
				if x2-x1 > 5 {
					g.drawRoom(x1, y1, x2, y2)
					g.knockWalls(x1, y1, x2, y2)
				}
			}
		case !yf1:
			for xf1 || xf2 {
				if x1 == 0 {
					xf1 = false
				}
				if x2 == 39 {
					xf2 = false
				}
				if x2-x1 >= 14 {
					xf1, xf2 = false, false
				}
				if xf1 {
					x1--
				}
				if xf2 {
					x2++
				}
				if p(x1, y2) != ' ' {
					xf1 = false
				}
				if p(x2, y2) != ' ' {
					xf2 = false
				}
			}
			x1 += 2
			x2 -= 2
			if x2-x1 > 5 {
				for yf2 {
					if y2 == 39 || y2-y1 >= 12 {
						yf2 = false
					}
					for ii := x1; ii <= x2; ii++ {
						if p(ii, y2) != ' ' {
							yf2 = false
						}
					}
					if yf2 {
						y2++
					}
				}
				y2 -= 2
				if y2-y1 > 5 {
					g.drawRoom(x1, y1, x2, y2)
					g.knockWalls(x1, y1, x2, y2)
				}
			}
		case !yf2:
			for xf1 || xf2 {
				if x1 == 0 {
					xf1 = false
				}
				if x2 == 39 {
					xf2 = false
				}
				if x2-x1 >= 14 {
					xf1, xf2 = false, false
				}
				if xf1 {
					x1--
				}
				if xf2 {
					x2++
				}
				if p(x1, y1) != ' ' {
					xf1 = false
				}
				if p(x2, y1) != ' ' {
					xf2 = false
				}
			}
			x1 += 2
			x2 -= 2
			if x2-x1 > 5 {
				for yf1 {
					if y1 == 0 || y2-y1 >= 12 {
						yf1 = false
					}
					for ii := x1; ii <= x2; ii++ {
						if p(ii, y1) != ' ' {
							yf1 = false
						}
					}
					if yf1 {
						y1--
					}
				}
				y1 += 2
				if y2-y1 > 5 {
					g.drawRoom(x1, y1, x2, y2)
					g.knockWalls(x1, y1, x2, y2)
				}
			}
		}
	}
	return g.numNoChar() <= 700
}

func (g *l2) createDungeon() bool {
	g.rooms = []l2Room{{}}
	g.createRoom(2, 2, 39, 39, 0, 0)
	for len(g.halls) > 0 {
		h := g.halls[0]
		g.halls = g.halls[1:]
		if !g.connectHall(h) {
			return false
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			switch g.pre[i][j] {
			case 'A', 'B', 'C', 'E':
				g.pre[i][j] = '#'
			case ',':
				g.pre[i][j] = '.'
				for _, d := range [8][2]int{{-1, -1}, {-1, 0}, {-1, 1}, {1, -1}, {1, 0}, {1, 1}, {0, -1}, {0, 1}} {
					if g.p(i+d[0], j+d[1]) == ' ' {
						g.setP(i+d[0], j+d[1], '#')
					}
				}
			}
		}
	}
	if !g.fillVoids() {
		return false
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			g.patternCheck(i, j)
		}
	}
	return true
}

func (g *l2) tileFix() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.at(i, j) == 1 && g.at(i, j+1) == 3 {
				g.set(i, j+1, 1)
			}
			if g.at(i, j) == 3 && g.at(i, j+1) == 1 {
				g.set(i, j+1, 3)
			}
			if g.at(i, j) == 3 && g.at(i+1, j) == 7 {
				g.set(i+1, j, 3)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 3 {
				g.set(i+1, j, 2)
			}
			if g.at(i, j) == 11 && g.at(i+1, j) == 14 {
				g.set(i+1, j, 16)
			}
		}
	}
}

// lockoutFix ports L2LockoutFix: a wall between two floors gets a door if none of its megatiles has one.
func (g *l2) lockoutFix() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.dungeon[i][j] == 4 && g.at(i-1, j) != 3 {
				g.dungeon[i][j] = 1
			}
			if g.dungeon[i][j] == 5 && g.at(i, j-1) != 3 {
				g.dungeon[i][j] = 2
			}
		}
	}
	for j := 1; j < 39; j++ {
		for i := 1; i < 39; i++ {
			if c := g.dungeon[i][j]; (c == 2 || c == 5) && g.dungeon[i][j-1] == 3 && g.dungeon[i][j+1] == 3 {
				doorok := false
				for ; (g.at(i, j) == 2 || g.at(i, j) == 5) && g.at(i, j-1) == 3 && g.at(i, j+1) == 3; i++ {
					doorok = doorok || g.at(i, j) == 5
				}
				if !doorok {
					g.set(i-1, j, 5)
				}
			}
		}
	}
	for j := 1; j < 39; j++ {
		for i := 1; i < 39; i++ {
			if c := g.dungeon[j][i]; (c == 1 || c == 4) && g.dungeon[j-1][i] == 3 && g.dungeon[j+1][i] == 3 {
				doorok := false
				for ; (g.at(j, i) == 1 || g.at(j, i) == 4) && g.at(j-1, i) == 3 && g.at(j+1, i) == 3; i++ {
					doorok = doorok || g.at(j, i) == 4
				}
				if !doorok {
					g.set(j, i-1, 4)
				}
			}
		}
	}
}

// GenerateL2 ports DRLG_L2 (Catacombs) down to the grid. Walls are whole megatiles here: D1's catacomb rooms
// are walled on all four sides. ponytail: arches, rubble and shadows (PlaceRndSet, Subs, Shadows) are skipped,
// they don't change what's walkable; quest set pieces come with their quests.
func GenerateL2(rng *rand.Rand, dlvl int) *Level {
	for {
		g := &l2{drlg: newDrlg(rng)}
		for j := range g.pre {
			for i := range g.pre[j] {
				g.pre[j][i] = ' '
			}
		}
		if !g.createDungeon() {
			continue
		}
		g.tileFix()
		g.floodTVal(3)
		// L2TransFix only copies transvals into wall halves, which stay walls here.
		ux, uy := g.placeMiniSet(l2USTAIRS, 200, 0)
		if ux < 0 {
			continue
		}
		dx, dy := g.placeMiniSet(l2DSTAIRS, 200, 0)
		if dx < 0 {
			continue
		}
		wx, wy := -1, -1
		if dlvl == 5 {
			if wx, wy = g.placeMiniSet(l2WARPSTAIRS, 200, 0); wx < 0 {
				continue
			}
		}
		g.lockoutFix()
		g.placeThemeRooms(false, 6, 10, 3, 0, false, func(t themeLoc) {
			g.drawThemeRoom(t, 2, 1, 3, [4]byte{8, 7, 9, 6})
			if g.random(2) == 0 {
				g.set(t.x+t.w-1, t.y+t.h/2, 4)
			} else {
				g.set(t.x+t.w/2, t.y+t.h-1, 5)
			}
		})
		lvl := g.level()
		for j := 0; j < 40; j++ {
			for i := 0; i < 40; i++ {
				switch c := g.dungeon[i][j]; c {
				case 3, 72, 77, 76, 48, 71, 50, 78, 158, 160, 159:
					lvl.megaCells(i, j, CellFloor)
				case 4: // a door in a vertical wall
					lvl.Cells[2*i][2*j], lvl.Cells[2*i][2*j+1] = CellDoor, CellDoor
					lvl.Cells[2*i+1][2*j], lvl.Cells[2*i+1][2*j+1] = CellFloor, CellFloor
				case 5:
					lvl.Cells[2*i][2*j], lvl.Cells[2*i+1][2*j] = CellDoor, CellDoor
					lvl.Cells[2*i][2*j+1], lvl.Cells[2*i+1][2*j+1] = CellFloor, CellFloor
				}
			}
		}
		lvl.StairsUp = [2]int{2*ux + 3, 2*uy + 3}
		lvl.StairsDn = [2]int{2*dx + 3, 2*dy + 3}
		if wx >= 0 {
			lvl.Warp = [2]int{2*wx + 3, 2*wy + 3}
		}
		return lvl
	}
}
