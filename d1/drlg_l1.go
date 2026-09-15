package d1

// drlg_l1.cpp: the Cathedral.

func (g *gen) l1ChamberTest(x, y int) bool { i := bitIndex(x, y); return i >= 0 && g.l1.chamber[i] }

func (g *gen) l1ApplyShadows() {
	for y := 1; y < 40; y++ {
		for x := 1; x < 40; x++ {
			s00 := l1BaseTypes[g.d(x, y)]
			s10 := l1BaseTypes[g.d(x-1, y)]
			s01 := l1BaseTypes[g.d(x, y-1)]
			s11 := l1BaseTypes[g.d(x-1, y-1)]
			for _, p := range l1ShadowPatterns {
				if p[0] != s00 || p[1] != 0 && p[1] != s11 || p[2] != 0 && p[2] != s01 || p[3] != 0 && p[3] != s10 {
					continue
				}
				if p[4] != 0 && !g.prot(x-1, y-1) {
					g.sd(x-1, y-1, p[4])
				}
				if p[5] != 0 && !g.prot(x, y-1) {
					g.sd(x, y-1, p[5])
				}
				if p[6] != 0 && !g.prot(x-1, y) {
					g.sd(x-1, y, p[6])
				}
			}
		}
	}
	fence := func(t byte) bool {
		switch t {
		case t1DFence, t1VFenceEnd, t1VFence, t1HWallVFence, t1HArchVFence, t1HArchVDoor:
			return true
		}
		return false
	}
	for y := 1; y < 40; y++ {
		for x := 1; x < 40; x++ {
			if g.prot(x-1, y) {
				continue
			}
			for _, s := range [3][2]byte{{t1Floor12, t1Floor14}, {t1HArchShadow, t1HArchShadow2}, {t1HWallShadow, t1HWallShadow2}} {
				if g.d(x-1, y) == s[0] {
					if fence(g.d(x, y)) {
						g.sd(x-1, y, s[1])
					} else {
						g.sd(x-1, y, s[0])
					}
				}
			}
		}
	}
}

func (g *gen) l1FillFloor() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) != t1Floor || g.prot(i, j) {
				continue
			}
			switch g.rnd(3) {
			case 1:
				g.sd(i, j, t1Floor22)
			case 2:
				g.sd(i, j, t1Floor23)
			}
		}
	}
}

func (g *gen) l1InitSetPiece() {
	var name string
	switch {
	case g.available(QButcher):
		name = `Levels\L1Data\rnd6.DUN`
	case g.available(QSkelKing):
		name = `Levels\L1Data\SKngDO.DUN`
	case g.available(QLtBanner):
		name = `Levels\L1Data\Banner2.DUN`
	default:
		return
	}
	d := loadDun(name)
	x, y := g.l1SelectChamber()
	g.placeDunTiles(d, x, y, t1Floor)
	g.setPiece = rect{x, y, d.w(), d.h()}
}

func (g *gen) l1SelectChamber() (int, int) {
	chamber := 2
	switch {
	case g.l1.ch1 && g.l1.ch2 && g.l1.ch3:
		chamber = g.rnd(3) + 1
	case g.l1.ch1 && g.l1.ch2:
		chamber = g.pick(2, 1)
	case g.l1.ch1 && g.l1.ch3:
		chamber = g.pick(3, 1)
	case g.l1.ch2 && g.l1.ch3:
		chamber = g.pick(2, 3)
	}
	switch chamber {
	case 1:
		if g.l1.vertical {
			return 16, 2
		}
		return 2, 16
	case 3:
		if g.l1.vertical {
			return 16, 30
		}
		return 30, 16
	}
	return 16, 16
}

func (g *gen) l1MapRoom(r rect) {
	for y := 0; y < r.h; y++ {
		for x := 0; x < r.w; x++ {
			g.setMask(r.x+x, r.y+y, true)
		}
	}
}

func (g *gen) l1CheckRoom(r rect) bool {
	for j := 0; j < r.h; j++ {
		for i := 0; i < r.w; i++ {
			if i+r.x < 0 || i+r.x >= 40 || j+r.y < 0 || j+r.y >= 40 || g.masked(i+r.x, j+r.y) {
				return false
			}
		}
	}
	return true
}

func (g *gen) l1GenerateRoom(area rect, vertical bool) {
	rotate := g.flip(4)
	vertical = !vertical && rotate || vertical && !rotate
	var room1 rect
	place1 := false
	for num := 0; num < 20; num++ {
		room1.w = (g.rnd(5) + 2) &^ 1
		room1.h = (g.rnd(5) + 2) &^ 1
		if vertical {
			room1.x, room1.y = area.x-room1.w, area.y+area.h/2-room1.h/2
			place1 = g.l1CheckRoom(rect{room1.x - 1, room1.y - 1, room1.h + 2, room1.w + 1}) // D1 bug: width and height swapped
		} else {
			room1.x, room1.y = area.x+area.w/2-room1.w/2, area.y-room1.h
			place1 = g.l1CheckRoom(rect{room1.x - 1, room1.y - 1, room1.w + 2, room1.h + 1})
		}
		if place1 {
			break
		}
	}
	if place1 {
		g.l1MapRoom(rect{room1.x, room1.y, min(40-room1.x, room1.w), min(40-room1.y, room1.h)})
	}
	room2 := room1
	var place2 bool
	if vertical {
		room2.x = area.x + area.w
		place2 = g.l1CheckRoom(rect{room2.x, room2.y - 1, room2.w + 1, room2.h + 2})
	} else {
		room2.y = area.y + area.h
		place2 = g.l1CheckRoom(rect{room2.x - 1, room2.y, room2.w + 2, room2.h + 1})
	}
	if place2 {
		g.l1MapRoom(room2)
	}
	if place1 {
		g.l1GenerateRoom(room1, !vertical)
	}
	if place2 {
		g.l1GenerateRoom(room2, !vertical)
	}
}

func (g *gen) l1FirstRoom() {
	g.mask = [1600]bool{}
	g.l1.vertical = g.flip(2)
	g.l1.ch1 = !g.flip(2)
	g.l1.ch2 = !g.flip(2)
	g.l1.ch3 = !g.flip(2)
	if !g.l1.ch1 || !g.l1.ch3 {
		g.l1.ch2 = true
	}
	chamber1, chamber2, chamber3 := rect{1, 15, 10, 10}, rect{15, 15, 10, 10}, rect{29, 15, 10, 10}
	hallway := rect{1, 17, 38, 6}
	if !g.l1.ch1 {
		hallway.x += 17
		hallway.w -= 17
	}
	if !g.l1.ch3 {
		hallway.w -= 16
	}
	if g.l1.vertical {
		chamber1.x, chamber1.y = chamber1.y, chamber1.x
		chamber3.x, chamber3.y = chamber3.y, chamber3.x
		hallway = rect{hallway.y, hallway.x, hallway.h, hallway.w}
	}
	if g.l1.ch1 {
		g.l1MapRoom(chamber1)
	}
	if g.l1.ch2 {
		g.l1MapRoom(chamber2)
	}
	if g.l1.ch3 {
		g.l1MapRoom(chamber3)
	}
	g.l1MapRoom(hallway)
	if g.l1.ch1 {
		g.l1GenerateRoom(chamber1, g.l1.vertical)
	}
	if g.l1.ch2 {
		g.l1GenerateRoom(chamber2, g.l1.vertical)
	}
	if g.l1.ch3 {
		g.l1GenerateRoom(chamber3, g.l1.vertical)
	}
}

func (g *gen) l1MakeDmt() {
	for j := 0; j < 39; j++ {
		for i := 0; i < 39; i++ {
			var t byte
			switch {
			case g.masked(i, j):
				t = t1Floor
			case !g.masked(i+1, j+1) && g.masked(i, j+1) && g.masked(i+1, j):
				t = t1Floor
			case g.masked(i+1, j+1) && g.masked(i, j+1) && g.masked(i+1, j):
				t = t1VCorner
			case g.masked(i, j+1):
				t = t1HWall
			case g.masked(i+1, j):
				t = t1VWall
			case g.masked(i+1, j+1):
				t = t1DWall
			default:
				t = t1Dirt
			}
			g.sd(i, j, t)
		}
	}
}

func l1WallEnd(t byte) bool {
	switch t {
	case t1Corner, t1DWall, t1DArch, t1VWallEnd, t1HWallEnd, t1VCorner, t1HCorner, t1DirtHwall, t1DirtVwall, t1VDirtCorner,
		t1HDirtCorner, t1DirtHwallEnd, t1DirtVwallEnd:
		return true
	}
	return false
}

func (g *gen) l1HorizontalWallOk(x, y int) int {
	length := 1
	for ; g.d(x+length, y) == t1Floor; length++ {
		if g.d(x+length, y-1) != t1Floor || g.d(x+length, y+1) != t1Floor || g.prot(x+length, y) || g.l1ChamberTest(x+length, y) {
			break
		}
	}
	if length == 1 || !l1WallEnd(g.d(x+length, y)) {
		return -1
	}
	return length
}

func (g *gen) l1VerticalWallOk(x, y int) int {
	length := 1
	for ; g.d(x, y+length) == t1Floor; length++ {
		if g.d(x-1, y+length) != t1Floor || g.d(x+1, y+length) != t1Floor || g.prot(x, y+length) || g.l1ChamberTest(x, y+length) {
			break
		}
	}
	if length == 1 || !l1WallEnd(g.d(x, y+length)) {
		return -1
	}
	return length
}

func (g *gen) l1HorizontalWall(x, y int, start byte, maxX int) {
	wall, door := byte(t1HWall), byte(t1HDoor)
	switch g.rnd(4) {
	case 2:
		wall, door = t1HArch, t1HArch
		if start == t1HWall {
			start = t1HArch
		} else if start == t1DWall {
			start = t1HArchVWall
		}
	case 3:
		wall = t1HFence
		if start == t1HWall {
			start = t1HFence
		} else if start == t1DWall {
			start = t1HFenceVWall
		}
	}
	if g.rnd(6) == 5 {
		door = t1HArch
	}
	g.sd(x, y, start)
	for i := 1; i < maxX; i++ {
		g.sd(x+i, y, wall)
	}
	i := g.rnd(maxX-1) + 1
	g.sd(x+i, y, door)
	if door == t1HDoor {
		g.setProt(x+i, y)
	}
}

func (g *gen) l1VerticalWall(x, y int, start byte, maxY int) {
	wall, door := byte(t1VWall), byte(t1VDoor)
	switch g.rnd(4) {
	case 2:
		wall, door = t1VArch, t1VArch
		if start == t1VWall {
			start = t1VArch
		} else if start == t1DWall {
			start = t1HWallVArch
		}
	case 3:
		wall = t1VFence
		if start == t1VWall {
			start = t1VFence
		} else if start == t1DWall {
			start = t1HWallVFence
		}
	}
	if g.rnd(6) == 5 {
		door = t1VArch
	}
	g.sd(x, y, start)
	for i := 1; i < maxY; i++ {
		g.sd(x, y+i, wall)
	}
	i := g.rnd(maxY-1) + 1
	g.sd(x, y+i, door)
	if door == t1VDoor {
		g.setProt(x, y+i)
	}
}

func (g *gen) l1AddWall() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.prot(i, j) || g.l1ChamberTest(i, j) {
				continue
			}
			type wallRule struct {
				tile, start byte
				horiz       bool
			}
			for _, r := range [6]wallRule{{t1Corner, t1HWall, true}, {t1Corner, t1VWall, false}, {t1VWallEnd, t1DWall, true},
				{t1HWallEnd, t1DWall, false}, {t1HWall, t1HWall, true}, {t1VWall, t1VWall, false}} {
				if g.d(i, j) != r.tile {
					continue
				}
				g.discard(1)
				if r.horiz {
					if n := g.l1HorizontalWallOk(i, j); n != -1 {
						g.l1HorizontalWall(i, j, r.start, n)
					}
				} else if n := g.l1VerticalWallOk(i, j); n != -1 {
					g.l1VerticalWall(i, j, r.start, n)
				}
			}
		}
	}
}

func (g *gen) l1GenerateChamber(x, y int, connectPrevious, connectNext, vertical bool) {
	if connectPrevious {
		if vertical {
			g.sd(x+2, y, t1HArch)
			g.sd(x+3, y, t1HArch)
			g.sd(x+4, y, t1Corner)
			g.sd(x+7, y, t1VArchEnd)
			g.sd(x+8, y, t1HArch)
			g.sd(x+9, y, t1HWall)
		} else {
			g.sd(x, y+2, t1VArch)
			g.sd(x, y+3, t1VArch)
			g.sd(x, y+4, t1Corner)
			g.sd(x, y+7, t1HArchEnd)
			g.sd(x, y+8, t1VArch)
			g.sd(x, y+9, t1VWall)
		}
	}
	if connectNext {
		if vertical {
			yy := y + 11
			g.sd(x+2, yy, t1HArchVWall)
			g.sd(x+3, yy, t1HArch)
			g.sd(x+4, yy, t1HArchEnd)
			g.sd(x+7, yy, t1DArch)
			g.sd(x+8, yy, t1HArch)
			if g.d(x+9, yy) != t1DWall {
				g.sd(x+9, yy, t1HDirtCorner)
			}
		} else {
			xx := x + 11
			g.sd(xx, y+2, t1HWallVArch)
			g.sd(xx, y+3, t1VArch)
			g.sd(xx, y+4, t1VArchEnd)
			g.sd(xx, y+7, t1DArch)
			g.sd(xx, y+8, t1VArch)
			if g.d(xx, y+9) != t1DWall {
				g.sd(xx, y+9, t1HDirtCorner)
			}
		}
	}
	for yy := 1; yy < 11; yy++ {
		for xx := 1; xx < 11; xx++ {
			g.sd(x+xx, y+yy, t1Floor)
			if i := bitIndex(x+xx, y+yy); i >= 0 {
				g.l1.chamber[i] = true
			}
		}
	}
	g.sd(x+4, y+4, t1Pillar)
	g.sd(x+7, y+4, t1Pillar)
	g.sd(x+4, y+7, t1Pillar)
	g.sd(x+7, y+7, t1Pillar)
}

func (g *gen) l1GenerateHall(x, y, length int, vertical bool) {
	if vertical {
		for i := y; i < y+length; i++ {
			g.sd(x, i, t1VArch)
			g.sd(x+3, i, t1VArch)
		}
		return
	}
	for i := x; i < x+length; i++ {
		g.sd(i, y, t1HArch)
		g.sd(i, y+3, t1HArch)
	}
}

// tileRule: if dungeon[i][j] == cur and dungeon[i+dx][j+dy] == match, it becomes to.
type tileRule struct {
	cur    byte
	dx, dy int
	match  byte
	to     byte
}

func (g *gen) applyRule(i, j int, r tileRule) {
	if g.d(i, j) == r.cur && g.d(i+r.dx, j+r.dy) == r.match {
		g.sd(i+r.dx, j+r.dy, r.to)
	}
}

func (g *gen) l1FixTilesPatterns() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if i+1 < 40 {
				for _, r := range [...]tileRule{{t1HWall, 1, 0, t1Dirt, t1DirtHwallEnd}, {t1Floor, 1, 0, t1Dirt, t1DirtHwall},
					{t1Floor, 1, 0, t1HWall, t1HWallEnd}, {t1VWallEnd, 1, 0, t1Dirt, t1DirtVwallEnd}} {
					g.applyRule(i, j, r)
				}
			}
			if j+1 < 40 {
				for _, r := range [...]tileRule{{t1VWall, 0, 1, t1Dirt, t1DirtVwallEnd}, {t1Floor, 0, 1, t1VWall, t1VWallEnd},
					{t1Floor, 0, 1, t1Dirt, t1DirtVwall}} {
					g.applyRule(i, j, r)
				}
			}
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if i+1 < 40 {
				for _, r := range [...]tileRule{{t1Floor, 1, 0, t1DirtVwall, t1HDirtCorner}, {t1Floor, 1, 0, t1Dirt, t1VDirtCorner},
					{t1HWallEnd, 1, 0, t1Dirt, t1DirtHwallEnd}, {t1Floor, 1, 0, t1DirtVwallEnd, t1HDirtCorner},
					{t1DirtVwall, 1, 0, t1Dirt, t1VDirtCorner}, {t1HWall, 1, 0, t1DirtVwall, t1HDirtCorner},
					{t1DirtVwall, 1, 0, t1VWall, t1VWallEnd}, {t1HWallEnd, 1, 0, t1DirtVwall, t1HDirtCorner},
					{t1HWall, 1, 0, t1VWall, t1VWallEnd}, {t1Corner, 1, 0, t1Dirt, t1DirtVwallEnd},
					{t1HDirtCorner, 1, 0, t1VWall, t1VWallEnd}, {t1HWallEnd, 1, 0, t1VWall, t1VWallEnd},
					{t1HWallEnd, 1, 0, t1DirtVwallEnd, t1HDirtCorner}, {t1DWall, 1, 0, t1VCorner, t1HCorner},
					{t1HWallEnd, 1, 0, t1Floor, t1HCorner}, {t1HWall, 1, 0, t1DirtVwallEnd, t1HDirtCorner},
					{t1HWall, 1, 0, t1Floor, t1HCorner}} {
					g.applyRule(i, j, r)
				}
			}
			if i > 0 {
				for _, r := range [...]tileRule{{t1DirtHwallEnd, -1, 0, t1Dirt, t1DirtVwall}, {t1DirtVwall, -1, 0, t1DirtHwallEnd, t1HDirtCorner},
					{t1VWallEnd, -1, 0, t1Dirt, t1DirtVwallEnd}, {t1VWallEnd, -1, 0, t1DirtHwallEnd, t1HDirtCorner}} {
					g.applyRule(i, j, r)
				}
			}
			if j+1 < 40 {
				for _, r := range [...]tileRule{{t1VWall, 0, 1, t1HWall, t1HWallEnd}, {t1VWallEnd, 0, 1, t1DirtHwall, t1HDirtCorner},
					{t1DirtHwall, 0, 1, t1HWall, t1HWallEnd}, {t1VWallEnd, 0, 1, t1HWall, t1HWallEnd},
					{t1HDirtCorner, 0, 1, t1HWall, t1HWallEnd}, {t1VWallEnd, 0, 1, t1Dirt, t1DirtVwallEnd},
					{t1VWallEnd, 0, 1, t1Floor, t1VCorner}, {t1VWall, 0, 1, t1Floor, t1VCorner},
					{t1Floor, 0, 1, t1VCorner, t1HCorner}} {
					g.applyRule(i, j, r)
				}
			}
			if j > 0 {
				for _, r := range [...]tileRule{{t1VWallEnd, 0, -1, t1Dirt, t1HWallEnd}, {t1VWallEnd, 0, -1, t1Dirt, t1DirtVwallEnd},
					{t1HWallEnd, 0, -1, t1DirtVwallEnd, t1HDirtCorner}, {t1DirtHwall, 0, -1, t1DirtVwallEnd, t1HDirtCorner}} {
					g.applyRule(i, j, r)
				}
			}
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if j+1 < 40 {
				g.applyRule(i, j, tileRule{t1DWall, 0, 1, t1HWall, t1HWallEnd})
			}
			if i+1 < 40 {
				g.applyRule(i, j, tileRule{t1HWall, 1, 0, t1DirtVwall, t1HDirtCorner})
			}
			if j+1 < 40 {
				g.applyRule(i, j, tileRule{t1DirtHwall, 0, 1, t1Dirt, t1VDirtCorner})
			}
		}
	}
}

func (g *gen) l1Substitution() {
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			if !g.flip(4) {
				continue
			}
			c := l1TileDecorations[g.d(x, y)]
			if c == 0 || g.prot(x, y) {
				continue
			}
			rv := g.rnd(16)
			i := -1
			for rv >= 0 {
				if i++; i == len(l1TileDecorations) {
					i = 0
				}
				if c == l1TileDecorations[i] {
					rv--
				}
			}
			if i == t1VWall4 && y > 0 {
				if l1TileDecorations[g.d(x, y-1)] != t1VWall2 || g.prot(x, y-1) {
					i = t1VWall2
				} else {
					g.sd(x, y-1, t1VWall5)
				}
			}
			if i == t1HWall4 && x+1 < 40 {
				if l1TileDecorations[g.d(x+1, y)] != t1HWall2 || g.prot(x+1, y) {
					i = t1HWall2
				} else {
					g.sd(x+1, y, t1HWall5)
				}
			}
			g.sd(x, y, byte(i))
		}
	}
}

func (g *gen) l1FillChambers() {
	chamber1, chamber3, hall1, hall2 := point{0, 14}, point{28, 14}, point{12, 18}, point{26, 18}
	v := g.l1.vertical
	if v {
		chamber1, chamber3, hall1, hall2 = point{14, 0}, point{14, 28}, point{18, 12}, point{18, 26}
	}
	if g.l1.ch1 {
		g.l1GenerateChamber(chamber1.x, chamber1.y, false, true, v)
	}
	if g.l1.ch2 {
		g.l1GenerateChamber(14, 14, g.l1.ch1, g.l1.ch3, v)
	}
	if g.l1.ch3 {
		g.l1GenerateChamber(chamber3.x, chamber3.y, true, false, v)
	}
	if g.l1.ch2 {
		if g.l1.ch1 {
			g.l1GenerateHall(hall1.x, hall1.y, 2, v)
		}
		if g.l1.ch3 {
			g.l1GenerateHall(hall2.x, hall2.y, 2, v)
		}
	} else {
		g.l1GenerateHall(hall1.x, hall1.y, 16, v)
	}
	g.l1InitSetPiece()
}

func (g *gen) l1FixTransparency() {
	yy := 16
	for j := 0; j < 40; j++ {
		xx := 16
		for i := 0; i < 40; i++ {
			t := g.d(i, j)
			if t == t1DirtHwallEnd && j > 0 && g.d(i, j-1) == t1DirtHwall {
				g.copyTrans(xx, yy, xx+1, yy)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == t1DirtVwallEnd && i+1 < 40 && g.d(i+1, j) == t1DirtVwall {
				g.copyTrans(xx, yy, xx, yy+1)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == t1DirtHwall {
				g.copyTrans(xx, yy, xx+1, yy)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == t1DirtVwall {
				g.copyTrans(xx, yy, xx, yy+1)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			if t == t1VDirtCorner {
				g.copyTrans(xx, yy, xx+1, yy)
				g.copyTrans(xx, yy, xx, yy+1)
				g.copyTrans(xx, yy, xx+1, yy+1)
			}
			xx += 2
		}
		yy += 2
	}
}

func (g *gen) l1FixDirtTiles() {
	for j := 0; j < 39; j++ {
		for i := 0; i < 39; i++ {
			if g.d(i, j) == t1HDirtCorner && g.d(i+1, j) != t1DirtVwall {
				g.sd(i, j, t1DirtCorner2)
			}
			if g.d(i, j) == t1DirtVwall && g.d(i+1, j) != t1DirtVwall {
				g.sd(i, j, t1DirtVWall2)
			}
			if g.d(i, j) == t1DirtVwallEnd && g.d(i+1, j) != t1DirtVwall {
				g.sd(i, j, t1DirtVWallEnd2)
			}
			if g.d(i, j) == t1DirtHwall && g.d(i, j+1) != t1DirtHwall {
				g.sd(i, j, t1DirtHWall2)
			}
			if g.d(i, j) == t1HDirtCorner && g.d(i, j+1) != t1DirtHwall {
				g.sd(i, j, t1DirtCorner2)
			}
			if g.d(i, j) == t1DirtHwallEnd && g.d(i, j+1) != t1DirtHwall {
				g.sd(i, j, t1DirtHWallEnd2)
			}
		}
	}
}

func (g *gen) l1FixCornerTiles() {
	for j := 1; j < 39; j++ {
		for i := 1; i < 39; i++ {
			if !g.prot(i, j) && g.d(i, j) == t1HCorner && g.d(i-1, j) == t1Floor && g.d(i, j-1) == t1VWall {
				g.sd(i, j, t1VCorner)
			}
			if g.d(i, j) == t1DirtCorner2 && g.d(i+1, j) == t1Floor && g.d(i, j+1) == t1VWall {
				g.sd(i, j, t1HArchEnd)
			}
			if g.d(i, j) == t1DirtCorner2 && g.d(i, j+1) == t1Floor && g.d(i+1, j) == t1HWall {
				g.sd(i, j, t1VArchEnd)
			}
		}
	}
}

func (g *gen) l1PlaceStairs(entry Entry) bool {
	ok := true
	if g.available(QPWater) {
		if x, y, found := g.placeMiniSet(&l1PWATERIN, 1600, true); !found {
			ok = false
		} else {
			t := g.transVal
			g.transVal = 0
			g.mRectTrans(x, y+2, x+5, y+4)
			g.transVal = t
			g.quests[QPWater].X, g.quests[QPWater].Y = 2*x+21, 2*y+22
			if entry == EntryRtnLvl {
				g.view = point{2*x + 21, 2*y + 22}
			}
		}
	}
	stairs := &l1L5STAIRSUP
	if g.available(QLtBanner) {
		stairs = &l1STAIRSUP
	}
	if x, y, found := g.placeMiniSet(stairs, 1600, true); !found {
		return false
	} else if entry == EntryMain {
		g.view = point{2*x + 19, 2*y + 20}
	}
	if g.available(QLtBanner) {
		if entry == EntryPrev {
			g.view = point{2*g.setPiece.x + 19, 2*g.setPiece.y + 27}
		}
	} else if x, y, found := g.placeMiniSet(&l1STAIRSDOWN, 1600, true); !found {
		ok = false
	} else if entry == EntryPrev {
		g.view = point{2*x + 19, 2*y + 19}
	}
	return ok
}

func (g *gen) l1GenerateLevel(entry Entry) {
	minarea := map[int]int{1: 533, 2: 693}[g.currlevel]
	if minarea == 0 {
		minarea = 761
	}
	for {
		g.initTrans()
		for {
			g.l1FirstRoom()
			if g.maskCount() >= minarea {
				break
			}
		}
		g.fillDungeon(t1Dirt)
		g.protected = [1600]bool{}
		g.l1.chamber = [1600]bool{}
		g.l1MakeDmt()
		g.l1FillChambers()
		g.l1FixTilesPatterns()
		g.l1AddWall()
		g.floodTrans(t1Floor)
		if g.l1PlaceStairs(entry) {
			break
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.d(i, j) == t1EntranceStairs {
				xx, yy := 2*i+16, 2*j+16
				g.copyTrans(xx, yy+1, xx, yy)
				g.copyTrans(xx+1, yy+1, xx+1, yy)
			}
		}
	}
	g.l1FixTransparency()
	g.l1FixDirtTiles()
	g.l1FixCornerTiles()
	g.l1Substitution()
	g.l1ApplyShadows()
	for n := g.rnd(5) + 5; n > 0; n-- {
		g.placeMiniSet(&l1LAMPS, 1600, true)
	}
	g.l1FillFloor()
	g.savePdungeon()
	g.checkQuests(g.setPiece.x, g.setPiece.y)
}

func (g *gen) createL1(seed int32, entry Entry) {
	g.rng.SetSeed(seed)
	g.l1GenerateLevel(entry)
	g.lPass3(t1Dirt - 1)
}

// loadTransparency ports DRLG_SetMapTrans: room regions from a .DUN's transparency layer.
func (g *gen) loadTransparency(d dun) {
	for j := 0; j < 2*d.h(); j++ {
		for i := 0; i < 2*d.w(); i++ {
			g.setTrans(16+i, 16+j, int8(d.layer(3, i, j)))
		}
	}
}
