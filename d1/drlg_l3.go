package d1

// drlg_l3.cpp: the Caves.

var l3ConvTbl = [16]byte{8, 11, 3, 10, 1, 9, 12, 12, 6, 13, 4, 13, 2, 14, 5, 7}

func (g *gen) l3FillRoom(x1, y1, x2, y2 int) bool {
	if x1 <= 1 || x2 >= 34 || y1 <= 1 || y2 >= 38 {
		return false
	}
	v := 0
	for j := y1; j <= y2; j++ {
		for i := x1; i <= x2; i++ {
			v += int(g.d(i, j))
		}
	}
	if v != 0 {
		return false
	}
	for j := y1 + 1; j < y2; j++ {
		for i := x1 + 1; i < x2; i++ {
			g.sd(i, j, 1)
		}
	}
	for j := y1; j <= y2; j++ {
		if !g.flip(2) {
			g.sd(x1, j, 1)
		}
		if !g.flip(2) {
			g.sd(x2, j, 1)
		}
	}
	for i := x1; i <= x2; i++ {
		if !g.flip(2) {
			g.sd(i, y1, 1)
		}
		if !g.flip(2) {
			g.sd(i, y2, 1)
		}
	}
	return true
}

func (g *gen) l3CreateBlock(x, y, obs, dir int) {
	var x1, y1, x2, y2 int
	bx := g.rnd(2) + 3
	by := g.rnd(2) + 3
	offset := func(size, at int) int {
		switch {
		case size < obs:
			return g.rnd(size) + at
		case size == obs:
			return at
		}
		return at - g.rnd(size)
	}
	switch dir {
	case 0:
		y2 = y - 1
		y1 = y2 - by
		x1 = offset(bx, x)
		x2 = bx + x1
	case 1:
		x1 = x + 1
		x2 = x1 + bx
		y1 = offset(by, y)
		y2 = y1 + by
	case 2:
		y1 = y + 1
		y2 = y1 + by
		x1 = offset(bx, x)
		x2 = bx + x1
	case 3:
		x2 = x - 1
		x1 = x2 - bx
		y1 = offset(by, y)
		y2 = y1 + by
	}
	if !g.l3FillRoom(x1, y1, x2, y2) || g.flip(4) {
		return
	}
	if dir != 2 {
		g.l3CreateBlock(x1, y1, by, 0)
	}
	if dir != 3 {
		g.l3CreateBlock(x2, y1, bx, 1)
	}
	if dir != 0 {
		g.l3CreateBlock(x1, y2, by, 2)
	}
	if dir != 1 {
		g.l3CreateBlock(x1, y1, bx, 3)
	}
}

func (g *gen) l3FloorArea(x1, y1, x2, y2 int) {
	for j := y1; j <= y2; j++ {
		for i := x1; i <= x2; i++ {
			g.sd(i, j, 1)
		}
	}
}

func (g *gen) l3FillDiagonals() {
	for j := 0; j < 39; j++ {
		for i := 0; i < 39; i++ {
			v := int(g.d(i+1, j+1)) + 2*int(g.d(i, j+1)) + 4*int(g.d(i+1, j)) + 8*int(g.d(i, j))
			if v == 6 {
				if g.flip(2) {
					g.sd(i, j, 1)
				} else {
					g.sd(i+1, j+1, 1)
				}
			}
			if v == 9 {
				if g.flip(2) {
					g.sd(i+1, j, 1)
				} else {
					g.sd(i, j+1, 1)
				}
			}
		}
	}
}

func (g *gen) l3FillSingles() {
	for j := 1; j < 39; j++ {
		for i := 1; i < 39; i++ {
			if g.d(i, j) == 0 && g.d(i, j-1)+g.d(i-1, j-1)+g.d(i+1, j-1) == 3 && g.d(i+1, j)+g.d(i-1, j) == 2 &&
				g.d(i, j+1)+g.d(i-1, j+1)+g.d(i+1, j+1) == 3 {
				g.sd(i, j, 1)
			}
		}
	}
}

func (g *gen) l3FillStraights() {
	var xc, yc int
	for j := 0; j < 39; j++ {
		xs := 0
		for i := 0; i < 37; i++ {
			if g.d(i, j) == 0 && g.d(i, j+1) == 1 {
				if xs == 0 {
					xc = i
				}
				xs++
			} else {
				if xs > 3 && !g.flip(2) {
					for k := xc; k < i; k++ {
						g.sd(k, j, byte(g.rnd(2)))
					}
				}
				xs = 0
			}
		}
	}
	for j := 0; j < 39; j++ {
		xs := 0
		for i := 0; i < 37; i++ {
			if g.d(i, j) == 1 && g.d(i, j+1) == 0 {
				if xs == 0 {
					xc = i
				}
				xs++
			} else {
				if xs > 3 && !g.flip(2) {
					for k := xc; k < i; k++ {
						g.sd(k, j+1, byte(g.rnd(2)))
					}
				}
				xs = 0
			}
		}
	}
	for i := 0; i < 39; i++ {
		ys := 0
		for j := 0; j < 37; j++ {
			if g.d(i, j) == 0 && g.d(i+1, j) == 1 {
				if ys == 0 {
					yc = j
				}
				ys++
			} else {
				if ys > 3 && !g.flip(2) {
					for k := yc; k < j; k++ {
						g.sd(i, k, byte(g.rnd(2)))
					}
				}
				ys = 0
			}
		}
	}
	for i := 0; i < 39; i++ {
		ys := 0
		for j := 0; j < 37; j++ {
			if g.d(i, j) == 1 && g.d(i+1, j) == 0 {
				if ys == 0 {
					yc = j
				}
				ys++
			} else {
				if ys > 3 && !g.flip(2) {
					for k := yc; k < j; k++ {
						g.sd(i+1, k, byte(g.rnd(2)))
					}
				}
				ys = 0
			}
		}
	}
}

func (g *gen) l3Edges() {
	for j := 0; j < 40; j++ {
		g.sd(39, j, 0)
	}
	for i := 0; i < 40; i++ {
		g.sd(i, 39, 0)
	}
}

func (g *gen) l3FloorAreaCount() (n int) {
	for i := 0; i < 1600; i++ {
		n += int(g.mem[i])
	}
	return
}

func (g *gen) l3MakeMegas() {
	for j := 0; j < 39; j++ {
		for i := 0; i < 39; i++ {
			v := int(g.d(i+1, j+1)) + 2*int(g.d(i, j+1)) + 4*int(g.d(i+1, j)) + 8*int(g.d(i, j))
			if v == 6 {
				v = g.pick(12, 5)
			}
			if v == 9 {
				v = g.pick(13, 14)
			}
			g.sd(i, j, l3ConvTbl[v])
		}
		g.sd(39, j, 8)
	}
	for i := 0; i < 40; i++ {
		g.sd(i, 39, 8)
	}
}

func (g *gen) l3River() {
	var river [301]int // D1's river[3][100], flat so the write one past a row lands where it does in D1
	rv := func(r, i int) *int { return &river[r*100+i] }
	var dir, nodir, riveramt int
	rivercnt, tries, pdir := 0, 0, -1
	for tries < 200 && rivercnt < 4 {
		bail := false
		for !bail && tries < 200 {
			tries++
			rx, ry, i := 0, 0, 0
			for (ry >= 40 || g.d(rx, ry) < 25 || g.d(rx, ry) > 28) && i < 100 {
				rx, ry = g.rnd(40), g.rnd(40)
				i++
				for ry < 40 && (g.d(rx, ry) < 25 || g.d(rx, ry) > 28) {
					if rx++; rx >= 40 {
						rx = 0
						ry++
					}
				}
			}
			if ry >= 40 {
				continue
			}
			if i >= 100 {
				return
			}
			switch g.d(rx, ry) {
			case 25:
				dir, nodir, *rv(2, 0) = 3, 2, 40
			case 26:
				dir, nodir, *rv(2, 0) = 0, 1, 38
			case 27:
				dir, nodir, *rv(2, 0) = 1, 0, 41
			case 28:
				dir, nodir, *rv(2, 0) = 2, 3, 39
			}
			*rv(0, 0), *rv(1, 0) = rx, ry
			riveramt = 1
			nodir2, dircheck := 4, 0
			for dircheck < 4 && riveramt < 100 {
				px, py := rx, ry
				if dircheck == 0 {
					dir = g.rnd(4)
				} else {
					dir = (dir + 1) & 3
				}
				dircheck++
				for dir == nodir || dir == nodir2 {
					dir = (dir + 1) & 3
					dircheck++
				}
				if dir == 0 && ry > 0 {
					ry--
				}
				if dir == 1 && ry < 40 {
					ry++
				}
				if dir == 2 && rx < 40 {
					rx++
				}
				if dir == 3 && rx > 0 {
					rx--
				}
				if g.d(rx, ry) != 7 {
					rx, ry = px, py
					continue
				}
				dircheck = 0
				if dir < 2 {
					*rv(2, riveramt) = g.pick(17, 18)
				}
				if dir > 1 {
					*rv(2, riveramt) = g.pick(15, 16)
				}
				*rv(0, riveramt), *rv(1, riveramt) = rx, ry
				riveramt++
				if dir == 0 && pdir == 2 || dir == 3 && pdir == 1 {
					if riveramt > 2 {
						*rv(2, riveramt-2) = 22
					}
					if dir == 0 {
						nodir2 = 1
					} else {
						nodir2 = 2
					}
				}
				if dir == 0 && pdir == 3 || dir == 2 && pdir == 1 {
					if riveramt > 2 {
						*rv(2, riveramt-2) = 21
					}
					if dir == 0 {
						nodir2 = 1
					} else {
						nodir2 = 3
					}
				}
				if dir == 1 && pdir == 2 || dir == 3 && pdir == 0 {
					if riveramt > 2 {
						*rv(2, riveramt-2) = 20
					}
					if dir == 1 {
						nodir2 = 0
					} else {
						nodir2 = 2
					}
				}
				if dir == 1 && pdir == 3 || dir == 2 && pdir == 0 {
					if riveramt > 2 {
						*rv(2, riveramt-2) = 19
					}
					if dir == 1 {
						nodir2 = 0
					} else {
						nodir2 = 3
					}
				}
				pdir = dir
			}
			if dir == 0 && ry >= 2 && g.d(rx, ry-1) == 10 && g.d(rx, ry-2) == 8 {
				*rv(0, riveramt), *rv(1, riveramt), *rv(2, riveramt) = rx, ry-1, 24
				if pdir == 2 {
					*rv(2, riveramt-1) = 22
				}
				if pdir == 3 {
					*rv(2, riveramt-1) = 21
				}
				bail = true
			}
			if dir == 1 && ry+2 < 40 && g.d(rx, ry+1) == 2 && g.d(rx, ry+2) == 8 {
				*rv(0, riveramt), *rv(1, riveramt), *rv(2, riveramt) = rx, ry+1, 42
				if pdir == 2 {
					*rv(2, riveramt-1) = 20
				}
				if pdir == 3 {
					*rv(2, riveramt-1) = 19
				}
				bail = true
			}
			if dir == 2 && rx+2 < 40 && g.d(rx+1, ry) == 4 && g.d(rx+2, ry) == 8 {
				*rv(0, riveramt), *rv(1, riveramt), *rv(2, riveramt) = rx+1, ry, 43
				if pdir == 0 {
					*rv(2, riveramt-1) = 19
				}
				if pdir == 1 {
					*rv(2, riveramt-1) = 21
				}
				bail = true
			}
			if dir == 3 && rx >= 2 && g.d(rx-1, ry) == 9 && g.d(rx-2, ry) == 8 {
				*rv(0, riveramt), *rv(1, riveramt), *rv(2, riveramt) = rx-1, ry, 23
				if pdir == 0 {
					*rv(2, riveramt-1) = 20
				}
				if pdir == 1 {
					*rv(2, riveramt-1) = 22
				}
				bail = true
			}
		}
		if bail && riveramt < 7 {
			bail = false
		}
		if !bail {
			continue
		}
		found, lpcnt, bridge := 0, 0, 0
		for found == 0 && lpcnt < 30 {
			lpcnt++
			bridge = g.rnd(riveramt)
			bx, by, bt := *rv(0, bridge), *rv(1, bridge), *rv(2, bridge)
			if (bt == 15 || bt == 16) && g.d(bx, by-1) == 7 && g.d(bx, by+1) == 7 {
				found = 1
			}
			if (bt == 17 || bt == 18) && g.d(bx-1, by) == 7 && g.d(bx+1, by) == 7 {
				found = 2
			}
			for i := 0; i < riveramt && found != 0; i++ {
				if found == 1 && (by-1 == *rv(1, i) || by+1 == *rv(1, i)) && bx == *rv(0, i) {
					found = 0
				}
				if found == 2 && (bx-1 == *rv(0, i) || bx+1 == *rv(0, i)) && by == *rv(1, i) {
					found = 0
				}
			}
		}
		if found != 0 {
			if found == 1 {
				*rv(2, bridge) = 44
			} else {
				*rv(2, bridge) = 45
			}
			rivercnt++
			for bridge = 0; bridge <= riveramt; bridge++ {
				g.sd(*rv(0, bridge), *rv(1, bridge), byte(*rv(2, bridge)))
			}
		}
	}
}

var (
	l3SpawnEdgeTable = [15]byte{0x00, 0x0A, 0x43, 0x05, 0x2c, 0x06, 0x09, 0x00, 0x00, 0x1c, 0x83, 0x06, 0x09, 0x0A, 0x05}
	l3SpawnTable     = [15]byte{0x00, 0x0A, 0x03, 0x05, 0x0C, 0x06, 0x09, 0x00, 0x00, 0x0C, 0x03, 0x06, 0x09, 0x0A, 0x05}
)

func (g *gen) l3SpawnEdge(x, y int, totarea *int) bool {
	if *totarea > 40 || x < 0 || y < 0 || x >= 40 || y >= 40 {
		return true
	}
	if g.d(x, y)&0x80 != 0 {
		return false
	}
	if g.d(x, y) > 15 {
		return true
	}
	i := g.d(x, y)
	g.sd(x, y, i|0x80)
	*totarea++
	t := l3SpawnEdgeTable[i]
	return t&8 != 0 && g.l3SpawnEdge(x, y-1, totarea) || t&4 != 0 && g.l3SpawnEdge(x, y+1, totarea) ||
		t&2 != 0 && g.l3SpawnEdge(x+1, y, totarea) || t&1 != 0 && g.l3SpawnEdge(x-1, y, totarea) ||
		t&0x80 != 0 && g.l3Spawn(x, y-1, totarea) || t&0x40 != 0 && g.l3Spawn(x, y+1, totarea) ||
		t&0x20 != 0 && g.l3Spawn(x+1, y, totarea) || t&0x10 != 0 && g.l3Spawn(x-1, y, totarea)
}

func (g *gen) l3Spawn(x, y int, totarea *int) bool {
	if *totarea > 40 || x < 0 || y < 0 || x >= 40 || y >= 40 {
		return true
	}
	if g.d(x, y)&0x80 != 0 {
		return false
	}
	if g.d(x, y) > 15 {
		return true
	}
	i := g.d(x, y)
	g.sd(x, y, i|0x80)
	*totarea++
	if i != 8 {
		t := l3SpawnTable[i]
		return t&8 != 0 && g.l3SpawnEdge(x, y-1, totarea) || t&4 != 0 && g.l3SpawnEdge(x, y+1, totarea) ||
			t&2 != 0 && g.l3SpawnEdge(x+1, y, totarea) || t&1 != 0 && g.l3SpawnEdge(x-1, y, totarea)
	}
	return g.l3Spawn(x+1, y, totarea) || g.l3Spawn(x-1, y, totarea) || g.l3Spawn(x, y+1, totarea) || g.l3Spawn(x, y-1, totarea)
}

func (g *gen) l3CanReplaceTile(replace byte, x, y int) bool {
	if replace < 84 || replace > 100 {
		return true
	}
	nw := point{x - 1, y}
	in := func(p point) bool { return p.x >= 0 && p.x < 40 && p.y >= 0 && p.y < 40 }
	cmp := func(p1 point) bool { return in(p1) && in(nw) && g.d(p1.x, p1.y) >= 84 && g.d(nw.x, nw.y) <= 100 }
	return !(cmp(nw) || cmp(point{x + 1, y}) || cmp(point{x, y + 1}) || cmp(point{x, y - 1}))
}

func (g *gen) l3PlaceMiniSetRandom(m *miniset, rndper int) bool {
	placed := false
	for sy := 0; sy < 40-m.h; sy++ {
		for sx := 0; sx < 40-m.w; sx++ {
			if !m.matches(g, sx, sy, true) || !g.l3CanReplaceTile(m.replace[0][0], sx, sy) || g.rnd(100) >= rndper {
				continue
			}
			m.place(g, sx, sy, false)
			placed = true
		}
	}
	return placed
}

// l3PlaceDecoration scatters one single-tile decoration the DRLG_L3 way, which also honours l3CanReplaceTile.
func (g *gen) l3PlaceDecoration(d Decoration) {
	g.l3PlaceMiniSetRandom(&miniset{w: 1, h: 1, search: [6][6]byte{{d.Search}}, replace: [6][6]byte{{d.Replace}}}, d.Freq)
}

func (g *gen) l3PlaceLavaPool() bool {
	poolsub := [15]byte{0, 35, 26, 36, 25, 29, 34, 7, 33, 28, 27, 37, 32, 31, 30}
	placed := false
	for duny := 0; duny < 40; duny++ {
		for dunx := 0; dunx < 40; dunx++ {
			if g.d(dunx, duny) != 8 {
				continue
			}
			g.sd(dunx, duny, 8|0x80)
			totarea := 1
			found := true
			if dunx+1 < 40 {
				found = g.l3Spawn(dunx+1, duny, &totarea)
			}
			if dunx-1 > 0 && !found {
				found = g.l3Spawn(dunx-1, duny, &totarea)
			} else {
				found = true
			}
			if duny+1 < 40 && !found {
				found = g.l3Spawn(dunx, duny+1, &totarea)
			} else {
				found = true
			}
			if duny-1 > 0 && !found {
				found = g.l3Spawn(dunx, duny-1, &totarea)
			} else {
				found = true
			}
			placePool := g.rnd(100) < 25
			for j := max(duny-totarea, 0); j < min(duny+totarea, 40); j++ {
				for i := max(dunx-totarea, 0); i < min(dunx+totarea, 40); i++ {
					if g.d(i, j)&0x80 == 0 {
						continue
					}
					g.sd(i, j, g.d(i, j)&^0x80)
					if totarea > 4 && placePool && !found {
						if k := poolsub[g.d(i, j)]; k != 0 && k <= 37 {
							g.sd(i, j, k)
						}
						placed = true
					}
				}
			}
		}
	}
	return placed
}

func (g *gen) l3PoolFix() {
	lava := func(x, y int) bool { t := g.d(x, y); return t >= 25 && t <= 41 }
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if g.d(x, y) == 8 && lava(x-1, y-1) && lava(x-1, y) && lava(x-1, y+1) && lava(x, y-1) && lava(x, y+1) &&
				lava(x+1, y-1) && lava(x+1, y) && lava(x+1, y+1) {
				g.sd(x, y, 33)
			}
		}
	}
}

func isFenceTile(t byte) bool { return t >= 130 && t <= 152 }

func (g *gen) l3FenceVerticalUp(i, y int) bool {
	if isFenceTile(g.d(i+1, y)) || isFenceTile(g.d(i-1, y)) {
		return false
	}
	switch g.d(i, y) {
	case 7, 10, 126, 129, 134, 136:
		return true
	}
	return false
}

func (g *gen) l3FenceVerticalDown(i, y int) bool {
	if isFenceTile(g.d(i+1, y)) || isFenceTile(g.d(i-1, y)) {
		return false
	}
	switch g.d(i, y) {
	case 2, 7, 134, 136:
		return true
	}
	return false
}

func (g *gen) l3FenceHorizontalLeft(x, j int) bool {
	if isFenceTile(g.d(x, j+1)) || isFenceTile(g.d(x, j-1)) {
		return false
	}
	switch g.d(x, j) {
	case 7, 9, 121, 124, 135, 137:
		return true
	}
	return false
}

func (g *gen) l3FenceHorizontalRight(x, j int) bool {
	if isFenceTile(g.d(x, j+1)) || isFenceTile(g.d(x, j-1)) {
		return false
	}
	switch g.d(x, j) {
	case 4, 7, 135, 137:
		return true
	}
	return false
}

func (g *gen) l3AddFenceDoors() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) != 7 {
				continue
			}
			if isFenceTile(g.d(i-1, j)) && isFenceTile(g.d(i+1, j)) {
				g.sd(i, j, 146)
				continue
			}
			if isFenceTile(g.d(i, j-1)) && isFenceTile(g.d(i, j+1)) {
				g.sd(i, j, 147)
			}
		}
	}
}

func oneOf(t byte, set ...byte) bool {
	for _, s := range set {
		if t == s {
			return true
		}
	}
	return false
}

func (g *gen) l3FenceDoorFix() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) == 146 {
				if !isFenceTile(g.d(i+1, j)) || !isFenceTile(g.d(i-1, j)) {
					g.sd(i, j, 7)
					continue
				}
				if !oneOf(g.d(i+1, j), 130, 132, 133, 134, 136, 138, 140) && !oneOf(g.d(i-1, j), 130, 132, 133, 134, 136, 138, 140) {
					g.sd(i, j, 7)
					continue
				}
			}
			if g.d(i, j) == 147 {
				if !isFenceTile(g.d(i, j+1)) || !isFenceTile(g.d(i, j-1)) {
					g.sd(i, j, 7)
					continue
				}
				if !oneOf(g.d(i, j+1), 131, 132, 133, 135, 137, 138, 139) && !oneOf(g.d(i, j-1), 131, 132, 133, 135, 137, 138, 139) {
					g.sd(i, j, 7)
					continue
				}
			}
		}
	}
}

func (g *gen) l3Fence() {
	for j := 1; j < 39; j++ {
		for i := 1; i < 39; i++ {
			if g.d(i, j) == 10 && !g.flip(2) {
				x := i
				for g.d(x, j) == 10 {
					x++
				}
				x--
				if x-i > 0 {
					g.sd(i, j, 127)
					for xx := i + 1; xx < x; xx++ {
						g.sd(xx, j, byte(g.pick(129, 126)))
					}
					g.sd(x, j, 128)
				}
			}
			if g.d(i, j) == 9 && !g.flip(2) {
				y := j
				for g.d(i, y) == 9 {
					y++
				}
				y--
				if y-j > 0 {
					g.sd(i, j, 123)
					for yy := j + 1; yy < y; yy++ {
						g.sd(i, yy, byte(g.pick(124, 121)))
					}
					g.sd(i, y, 122)
				}
			}
			if g.d(i, j) == 11 && g.d(i+1, j) == 10 && g.d(i, j+1) == 9 && !g.flip(2) {
				g.sd(i, j, 125)
				x := i + 1
				for g.d(x, j) == 10 {
					x++
				}
				x--
				for xx := i + 1; xx < x; xx++ {
					g.sd(xx, j, byte(g.pick(129, 126)))
				}
				g.sd(x, j, 128)
				y := j + 1
				for g.d(i, y) == 9 {
					y++
				}
				y--
				for yy := j + 1; yy < y; yy++ {
					g.sd(i, yy, byte(g.pick(124, 121)))
				}
				g.sd(i, y, 122)
			}
		}
	}
	for j := 1; j < 40; j++ {
		for i := 1; i < 40; i++ {
			if g.d(i, j) != 7 {
				continue
			}
			g.discard(1)
			if g.nearThemeRoom(i, j) {
				continue
			}
			if g.flip(2) {
				y1 := j
				for y1 >= 0 && g.l3FenceVerticalUp(i, y1) {
					y1--
				}
				y1++
				y2 := j
				for y2 < 40 && g.l3FenceVerticalDown(i, y2) {
					y2++
				}
				y2--
				skip := g.d(i, y1) != 7 && g.d(i, y2) != 7
				if y2-y1 > 1 && skip {
					rp := g.rnd(y2-y1-1) + y1 + 1
					for y := y1; y <= y2; y++ {
						if y == rp {
							continue
						}
						switch g.d(i, y) {
						case 7:
							g.sd(i, y, byte(g.pick(137, 135)))
						case 10:
							g.sd(i, y, 131)
						case 126, 129:
							g.sd(i, y, 133)
						case 2:
							g.sd(i, y, 139)
						case 134, 136:
							g.sd(i, y, 138)
						}
					}
				}
			} else {
				x1 := i
				for x1 >= 0 && g.l3FenceHorizontalLeft(x1, j) {
					x1--
				}
				x1++
				x2 := i
				for x2 < 40 && g.l3FenceHorizontalRight(x2, j) {
					x2++
				}
				x2--
				skip := g.d(x1, j) != 7 && g.d(x2, j) != 7
				if x2-x1 > 1 && skip {
					rp := g.rnd(x2-x1-1) + x1 + 1
					for x := x1; x <= x2; x++ {
						if x == rp {
							continue
						}
						switch g.d(x, j) {
						case 7:
							g.sd(x, j, byte(g.pick(136, 134)))
						case 9:
							g.sd(x, j, 130)
						case 121, 124:
							g.sd(x, j, 132)
						case 4:
							g.sd(x, j, 140)
						case 135, 137:
							g.sd(x, j, 138)
						}
					}
				}
			}
		}
	}
	g.l3AddFenceDoors()
	g.l3FenceDoorFix()
}

func (g *gen) l3PlaceAnvil() bool {
	d := g.loadDun("l3/anvil")
	aw, ah := d.w()+2, d.h()+2
	sx, sy := g.rnd(40-aw), g.rnd(40-ah)
	for tries := 0; ; tries, sx = tries+1, sx+1 {
		if tries > 198 {
			return false
		}
		if sx == 40-aw {
			sx = 0
			if sy++; sy == 40-ah {
				sy = 0
			}
		}
		found := true
	area:
		for y := sy; y < sy+ah; y++ {
			for x := sx; x < sx+aw; x++ {
				if g.prot(x, y) || g.d(x, y) != 7 {
					found = false
					break area
				}
			}
		}
		if found {
			break
		}
	}
	g.placeDunTiles(d, sx+1, sy+1, 7)
	g.setPiece = rect{sx, sy, aw, ah}
	for y := sy; y < sy+ah; y++ {
		for x := sx; x < sx+aw; x++ {
			g.setProt(x, y)
		}
	}
	// keep rivers out of the island, undone after River
	g.sd(sx+7, sy+5, 2)
	g.sd(sx+8, sy+5, 2)
	g.sd(sx+9, sy+5, 2)
	return true
}

func (g *gen) l3Warp() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) == 125 && g.d(i+1, j) == 125 && g.d(i, j+1) == 125 && g.d(i+1, j+1) == 125 {
				g.sd(i, j, 156)
				g.sd(i+1, j, 155)
				g.sd(i, j+1, 153)
				g.sd(i+1, j+1, 154)
				return
			}
			if g.d(i, j) == 5 && g.d(i+1, j+1) == 7 {
				g.sd(i, j, 7)
			}
		}
	}
}

func (g *gen) l3HallOfHeroes() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) == 5 && g.d(i+1, j+1) == 7 {
				g.sd(i, j, 7)
			}
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) == 5 && g.d(i+1, j+1) == 12 && g.d(i+1, j) == 7 {
				g.sd(i, j, 7)
				g.sd(i, j+1, 7)
				g.sd(i+1, j+1, 7)
			}
			if g.d(i, j) == 5 && g.d(i+1, j+1) == 12 && g.d(i, j+1) == 7 {
				g.sd(i, j, 7)
				g.sd(i+1, j, 7)
				g.sd(i+1, j+1, 7)
			}
		}
	}
}

func (g *gen) l3LockRectangle(x, y int) {
	if !g.masked(x, y) {
		return
	}
	g.setMask(x, y, false)
	g.l3.lockoutcnt++
	g.l3LockRectangle(x, y-1)
	g.l3LockRectangle(x, y+1)
	g.l3LockRectangle(x-1, y)
	g.l3LockRectangle(x+1, y)
}

func (g *gen) l3Lockout() bool {
	g.mask = [1600]bool{}
	fx, fy, t := 0, 0, 0
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) != 0 {
				g.setMask(i, j, true)
				fx, fy = i, j
				t++
			}
		}
	}
	g.l3.lockoutcnt = 0
	g.l3LockRectangle(fx, fy)
	return t == g.l3.lockoutcnt
}

func (g *gen) l3PlaceStairs(entry Entry) bool {
	x, y, ok := g.placeMiniSet(&l3L3UP, false)
	if !ok {
		return false
	}
	if entry == EntryMain {
		g.view = point{2*x + 17, 2*y + 19}
	}
	if x, y, ok = g.placeMiniSet(&l3L3DOWN, false); !ok {
		return false
	}
	if entry == EntryPrev {
		g.view = point{2*x + 19, 2*y + 17}
	}
	if g.currlevel == 9 {
		if x, y, ok = g.placeMiniSet(&l3L3HOLDWARP, false); !ok {
			return false
		}
		if entry == EntryTWarpDn {
			g.view = point{2*x + 17, 2*y + 19}
		}
	}
	return true
}

func (g *gen) l3GenerateLevel(entry Entry) {
	for {
		g.fillDungeon(0)
		g.protected = [1600]bool{}
		x1, y1 := g.rnd(20)+10, g.rnd(20)+10
		x2, y2 := x1+2, y1+2
		g.l3FillRoom(x1, y1, x2, y2)
		g.l3CreateBlock(x1, y1, 2, 0)
		g.l3CreateBlock(x2, y1, 2, 1)
		g.l3CreateBlock(x1, y2, 2, 2)
		g.l3CreateBlock(x1, y1, 2, 3)
		if g.available(QAnvil) {
			x1, y1 = g.rnd(10)+10, g.rnd(10)+10
			g.l3FloorArea(x1, y1, x1+12, y1+12)
		}
		g.l3FillDiagonals()
		g.l3FillSingles()
		g.l3FillStraights()
		g.l3FillDiagonals()
		g.l3Edges()
		if g.l3FloorAreaCount() < 600 || !g.l3Lockout() {
			continue
		}
		g.l3MakeMegas()
		if !g.l3PlaceStairs(entry) {
			continue
		}
		if g.available(QAnvil) && !g.l3PlaceAnvil() {
			continue
		}
		if g.l3PlaceLavaPool() {
			break
		}
	}
	g.l3PoolFix()
	g.l3Warp()
	for _, s := range []struct {
		m   *miniset
		per int
	}{{&l3L3ISLE1, 70}, {&l3L3ISLE2, 70}, {&l3L3ISLE3, 30}, {&l3L3ISLE4, 30}, {&l3L3ISLE1, 100}, {&l3L3ISLE2, 100}, {&l3L3ISLE5, 90}} {
		g.l3PlaceMiniSetRandom(s.m, s.per)
	}
	g.l3HallOfHeroes()
	g.l3River()
	if g.available(QAnvil) {
		sp := g.setPiece
		g.sd(sp.x+7, sp.y+5, 7)
		g.sd(sp.x+8, sp.y+5, 7)
		g.sd(sp.x+9, sp.y+5, 7)
		if t := g.d(sp.x+10, sp.y+5); t == 17 || t == 18 {
			g.sd(sp.x+10, sp.y+5, 45)
		}
	}
	g.placeThemeRooms(5, 10, 7, g.params.ThemeFreq, false)
	g.l3Fence()
	for _, s := range []struct {
		m   *miniset
		per int
	}{{&l3L3TITE1, 10}, {&l3L3TITE2, 10}, {&l3L3TITE3, 10}, {&l3L3TITE6, 20}, {&l3L3TITE7, 20}, {&l3L3TITE8, 20},
		{&l3L3TITE9, 20}, {&l3L3TITE10, 20}, {&l3L3TITE11, 30}, {&l3L3TITE12, 20}, {&l3L3TITE13, 20},
		{&l3L3CREV1, 30}, {&l3L3CREV2, 30}, {&l3L3CREV3, 30}, {&l3L3CREV4, 30}, {&l3L3CREV5, 30}, {&l3L3CREV6, 30},
		{&l3L3CREV7, 30}, {&l3L3CREV8, 30}, {&l3L3CREV9, 30}, {&l3L3CREV10, 30}, {&l3L3CREV11, 30}} {
		g.l3PlaceMiniSetRandom(s.m, s.per)
	}
	g.placeDecorations()
	g.savePdungeon()
}
