package d1

// drlg_l2.cpp: the Catacombs.

type l2Hall struct {
	begin, end point
	dir        int // 1 up, 2 right, 3 down, 4 left
}

var l2DirAdd = [5]point{{0, 0}, {0, -1}, {1, 0}, {0, 1}, {-1, 0}}

// p reads predungeon (the ASCII layout: ' ' void, '.' floor, '#' wall, 'D' door, ',' hall, A/B/C/E room corners).
func (g *gen) p(x, y int) byte {
	if x < 0 || y < 0 || x >= 40 || y >= 40 {
		return 0
	}
	return g.l2.pre[x][y]
}

func (g *gen) setP(x, y int, v byte) {
	if x >= 0 && y >= 0 && x < 40 && y < 40 {
		g.l2.pre[x][y] = v
	}
}

func (g *gen) l2ApplyShadows() {
	for y := 1; y < 40; y++ {
		for x := 1; x < 40; x++ {
			if l2BSTypes[g.d(x, y)] != 6 && l2BSTypes[g.d(x, y)] != 9 || l2BSTypes[g.d(x-1, y-1)] != 3 || l2BSTypes[g.d(x-1, y)] != 3 {
				continue
			}
			g.sd(x-1, y-1, 48) // SPATSL2: {6 or 9, 3, 0, 3, 48, 0, 50}
			g.sd(x-1, y, 50)
		}
	}
}

func (g *gen) l2PlaceMiniSetRandom(m *miniset, rndper int) {
	for sy := 0; sy < 40-m.h; sy++ {
		for sx := 0; sx < 40-m.w; sx++ {
			if g.setPieceRoom.contains(sx, sy) || !m.matches(g, sx, sy, true) {
				continue
			}
			found := true
			for yy := max(sy-m.h, 0); yy < min(sy+2*m.h, 40) && found; yy++ {
				for xx := max(sx-m.w, 0); xx < min(sx+2*m.w, 40); xx++ {
					if g.d(xx, yy) == m.replace[0][0] {
						found = false
						break
					}
				}
			}
			if !found || g.rnd(100) >= rndper {
				continue
			}
			m.place(g, sx, sy, false)
		}
	}
}

func (g *gen) l2PlaceMiniSetRandom1x1(search, replace byte, rndper int) {
	g.l2PlaceMiniSetRandom(&miniset{w: 1, h: 1, search: [6][6]byte{{search}}, replace: [6][6]byte{{replace}}}, rndper)
}

func (g *gen) l2InitSetPiece() {
	var name string
	switch {
	case g.available(QBlind):
		name = "l2/blind2"
	case g.available(QBlood):
		name = "l2/blood1"
	case g.available(QSchamb):
		name = "l2/bonestr2"
	default:
		return
	}
	d := g.loadDun(name)
	g.placeDunTiles(d, g.setPieceRoom.x, g.setPieceRoom.y, 3)
	g.setPiece = rect{g.setPieceRoom.x, g.setPieceRoom.y, d.w(), d.h()}
}

func (g *gen) l2MapRoom(x1, y1, x2, y2 int) {
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

func (g *gen) l2DefineRoom(x1, y1, x2, y2 int, forceHW bool) {
	g.setP(x1, y1, 'C')
	g.setP(x1, y2, 'E')
	g.setP(x2, y1, 'B')
	g.setP(x2, y2, 'A')
	g.l2.nRoomCnt++
	g.l2.rooms[g.l2.nRoomCnt].tl, g.l2.rooms[g.l2.nRoomCnt].br = point{x1, y1}, point{x2, y2}
	if forceHW {
		for i := x1; i < x2; i++ {
			for i < y2 { // D1 bug: loops i against y2 and always uses y1
				g.setProt(i, y1)
				i++
			}
		}
	}
	for i := x1 + 1; i <= x2-1; i++ {
		g.setP(i, y1, '#')
		g.setP(i, y2, '#')
	}
	y2--
	for j := y1 + 1; j <= y2; j++ {
		g.setP(x1, j, '#')
		g.setP(x2, j, '#')
		for i := x1 + 1; i < x2; i++ {
			g.setP(i, j, '.')
		}
	}
}

func (g *gen) l2CreateDoorType(x, y int) {
	if g.p(x-1, y) == 'D' || g.p(x+1, y) == 'D' || g.p(x, y-1) == 'D' || g.p(x, y+1) == 'D' {
		return
	}
	switch g.p(x, y) {
	case 'A', 'B', 'C', 'E':
		return
	}
	g.setP(x, y, 'D')
}

func (g *gen) l2PlaceHallExt(x, y int) {
	if g.p(x, y) == ' ' {
		g.setP(x, y, ',')
	}
}

// l2CreateRoom ports CreateRoom: a random room in the area, then the four areas around it; forced sizes are
// the quest set piece's room.
func (g *gen) l2CreateRoom(x1, y1, x2, y2, rdest, hdir int, forceHW bool, fh, fw int) {
	const areaMin, roomMax, roomMin = 2, 10, 4
	if g.l2.nRoomCnt >= 80 {
		return
	}
	aw, ah := x2-x1, y2-y1
	if aw < areaMin || ah < areaMin {
		return
	}
	var rw, rh int
	switch {
	case aw > roomMax:
		rw = g.rnd(roomMax-roomMin) + roomMin
	case aw > roomMin:
		rw = g.rnd(aw-roomMin) + roomMin
	default:
		rw = aw
	}
	switch {
	case ah > roomMax:
		rh = g.rnd(roomMax-roomMin) + roomMin
	case ah > roomMin:
		rh = g.rnd(ah-roomMin) + roomMin
	default:
		rh = ah
	}
	if forceHW {
		rw, rh = fw, fh
	}
	rx1 := g.rnd(x2-x1) + x1
	ry1 := g.rnd(y2-y1) + y1
	rx2, ry2 := rw+rx1, rh+ry1
	if rx2 > x2 {
		rx2 = x2
		rx1 = x2 - rw
	}
	if ry2 > y2 {
		ry2 = y2
		ry1 = y2 - rh
	}
	rx1, ry1, rx2, ry2 = clamp(rx1, 1, 38), clamp(ry1, 1, 38), clamp(rx2, 1, 38), clamp(ry2, 1, 38)
	g.l2DefineRoom(rx1, ry1, rx2, ry2, forceHW)
	if forceHW {
		g.setPieceRoom = rect{rx1 + 2, ry1 + 2, rx2 - rx1 - 1, ry2 - ry1 - 1}
	}
	rid := g.l2.nRoomCnt
	if rdest != 0 {
		dst := g.l2.rooms[rdest]
		var h l2Hall
		switch hdir {
		case 1:
			h.begin = point{g.rnd(rx2-rx1-2) + rx1 + 1, ry1}
			h.end = point{g.rnd(dst.br.x-dst.tl.x-2) + dst.tl.x + 1, dst.br.y}
		case 3:
			h.begin = point{g.rnd(rx2-rx1-2) + rx1 + 1, ry2}
			h.end = point{g.rnd(dst.br.x-dst.tl.x-2) + dst.tl.x + 1, dst.tl.y}
		case 2:
			h.begin = point{rx2, g.rnd(ry2-ry1-2) + ry1 + 1}
			h.end = point{dst.tl.x, g.rnd(dst.br.y-dst.tl.y-2) + dst.tl.y + 1}
		case 4:
			h.begin = point{rx1, g.rnd(ry2-ry1-2) + ry1 + 1}
			h.end = point{dst.br.x, g.rnd(dst.br.y-dst.tl.y-2) + dst.tl.y + 1}
		}
		h.dir = hdir
		g.l2.halls = append(g.l2.halls, h)
	}
	if rh > rw {
		g.l2CreateRoom(x1+2, y1+2, rx1-2, ry2-2, rid, 2, false, 0, 0)
		g.l2CreateRoom(rx2+2, ry1+2, x2-2, y2-2, rid, 4, false, 0, 0)
		g.l2CreateRoom(x1+2, ry2+2, rx2-2, y2-2, rid, 1, false, 0, 0)
		g.l2CreateRoom(rx1+2, y1+2, x2-2, ry1-2, rid, 3, false, 0, 0)
	} else {
		g.l2CreateRoom(x1+2, y1+2, rx2-2, ry1-2, rid, 3, false, 0, 0)
		g.l2CreateRoom(rx1+2, ry2+2, x2-2, y2-2, rid, 1, false, 0, 0)
		g.l2CreateRoom(x1+2, ry1+2, rx1-2, y2-2, rid, 2, false, 0, 0)
		g.l2CreateRoom(rx2+2, y1+2, x2-2, ry2-2, rid, 4, false, 0, 0)
	}
}

func (g *gen) l2ConnectHall(h l2Hall) {
	x1, y1, x2, y2 := h.begin.x, h.begin.y, h.end.x, h.end.y
	minusFlag := g.rnd(100)
	plusFlag := g.rnd(100)
	origX1, origY1 := x1, y1
	g.l2CreateDoorType(x1, y1)
	g.l2CreateDoorType(x2, y2)
	currd := h.dir
	x2 -= l2DirAdd[currd].x
	y2 -= l2DirAdd[currd].y
	g.setP(x2, y2, ',')
	inRoom := false
	for done := false; !done; {
		if x1 >= 38 && currd == 2 {
			currd = 4
		}
		if y1 >= 38 && currd == 3 {
			currd = 1
		}
		if x1 <= 1 && currd == 4 {
			currd = 2
		}
		if y1 <= 1 && currd == 1 {
			currd = 3
		}
		if c := g.p(x1, y1); c == 'C' && (currd == 1 || currd == 4) {
			currd = 2
		}
		if c := g.p(x1, y1); c == 'B' && (currd == 1 || currd == 2) {
			currd = 3
		}
		if c := g.p(x1, y1); c == 'E' && (currd == 4 || currd == 3) {
			currd = 1
		}
		if c := g.p(x1, y1); c == 'A' && (currd == 2 || currd == 3) {
			currd = 4
		}
		x1 += l2DirAdd[currd].x
		y1 += l2DirAdd[currd].y
		if g.p(x1, y1) == ' ' {
			if inRoom {
				g.l2CreateDoorType(x1-l2DirAdd[currd].x, y1-l2DirAdd[currd].y)
			} else {
				if minusFlag < 50 {
					if currd != 1 && currd != 3 {
						g.l2PlaceHallExt(x1, y1-1)
					} else {
						g.l2PlaceHallExt(x1-1, y1)
					}
				}
				if plusFlag < 50 {
					if currd != 1 && currd != 3 {
						g.l2PlaceHallExt(x1, y1+1)
					} else {
						g.l2PlaceHallExt(x1+1, y1)
					}
				}
			}
			g.setP(x1, y1, ',')
			inRoom = false
		} else {
			if !inRoom && g.p(x1, y1) == '#' {
				g.l2CreateDoorType(x1, y1)
			}
			if g.p(x1, y1) != ',' {
				inRoom = true
			}
		}
		dx, dy := iabs(x2-x1), iabs(y2-y1)
		if dx > dy {
			if g.rnd(100) < min(2*dx, 30) {
				if x2 <= x1 || x1 >= 40 {
					currd = 4
				} else {
					currd = 2
				}
			}
		} else if g.rnd(100) < min(5*dy, 80) {
			if y2 <= y1 || y1 >= 40 {
				currd = 1
			} else {
				currd = 3
			}
		}
		if dy < 10 && x1 == x2 && (currd == 2 || currd == 4) {
			if y2 <= y1 || y1 >= 40 {
				currd = 1
			} else {
				currd = 3
			}
		}
		if dx < 10 && y1 == y2 && (currd == 1 || currd == 3) {
			if x2 <= x1 || x1 >= 40 {
				currd = 4
			} else {
				currd = 2
			}
		}
		if dy == 1 && dx > 1 && (currd == 1 || currd == 3) {
			if x2 <= x1 || x1 >= 40 {
				currd = 4
			} else {
				currd = 2
			}
		}
		if dx == 1 && dy > 1 && (currd == 2 || currd == 4) {
			if y2 <= y1 || x1 >= 40 {
				currd = 1
			} else {
				currd = 3
			}
		}
		if dx == 0 && g.p(x1, y1) != ' ' && (currd == 2 || currd == 4) {
			if x2 <= origX1 || x1 >= 40 {
				currd = 1
			} else {
				currd = 3
			}
		}
		if dy == 0 && g.p(x1, y1) != ' ' && (currd == 1 || currd == 3) {
			if y2 <= origY1 || y1 >= 40 {
				currd = 4
			} else {
				currd = 2
			}
		}
		done = x1 == x2 && y1 == y2
	}
}

func (g *gen) l2DoPatternCheck(i, j int) {
	for k := 0; l2Patterns[k][4] != 255; k++ {
		x, y := i-1, j-1
		ok := 254
		for l := 0; l < 9 && ok == 254; l++ {
			ok = 255
			if l == 3 || l == 6 {
				y++
				x = i - 1
			}
			if x >= 0 && x < 40 && y >= 0 && y < 40 {
				c := g.p(x, y)
				switch l2Patterns[k][l] {
				case 0:
					ok = 254
				case 1:
					if c == '#' {
						ok = 254
					}
				case 2:
					if c == '.' {
						ok = 254
					}
				case 4:
					if c == ' ' {
						ok = 254
					}
				case 3:
					if c == 'D' {
						ok = 254
					}
				case 5:
					if c == 'D' || c == '.' {
						ok = 254
					}
				case 6:
					if c == 'D' || c == '#' {
						ok = 254
					}
				case 7:
					if c == ' ' || c == '.' {
						ok = 254
					}
				case 8:
					if c == 'D' || c == '#' || c == '.' {
						ok = 254
					}
				}
			} else {
				ok = 254
			}
			x++
		}
		if ok == 254 {
			g.sd(i, j, byte(l2Patterns[k][9]))
		}
	}
}

func (g *gen) l2FixTilesPatterns() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			for _, r := range [...]tileRule{{1, 0, 1, 3, 1}, {3, 0, 1, 1, 3}, {3, 1, 0, 7, 3}, {2, 1, 0, 3, 2}, {11, 1, 0, 14, 16}} {
				g.applyRule(i, j, r)
			}
		}
	}
}

func (g *gen) l2Substitution() {
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if g.setPieceRoom.contains(x, y) || !g.flip(4) {
				continue
			}
			c := l2BTypes[g.d(x, y)]
			if c == 0 {
				continue
			}
			rv := g.rnd(16)
			k := -1
			for rv >= 0 {
				if k++; k == len(l2BTypes) {
					k = 0
				}
				if c == l2BTypes[k] {
					rv--
				}
			}
			j := y - 2
			for ; j < y+2; j++ {
				for i := x - 2; i < x+2; i++ {
					if int(g.d(i, j)) == k {
						j = y + 3
						i = x + 2
					}
				}
			}
			if j < y+3 {
				g.sd(x, y, byte(k))
			}
		}
	}
}

func (g *gen) l2CountEmpty() (t int) {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.l2.pre[i][j] == ' ' {
				t++
			}
		}
	}
	return
}

func (g *gen) l2KnockWalls(x1, y1, x2, y2 int) {
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

// l2FillVoid grows a room into the void next to wall (xx, yy) in the free directions (DL2_FillVoids' body).
func (g *gen) l2FillVoid(xf1, yf1, xf2, yf2 bool, xx, yy int) {
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
			if g.p(x2, y1) != ' ' {
				yf1 = false
			}
			if g.p(x2, y2) != ' ' {
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
					if g.p(x2, jj) != ' ' {
						xf2 = false
					}
				}
				if xf2 {
					x2++
				}
			}
			x2 -= 2
			if x2-x1 > 5 {
				g.l2MapRoom(x1, y1, x2, y2)
				g.l2KnockWalls(x1, y1, x2, y2)
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
			if g.p(x1, y1) != ' ' {
				yf1 = false
			}
			if g.p(x1, y2) != ' ' {
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
					if g.p(x1, jj) != ' ' {
						xf1 = false
					}
				}
				if xf1 {
					x1--
				}
			}
			x1 += 2
			if x2-x1 > 5 {
				g.l2MapRoom(x1, y1, x2, y2)
				g.l2KnockWalls(x1, y1, x2, y2)
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
			if g.p(x1, y2) != ' ' {
				xf1 = false
			}
			if g.p(x2, y2) != ' ' {
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
					if g.p(ii, y2) != ' ' {
						yf2 = false
					}
				}
				if yf2 {
					y2++
				}
			}
			y2 -= 2
			if y2-y1 > 5 {
				g.l2MapRoom(x1, y1, x2, y2)
				g.l2KnockWalls(x1, y1, x2, y2)
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
			if g.p(x1, y1) != ' ' {
				xf1 = false
			}
			if g.p(x2, y1) != ' ' {
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
					if g.p(ii, y1) != ' ' {
						yf1 = false
					}
				}
				if yf1 {
					y1--
				}
			}
			y1 += 2
			if y2-y1 > 5 {
				g.l2MapRoom(x1, y1, x2, y2)
				g.l2KnockWalls(x1, y1, x2, y2)
			}
		}
	}
}

func (g *gen) l2FillVoids() bool {
	for to := 0; g.l2CountEmpty() > 700 && to < 100; {
		xx, yy := g.rnd(38)+1, g.rnd(38)+1
		if g.p(xx, yy) != '#' {
			continue
		}
		var xf1, xf2, yf1, yf2 bool
		switch {
		case g.p(xx-1, yy) == ' ' && g.p(xx+1, yy) == '.':
			if g.p(xx+1, yy-1) == '.' && g.p(xx+1, yy+1) == '.' && g.p(xx-1, yy-1) == ' ' && g.p(xx-1, yy+1) == ' ' {
				xf1, yf1, yf2 = true, true, true
			}
		case g.p(xx+1, yy) == ' ' && g.p(xx-1, yy) == '.':
			if g.p(xx-1, yy-1) == '.' && g.p(xx-1, yy+1) == '.' && g.p(xx+1, yy-1) == ' ' && g.p(xx+1, yy+1) == ' ' {
				xf2, yf1, yf2 = true, true, true
			}
		case g.p(xx, yy-1) == ' ' && g.p(xx, yy+1) == '.':
			if g.p(xx-1, yy+1) == '.' && g.p(xx+1, yy+1) == '.' && g.p(xx-1, yy-1) == ' ' && g.p(xx+1, yy-1) == ' ' {
				yf1, xf1, xf2 = true, true, true
			}
		case g.p(xx, yy+1) == ' ' && g.p(xx, yy-1) == '.':
			if g.p(xx-1, yy-1) == '.' && g.p(xx+1, yy-1) == '.' && g.p(xx-1, yy+1) == ' ' && g.p(xx+1, yy+1) == ' ' {
				yf2, xf1, xf2 = true, true, true
			}
		}
		if xf1 || yf1 || xf2 || yf2 {
			g.l2FillVoid(xf1, yf1, xf2, yf2, xx, yy)
		}
		to++
	}
	return g.l2CountEmpty() <= 700
}

func (g *gen) l2CreateDungeon() bool {
	forceHW, fw, fh := false, 0, 0
	switch {
	case g.currlevel == 5 && g.quests[QBlood].Active != QuestNotAvail:
		forceHW, fw, fh = true, 14, 20
	case g.currlevel == 6 && g.quests[QSchamb].Active != QuestNotAvail:
		forceHW, fw, fh = true, 10, 10
	case g.currlevel == 7 && g.quests[QBlind].Active != QuestNotAvail:
		forceHW, fw, fh = true, 15, 15
	}
	g.l2CreateRoom(2, 2, 39, 39, 0, 0, forceHW, fh, fw)
	for len(g.l2.halls) > 0 {
		h := g.l2.halls[0]
		g.l2.halls = g.l2.halls[1:]
		g.l2ConnectHall(h)
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			switch g.p(i, j) {
			case 'A', 'B', 'C', 'E':
				g.setP(i, j, '#')
			}
			if g.p(i, j) == ',' {
				g.setP(i, j, '.')
				for a := -1; a <= 1; a++ {
					for b := -1; b <= 1; b++ {
						if (a != 0 || b != 0) && g.p(i+a, j+b) == ' ' {
							g.setP(i+a, j+b, '#')
						}
					}
				}
			}
		}
	}
	if !g.l2FillVoids() {
		return false
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			g.l2DoPatternCheck(i, j)
		}
	}
	return true
}

func (g *gen) l2FixTransparency() {
	yy := 16
	for j := 0; j < 40; j++ {
		xx := 16
		for i := 0; i < 40; i++ {
			t := g.d(i, j)
			if t == 14 && g.d(i, j-1) == 10 {
				g.copyTrans(xx, yy, xx+1, yy)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == 15 && g.d(i+1, j) == 11 {
				g.copyTrans(xx, yy, xx, yy+1)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == 10 {
				g.copyTrans(xx, yy, xx+1, yy)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == 11 {
				g.copyTrans(xx, yy, xx, yy+1)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == 16 {
				g.copyTrans(xx, yy, xx+1, yy)
				g.copyTrans(xx, yy, xx, yy+1)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			xx += 2
		}
		yy += 2
	}
}

func (g *gen) l2FixDirtTiles() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) == 13 && g.d(i+1, j) != 11 {
				g.sd(i, j, 146)
			}
			if g.d(i, j) == 11 && g.d(i+1, j) != 11 {
				g.sd(i, j, 144)
			}
			if g.d(i, j) == 15 && g.d(i+1, j) != 11 {
				g.sd(i, j, 148)
			}
			if g.d(i, j) == 10 && g.d(i, j+1) != 10 {
				g.sd(i, j, 143)
			}
			if g.d(i, j) == 13 && g.d(i, j+1) != 10 {
				g.sd(i, j, 146)
			}
			if g.d(i, j) == 14 && g.d(i, j+1) != 15 {
				g.sd(i, j, 147)
			}
		}
	}
}

func (g *gen) l2FixLockout() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) == 4 && g.d(i-1, j) != 3 {
				g.sd(i, j, 1)
			}
			if g.d(i, j) == 5 && g.d(i, j-1) != 3 {
				g.sd(i, j, 2)
			}
		}
	}
	for j := 1; j < 39; j++ {
		for i := 1; i < 39; i++ {
			if g.prot(i, j) {
				continue
			}
			if (g.d(i, j) == 2 || g.d(i, j) == 5) && g.d(i, j-1) == 3 && g.d(i, j+1) == 3 {
				doorok := false
				for (g.d(i, j) == 2 || g.d(i, j) == 5) && g.d(i, j-1) == 3 && g.d(i, j+1) == 3 {
					if g.d(i, j) == 5 {
						doorok = true
					}
					i++
				}
				if !doorok && !g.prot(i-1, j) {
					g.sd(i-1, j, 5)
				}
			}
		}
	}
	for j := 1; j < 39; j++ {
		for i := 1; i < 39; i++ {
			if g.prot(j, i) {
				continue
			}
			if (g.d(j, i) == 1 || g.d(j, i) == 4) && g.d(j-1, i) == 3 && g.d(j+1, i) == 3 {
				doorok := false
				for (g.d(j, i) == 1 || g.d(j, i) == 4) && g.d(j-1, i) == 3 && g.d(j+1, i) == 3 {
					if g.d(j, i) == 4 {
						doorok = true
					}
					i++
				}
				if !doorok && !g.prot(j, i-1) {
					g.sd(j, i-1, 4)
				}
			}
		}
	}
}

func (g *gen) l2FixDoors() {
	for j := 1; j < 40; j++ {
		for i := 1; i < 40; i++ {
			if g.d(i, j) == 4 && g.d(i, j-1) == 3 {
				g.sd(i, j, 7)
			}
			if g.d(i, j) == 5 && g.d(i-1, j) == 3 {
				g.sd(i, j, 9)
			}
		}
	}
}

func (g *gen) l2PlaceStairs(entry Entry) bool {
	x, y, ok := g.placeMiniSet(&l2USTAIRS, 199, false)
	if !ok {
		return false
	}
	if entry == EntryMain {
		g.view = point{2*x + 21, 2*y + 20}
	}
	if x, y, ok = g.placeMiniSet(&l2DSTAIRS, 199, false); !ok {
		return false
	}
	if entry == EntryPrev {
		g.view = point{2*x + 20, 2*y + 22}
	}
	if g.currlevel == 5 {
		if x, y, ok = g.placeMiniSet(&l2WARPSTAIRS, 199, false); !ok {
			return false
		}
		if entry == EntryTWarpDn {
			g.view = point{2*x + 21, 2*y + 20}
		}
	}
	return true
}

func (g *gen) l2GenerateLevel(entry Entry) {
	for {
		g.l2.nRoomCnt = 0
		g.l2.halls = nil
		g.protected = [1600]bool{}
		for i := range g.l2.pre {
			for j := range g.l2.pre[i] {
				g.l2.pre[i][j] = ' '
			}
		}
		g.initTrans()
		if !g.l2CreateDungeon() {
			continue
		}
		g.l2FixTilesPatterns()
		g.l2InitSetPiece()
		g.floodTrans(3)
		g.l2FixTransparency()
		if g.l2PlaceStairs(entry) {
			break
		}
	}
	g.l2FixLockout()
	g.l2FixDoors()
	g.l2FixDirtTiles()
	g.placeThemeRooms(6, 10, 3, 0, false)
	for _, m := range []*miniset{&l2CTRDOOR1, &l2CTRDOOR2, &l2CTRDOOR3, &l2CTRDOOR4, &l2CTRDOOR5, &l2CTRDOOR6, &l2CTRDOOR7, &l2CTRDOOR8,
		&l2VARCH33, &l2VARCH34, &l2VARCH35, &l2VARCH36, &l2VARCH37, &l2VARCH38, &l2VARCH39, &l2VARCH40,
		&l2VARCH1, &l2VARCH2, &l2VARCH3, &l2VARCH4, &l2VARCH5, &l2VARCH6, &l2VARCH7, &l2VARCH8, &l2VARCH9, &l2VARCH10,
		&l2VARCH11, &l2VARCH12, &l2VARCH13, &l2VARCH14, &l2VARCH15, &l2VARCH16, &l2VARCH17, &l2VARCH18, &l2VARCH19, &l2VARCH20,
		&l2VARCH21, &l2VARCH22, &l2VARCH23, &l2VARCH24, &l2VARCH25, &l2VARCH26, &l2VARCH27, &l2VARCH28, &l2VARCH29, &l2VARCH30,
		&l2VARCH31, &l2VARCH32,
		&l2HARCH1, &l2HARCH2, &l2HARCH3, &l2HARCH4, &l2HARCH5, &l2HARCH6, &l2HARCH7, &l2HARCH8, &l2HARCH9, &l2HARCH10,
		&l2HARCH11, &l2HARCH12, &l2HARCH13, &l2HARCH14, &l2HARCH15, &l2HARCH16, &l2HARCH17, &l2HARCH18, &l2HARCH19, &l2HARCH20,
		&l2HARCH21, &l2HARCH22, &l2HARCH23, &l2HARCH24, &l2HARCH25, &l2HARCH26, &l2HARCH27, &l2HARCH28, &l2HARCH29, &l2HARCH30,
		&l2HARCH31, &l2HARCH32, &l2HARCH33, &l2HARCH34, &l2HARCH35, &l2HARCH36, &l2HARCH37, &l2HARCH38, &l2HARCH39, &l2HARCH40} {
		g.l2PlaceMiniSetRandom(m, 100)
	}
	g.l2PlaceMiniSetRandom(&l2CRUSHCOL, 99)
	g.l2PlaceMiniSetRandom1x1(1, 80, 10)
	g.l2PlaceMiniSetRandom1x1(1, 81, 10)
	g.l2PlaceMiniSetRandom1x1(1, 82, 10)
	g.l2PlaceMiniSetRandom1x1(2, 84, 10)
	g.l2PlaceMiniSetRandom1x1(2, 85, 10)
	g.l2PlaceMiniSetRandom1x1(2, 86, 10)
	g.l2PlaceMiniSetRandom1x1(8, 87, 50)
	for _, s := range []struct {
		m   *miniset
		per int
	}{{&l2PANCREAS1, 1}, {&l2PANCREAS2, 1}, {&l2BIG1, 3}, {&l2BIG2, 3}, {&l2BIG3, 3}, {&l2BIG4, 3}, {&l2BIG5, 3},
		{&l2BIG6, 20}, {&l2BIG7, 20}, {&l2BIG8, 3}, {&l2BIG9, 20}, {&l2BIG10, 20}} {
		g.l2PlaceMiniSetRandom(s.m, s.per)
	}
	g.l2Substitution()
	g.l2ApplyShadows()
	g.savePdungeon()
	g.checkQuests(g.setPieceRoom.x, g.setPieceRoom.y)
}

func (g *gen) createL2(seed int32, entry Entry) {
	g.rng.SetSeed(seed)
	g.l2GenerateLevel(entry)
	g.lPass3(12 - 1)
}

func iabs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
