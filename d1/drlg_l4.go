package d1

// drlg_l4.cpp: Hell.

var l4ConvTbl = [16]byte{30, 6, 1, 6, 2, 6, 6, 6, 9, 6, 1, 6, 2, 6, 3, 6}

func (g *gen) l4ApplyShadows() {
	for y := 1; y < 40; y++ {
		for x := 1; x < 40; x++ {
			if !oneOf(g.d(x, y), 3, 4, 8, 15) {
				continue
			}
			if g.d(x-1, y) == 6 {
				g.sd(x-1, y, 47)
			}
			if g.d(x-1, y-1) == 6 {
				g.sd(x-1, y-1, 48)
			}
		}
	}
}

func (g *gen) l4InitSetPiece() {
	if !g.available(QWarlord) {
		return
	}
	d := g.loadDun("l4/warlord")
	g.placeDunTiles(d, g.setPieceRoom.x, g.setPieceRoom.y, 6)
	g.setPiece = rect{g.setPieceRoom.x, g.setPieceRoom.y, d.w(), d.h()}
}

func (g *gen) l4MapRoom(r rect) {
	for y := 0; y < r.h && y+r.y < 20; y++ {
		for x := 0; x < r.w && x+r.x < 20; x++ {
			g.setMask(r.x+x, r.y+y, true)
		}
	}
}

func (g *gen) l4CheckRoom(r rect) bool {
	if r.x <= 0 || r.y <= 0 {
		return false
	}
	for y := 0; y < r.h; y++ {
		for x := 0; x < r.w; x++ {
			if x+r.x < 0 || x+r.x >= 20 || y+r.y < 0 || y+r.y >= 20 || g.masked(r.x+x, r.y+y) {
				return false
			}
		}
	}
	return true
}

func (g *gen) l4GenerateRoom(area rect, vertical bool) {
	rotate := !g.flip(4)
	vertical = !vertical && rotate || vertical && !rotate
	var room1 rect
	place1 := false
	for num := 0; num < 20; num++ {
		room1.w = (g.rnd(5) + 2) &^ 1
		room1.h = (g.rnd(5) + 2) &^ 1
		if vertical {
			room1.x, room1.y = area.x-room1.w, area.y+area.h/2-room1.h/2
			place1 = g.l4CheckRoom(rect{room1.x - 1, room1.y - 1, room1.h + 2, room1.w + 1}) // D1 bug: width and height swapped
		} else {
			room1.x, room1.y = area.x+area.w/2-room1.w/2, area.y-room1.h
			place1 = g.l4CheckRoom(rect{room1.x - 1, room1.y - 1, room1.w + 2, room1.h + 1})
		}
		if place1 {
			break
		}
	}
	if place1 {
		g.l4MapRoom(rect{room1.x, room1.y, min(40-room1.x, room1.w), min(40-room1.y, room1.h)})
	}
	room2 := room1
	var place2 bool
	if vertical {
		room2.x = area.x + area.w
		place2 = g.l4CheckRoom(rect{room2.x, room2.y - 1, room2.w + 1, room2.h + 2})
	} else {
		room2.y = area.y + area.h
		place2 = g.l4CheckRoom(rect{room2.x - 1, room2.y, room2.w + 2, room2.h + 1})
	}
	if place2 {
		g.l4MapRoom(room2)
	}
	if place1 {
		g.l4GenerateRoom(room1, vertical)
	}
	if place2 {
		g.l4GenerateRoom(room2, vertical)
	}
}

func (g *gen) l4FirstRoom() {
	room := rect{0, 0, 14, 14}
	if g.currlevel != 16 {
		if g.currlevel == g.quests[QWarlord].Level && g.quests[QWarlord].Active != QuestNotAvail {
			room.w, room.h = 11, 11
		} else {
			room.w = g.rnd(5) + 2
			room.h = g.rnd(5) + 2
		}
	}
	xmin, xmax := (20-room.w)/2, 19-room.w
	ymin, ymax := (20-room.h)/2, 19-room.h
	room.x = g.rnd(xmax-xmin+1) + xmin
	room.y = g.rnd(ymax-ymin+1) + ymin
	if g.currlevel == 16 {
		g.l4.hold = point{room.x, room.y}
	}
	if g.available(QWarlord) {
		g.setPieceRoom = rect{room.x + 1, room.y + 1, room.w + 1, room.h + 1}
	} else {
		g.setPieceRoom = rect{}
	}
	g.l4MapRoom(room)
	g.l4GenerateRoom(room, !g.flip(2))
}

func (g *gen) l4MirrorLayout() {
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			if g.masked(x, y) {
				g.setMask(x, 39-y, true)
				g.setMask(39-x, y, true)
				g.setMask(39-x, 39-y, true)
			}
		}
	}
}

func (g *gen) l4MakeDmt() {
	b := func(x, y int) int {
		if g.masked(x, y) {
			return 1
		}
		return 0
	}
	for y := 0; y < 39; y++ {
		for x := 0; x < 39; x++ {
			g.sd(x, y, l4ConvTbl[b(x+1, y+1)<<3|b(x, y+1)<<2|b(x+1, y)<<1|b(x, y)])
		}
	}
}

func (g *gen) l4HorizontalWallOk(i, j int) int {
	x := 1
	for ; g.d(i+x, j) == 6; x++ {
		if g.prot(i+x, j) || g.d(i+x, j-1) != 6 || g.d(i+x, j+1) != 6 {
			break
		}
	}
	if oneOf(g.d(i+x, j), 10, 12, 13, 15, 16, 21, 22) && x > 3 {
		return x
	}
	return -1
}

func (g *gen) l4VerticalWallOk(i, j int) int {
	y := 1
	for ; g.d(i, j+y) == 6; y++ {
		if g.prot(i, j+y) || g.d(i-1, j+y) != 6 || g.d(i+1, j+y) != 6 {
			break
		}
	}
	if oneOf(g.d(i, j+y), 8, 9, 11, 14, 15, 16, 21, 23) && y > 3 {
		return y
	}
	return -1
}

func (g *gen) replaceIf(x, y int, from, to byte) {
	if g.d(x, y) == from {
		g.sd(x, y, to)
	}
}

func (g *gen) l4HorizontalWall(i, j, dx int) {
	g.replaceIf(i, j, 13, 17)
	g.replaceIf(i, j, 16, 11)
	g.replaceIf(i, j, 12, 14)
	for xx := 1; xx < dx; xx++ {
		g.sd(i+xx, j, 2)
	}
	g.replaceIf(i+dx, j, 15, 14)
	g.replaceIf(i+dx, j, 10, 17)
	g.replaceIf(i+dx, j, 21, 23)
	g.replaceIf(i+dx, j, 22, 29)
	xx := g.rnd(dx-3) + 1
	g.sd(i+xx, j, 57)
	g.sd(i+xx+2, j, 56)
	g.sd(i+xx+1, j, 60)
	g.replaceIf(i+xx, j-1, 6, 58)
	g.replaceIf(i+xx+1, j-1, 6, 59)
}

func (g *gen) l4VerticalWall(i, j, dy int) {
	g.replaceIf(i, j, 14, 17)
	g.replaceIf(i, j, 8, 9)
	g.replaceIf(i, j, 15, 10)
	for yy := 1; yy < dy; yy++ {
		g.sd(i, j+yy, 1)
	}
	g.replaceIf(i, j+dy, 11, 17)
	g.replaceIf(i, j+dy, 9, 10)
	g.replaceIf(i, j+dy, 16, 13)
	g.replaceIf(i, j+dy, 21, 22)
	g.replaceIf(i, j+dy, 23, 29)
	yy := g.rnd(dy-3) + 1
	g.sd(i, j+yy, 53)
	g.sd(i, j+yy+2, 52)
	g.sd(i, j+yy+1, 6)
	g.replaceIf(i-1, j+yy, 6, 54)
	g.replaceIf(i-1, j+yy-1, 6, 55)
}

func (g *gen) l4AddWall() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.prot(i, j) {
				continue
			}
			for _, d := range [7]byte{10, 12, 13, 15, 16, 21, 22} {
				if d == g.d(i, j) {
					g.discard(1)
					if x := g.l4HorizontalWallOk(i, j); x != -1 {
						g.l4HorizontalWall(i, j, x)
					}
				}
			}
			for _, d := range [8]byte{8, 9, 11, 14, 15, 16, 21, 23} {
				if d == g.d(i, j) {
					g.discard(1)
					if y := g.l4VerticalWallOk(i, j); y != -1 {
						g.l4VerticalWall(i, j, y)
					}
				}
			}
		}
	}
}

func (g *gen) l4Substitution() {
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if !g.flip(3) {
				continue
			}
			c := l4BTypes[g.d(x, y)]
			if c == 0 || g.prot(x, y) {
				continue
			}
			rv := g.rnd(16)
			i := -1
			for rv >= 0 {
				if i++; i == len(l4BTypes) {
					i = 0
				}
				if c == l4BTypes[i] {
					rv--
				}
			}
			g.sd(x, y, byte(i))
		}
	}
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if g.flip(10) && l4BTypes[g.d(x, y)] == 6 && !g.prot(x, y) {
				g.sd(x, y, byte([3]int{95, 96, 97}[g.rnd(3)]))
			}
		}
	}
}

func (g *gen) l4PrepareInnerBorders() {
	for y := 19; y >= 0; y-- {
		for x := 19; x >= 0; x-- {
			if !g.masked(x, y) {
				g.l4.hallok[y] = false
			} else {
				g.l4.hallok[y] = y+1 < 20 && g.masked(x, y+1) && !(x+1 < 20 && y+1 < 20 && g.masked(x+1, y+1))
				x = 0
			}
		}
	}
	ry := g.rnd(19) + 1
	for ry != 0 {
		if g.l4.hallok[ry] {
			for x := 19; x >= 0; x-- {
				if g.masked(x, ry) {
					x = -1
					ry = 0
				} else {
					g.setMask(x, ry, true)
					g.setMask(x, ry+1, true)
				}
			}
		} else if ry++; ry == 20 {
			ry = 1
		}
	}
	for x := 19; x >= 0; x-- {
		for y := 19; y >= 0; y-- {
			if !g.masked(x, y) {
				g.l4.hallok[x] = false
			} else {
				g.l4.hallok[x] = x+1 < 20 && g.masked(x+1, y) && !(x+1 < 20 && y+1 < 20 && g.masked(x+1, y+1))
				y = 0
			}
		}
	}
	rx := g.rnd(19) + 1
	for rx != 0 {
		if g.l4.hallok[rx] {
			for y := 19; y >= 0; y-- {
				if g.masked(rx, y) {
					y = -1
					rx = 0
				} else {
					g.setMask(rx, y, true)
					g.setMask(rx+1, y, true)
				}
			}
		} else if rx++; rx == 20 {
			rx = 1
		}
	}
}

func (g *gen) l4ProtectQuads() {
	h := g.l4.hold
	for y := 0; y < 14; y++ {
		for x := 0; x < 14; x++ {
			g.setProt(h.x+x, h.y+y)
			g.setProt(39-x-h.x, h.y+y)
			g.setProt(h.x+x, 39-y-h.y)
			g.setProt(39-x-h.x, 39-y-h.y)
		}
	}
}

// l4LoadDiabQuads places Diablo's four set pieces; preflag picks the closed variants (2b-4b) used while
// the level is generated.
func (g *gen) l4LoadDiabQuads(preflag bool) {
	h := g.l4.hold
	g.l4.quads = [4]point{{h.x + 4, h.y + 4}, {27 - h.x, 1 + h.y}, {1 + h.x, 27 - h.y}, {28 - h.x, 28 - h.y}}
	suffix := "a"
	if preflag {
		suffix = "b"
	}
	for i, name := range [4]string{"l4/diab1", "l4/diab2" + suffix, "l4/diab3" + suffix, "l4/diab4" + suffix} {
		g.placeDunTiles(g.loadDun(name), g.l4.quads[i].x, g.l4.quads[i].y, 6)
	}
}

func (g *gen) l4FixTransparency() {
	yy := 16
	for j := 0; j < 40; j++ {
		xx := 16
		for i := 0; i < 40; i++ {
			t := g.d(i, j)
			if oneOf(t, 25, 28, 23) && g.d(i, j-1) == 18 {
				g.copyTrans(xx, yy, xx+1, yy)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if oneOf(t, 27, 26, 22) && g.d(i+1, j) == 19 {
				g.copyTrans(xx, yy, xx, yy+1)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == 18 {
				g.copyTrans(xx, yy, xx+1, yy)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == 19 {
				g.copyTrans(xx, yy, xx, yy+1)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == 24 {
				g.copyTrans(xx, yy, xx+1, yy)
				g.copyTrans(xx, yy, xx, yy+1)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == 57 {
				g.copyTrans(xx, yy+1, xx-1, yy)
				g.copyTrans(xx, yy+1, xx, yy)
			}
			if t == 53 {
				g.copyTrans(xx+1, yy, xx, yy-1)
				g.copyTrans(xx+1, yy, xx, yy)
			}
			xx += 2
		}
		yy += 2
	}
}

func (g *gen) l4FixCornerTiles() {
	for j := 1; j < 39; j++ {
		for i := 1; i < 39; i++ {
			if t := g.d(i, j); t >= 18 && t <= 30 && (g.d(i+1, j) < 18 || g.d(i, j+1) < 18) {
				g.sd(i, j, t+98)
			}
		}
	}
}

func (g *gen) l4GeneralFix() {
	for j := 0; j < 39; j++ {
		for i := 0; i < 39; i++ {
			if (g.d(i, j) == 24 || g.d(i, j) == 122) && g.d(i+1, j) == 2 && g.d(i, j+1) == 5 {
				g.sd(i, j, 17)
			}
		}
	}
}

func (g *gen) l4PlaceStairs(entry Entry) bool {
	x, y, ok := g.placeMiniSet(&l4L4USTAIRS, 199, false)
	if !ok {
		return false
	}
	if entry == EntryMain {
		g.view = point{2*x + 22, 2*y + 22}
	}
	if g.currlevel != 15 {
		if g.currlevel != 16 {
			if g.available(QWarlord) {
				if entry == EntryPrev {
					g.view = point{2*g.setPiece.x + 23, 2*g.setPiece.y + 23}
				}
			} else {
				if x, y, ok = g.placeMiniSet(&l4L4DSTAIRS, 199, false); !ok {
					return false
				}
				if entry == EntryPrev {
					g.view = point{2*x + 23, 2*y + 21}
				}
			}
		}
		if g.currlevel == 13 {
			if x, y, ok = g.placeMiniSet(&l4L4TWARP, 199, false); !ok {
				return false
			}
			if entry == EntryTWarpDn {
				g.view = point{2*x + 22, 2*y + 22}
			}
		}
		return true
	}
	if x, y, ok = g.placeMiniSet(&l4L4PENTA2, 199, false); !ok {
		return false
	}
	g.quests[QDiablo].X, g.quests[QDiablo].Y = x, y
	if entry == EntryPrev {
		g.view = point{2*x + 22, 2*y + 21}
	}
	return true
}

func (g *gen) l4GenerateLevel(entry Entry) {
	for {
		g.initTrans()
		for {
			g.mask = [1600]bool{}
			g.protected = [1600]bool{}
			g.fillDungeon(30)
			g.l4FirstRoom()
			for x := 0; x < 20; x++ {
				g.setMask(x, 0, false)
			}
			for y := 0; y < 20; y++ {
				g.setMask(0, y, false)
			}
			if g.maskCount()*4 >= 692 {
				break
			}
		}
		g.l4PrepareInnerBorders()
		g.l4MirrorLayout()
		g.l4MakeDmt()
		g.l4FixTilesPatterns()
		if g.currlevel == 16 {
			g.l4ProtectQuads()
		}
		if g.available(QWarlord) {
			sp := g.setPieceRoom
			for i := sp.x; i < sp.x+sp.w-1; i++ {
				for j := sp.y; j < sp.y+sp.h-1; j++ {
					g.setProt(i, j)
				}
			}
		}
		g.l4AddWall()
		g.floodTrans(6)
		g.l4FixTransparency()
		g.l4InitSetPiece()
		if g.currlevel == 16 {
			g.l4LoadDiabQuads(true)
		}
		if g.l4PlaceStairs(entry) {
			break
		}
	}
	g.l4GeneralFix()
	if g.currlevel != 16 {
		g.placeThemeRooms(7, 10, 6, 8, true)
	}
	g.l4ApplyShadows()
	g.l4FixCornerTiles()
	g.l4Substitution()
	g.savePdungeon()
	g.checkQuests(g.setPieceRoom.x, g.setPieceRoom.y)
	if g.currlevel == 15 {
		q := &g.quests[QDiablo]
		if q.Active != QuestActive && q.Active != QuestDone {
			l4L4PENTA.place(g, q.X, q.Y, false)
		}
		for j := 1; j < 40; j++ {
			for i := 1; i < 40; i++ {
				if t := g.d(i, j); t == 98 || t == 107 {
					g.makeSetPC(rect{i - 1, j - 1, 5, 5})
					g.quests[QBetrayer].X, g.quests[QBetrayer].Y = 2*i+16, 2*j+16
				}
			}
		}
	}
	if g.currlevel == 16 {
		g.l4LoadDiabQuads(false)
	}
}

func (g *gen) createL4(seed int32, entry Entry) {
	g.rng.SetSeed(seed)
	g.l4GenerateLevel(entry)
	g.lPass3(30 - 1)
}
