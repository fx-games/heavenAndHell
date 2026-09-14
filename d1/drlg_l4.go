package d1

import "math/rand"

// l4 holds drlg_l4.cpp's globals.
type l4 struct {
	drlg
	dung  [20][20]byte // one quarter, mirrored into L4dungeon
	fine  [80][80]byte // L4dungeon
	edges [40][40]byte // walls AddWall and theme rooms put on a megatile: edgeTop, edgeLeft, edgePost
}

const (
	edgeTop byte = 1 << iota
	edgeLeft
	edgePost // only the top-left cell, where two walls meet
)

var (
	l4ConvTbl = [16]byte{30, 6, 1, 6, 2, 6, 6, 6, 9, 6, 1, 6, 2, 6, 3, 6}
	l4USTAIRS = []byte{4, 5, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
		0, 0, 0, 0, 36, 38, 35, 0, 37, 34, 33, 32, 0, 0, 31, 0, 0, 0, 0, 0}
	l4TWARP = []byte{4, 5, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
		0, 0, 0, 0, 134, 136, 133, 0, 135, 132, 131, 130, 0, 0, 129, 0, 0, 0, 0, 0}
	l4DSTAIRS = []byte{5, 5, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6,
		0, 0, 0, 0, 0, 0, 0, 45, 41, 0, 0, 44, 43, 40, 0, 0, 46, 42, 39, 0, 0, 0, 0, 0, 0}
)

func (g *l4) drawRoom(x, y, w, h int) {
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			if i+x < 20 && j+y < 20 { // the swapped checkRoom args let D1 write past the quarter
				g.dung[i+x][j+y] = 1
			}
		}
	}
}

func (g *l4) checkRoom(x, y, w, h int) bool {
	if x <= 0 || y <= 0 {
		return false
	}
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			if i+x < 0 || i+x >= 20 || j+y < 0 || j+y >= 20 || g.dung[i+x][j+y] != 0 {
				return false
			}
		}
	}
	return true
}

// roomGen ports L4roomGen, the same recursion as L5roomGen on a 20×20 quarter.
func (g *l4) roomGen(x, y, w, h, dir int) {
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

// firstRoom ports L4firstRoom. ponytail: Diablo's and the Warlord's fixed rooms come with milestone 7.
func (g *l4) firstRoom() {
	w, h := g.random(5)+2, g.random(5)+2
	xmin, xmax := (20-w)>>1, 19-w
	x := g.random(xmax-xmin+1) + xmin
	if x+w > 19 {
		x = 19 - w + 1
	}
	ymin, ymax := (20-h)>>1, 19-h
	y := g.random(ymax-ymin+1) + ymin
	if y+h > 19 {
		y = 19 - h + 1
	}
	g.drawRoom(x, y, w, h)
	g.roomGen(x, y, w, h, g.random(2))
}

func (g *l4) area() (n int) {
	for j := 0; j < 20; j++ {
		for i := 0; i < 20; i++ {
			if g.dung[i][j] == 1 {
				n++
			}
		}
	}
	return
}

func (g *l4) d(i, j int) byte {
	if i < 0 || j < 0 || i >= 20 || j >= 20 {
		return 0
	}
	return g.dung[i][j]
}

// uShape ports uShape: a hall from the right edge and one from the bottom edge, so the mirrored halves connect.
func (g *l4) uShape() {
	var hallok [20]bool
	for j := 19; j >= 0; j-- {
		for i := 19; i >= 0; i-- {
			if g.dung[i][j] != 1 {
				hallok[j] = false
			} else {
				hallok[j] = g.d(i, j+1) == 1 && g.d(i+1, j+1) == 0
				i = 0
			}
		}
	}
	for rv := g.random(19) + 1; rv != 0; {
		if hallok[rv] {
			for i := 19; i >= 0; i-- {
				if g.dung[i][rv] == 1 {
					i, rv = -1, 0
				} else {
					g.dung[i][rv] = 1
					if rv < 19 {
						g.dung[i][rv+1] = 1
					}
				}
			}
		} else if rv++; rv == 20 {
			rv = 1
		}
	}
	for i := 19; i >= 0; i-- {
		for j := 19; j >= 0; j-- {
			if g.dung[i][j] != 1 {
				hallok[i] = false
			} else {
				hallok[i] = g.d(i+1, j) == 1 && g.d(i+1, j+1) == 0
				j = 0
			}
		}
	}
	for rv := g.random(19) + 1; rv != 0; {
		if hallok[rv] {
			for j := 19; j >= 0; j-- {
				if g.dung[rv][j] == 1 {
					j, rv = -1, 0
				} else {
					g.dung[rv][j] = 1
					if rv < 19 {
						g.dung[rv+1][j] = 1
					}
				}
			}
		} else if rv++; rv == 20 {
			rv = 1
		}
	}
}

// makeDungeon ports L4makeDungeon: the quarter mirrored into all four corners at 2× scale.
func (g *l4) makeDungeon() {
	for j := 0; j < 20; j++ {
		for i := 0; i < 20; i++ {
			for _, q := range [4][4]int{{0, 0, i, j}, {0, 40, i, 19 - j}, {40, 0, 19 - i, j}, {40, 40, 19 - i, 19 - j}} {
				v := g.dung[q[2]][q[3]]
				k, l := 2*i+q[0], 2*j+q[1]
				g.fine[k][l], g.fine[k][l+1], g.fine[k+1][l], g.fine[k+1][l+1] = v, v, v, v
			}
		}
	}
}

func (g *l4) makeDmt() {
	for j, dy := 0, 1; dy <= 77; j, dy = j+1, dy+2 {
		for i, dx := 0, 1; dx <= 77; i, dx = i+1, dx+2 {
			val := int(g.fine[dx+1][dy+1])
			val = 2*val + int(g.fine[dx][dy+1])
			val = 2*val + int(g.fine[dx+1][dy])
			val = 2*val + int(g.fine[dx][dy])
			g.dungeon[i][j] = l4ConvTbl[val]
		}
	}
}

func (g *l4) hWallOk(i, j int) int {
	x := 1
	for ; g.at(i+x, j) == 6; x++ {
		if g.flag(i+x, j) != 0 || g.at(i+x, j-1) != 6 || g.at(i+x, j+1) != 6 {
			break
		}
	}
	switch g.at(i+x, j) {
	case 10, 12, 13, 15, 16, 21, 22:
		if x > 3 {
			return x
		}
	}
	return -1
}

func (g *l4) vWallOk(i, j int) int {
	y := 1
	for ; g.at(i, j+y) == 6; y++ {
		if g.flag(i, j+y) != 0 || g.at(i-1, j+y) != 6 || g.at(i+1, j+y) != 6 {
			break
		}
	}
	switch g.at(i, j+y) {
	case 8, 9, 11, 14, 15, 16, 21, 23:
		if y > 3 {
			return y
		}
	}
	return -1
}

func (g *l4) horizWall(i, j, dx int) {
	switch g.dungeon[i][j] {
	case 13:
		g.dungeon[i][j] = 17
	case 16:
		g.dungeon[i][j] = 11
	case 12:
		g.dungeon[i][j] = 14
	}
	for xx := 1; xx < dx; xx++ {
		g.dungeon[i+xx][j] = 2
	}
	switch g.at(i+dx, j) {
	case 15:
		g.set(i+dx, j, 14)
	case 10:
		g.set(i+dx, j, 17)
	case 21:
		g.set(i+dx, j, 23)
	case 22:
		g.set(i+dx, j, 29)
	}
	xx := g.random(dx-3) + 1
	g.dungeon[i+xx][j] = 57
	g.dungeon[i+xx+2][j] = 56
	g.dungeon[i+xx+1][j] = 60
	if g.at(i+xx, j-1) == 6 {
		g.set(i+xx, j-1, 58)
	}
	if g.at(i+xx+1, j-1) == 6 {
		g.set(i+xx+1, j-1, 59)
	}
	for n := 0; n < dx; n++ {
		if n != xx+1 {
			g.edges[i+n][j] |= edgeTop
		}
	}
	g.edges[i+dx][j] |= edgePost
}

func (g *l4) vertWall(i, j, dy int) {
	switch g.dungeon[i][j] {
	case 14:
		g.dungeon[i][j] = 17
	case 8:
		g.dungeon[i][j] = 9
	case 15:
		g.dungeon[i][j] = 10
	}
	for yy := 1; yy < dy; yy++ {
		g.dungeon[i][j+yy] = 1
	}
	switch g.at(i, j+dy) {
	case 11:
		g.set(i, j+dy, 17)
	case 9:
		g.set(i, j+dy, 10)
	case 16:
		g.set(i, j+dy, 13)
	case 21:
		g.set(i, j+dy, 22)
	case 23:
		g.set(i, j+dy, 29)
	}
	yy := g.random(dy-3) + 1
	g.dungeon[i][j+yy] = 53
	g.dungeon[i][j+yy+2] = 52
	g.dungeon[i][j+yy+1] = 6
	if g.at(i-1, j+yy) == 6 {
		g.set(i-1, j+yy, 54)
	}
	if g.at(i-1, j+yy-1) == 6 {
		g.set(i-1, j+yy-1, 55)
	}
	for n := 0; n < dy; n++ {
		if n != yy+1 {
			g.edges[i][j+n] |= edgeLeft
		}
	}
	g.edges[i][j+dy] |= edgePost
}

// addWall ports L4AddWall: long open runs between two wall ends get a wall with a one megatile gap.
func (g *l4) addWall() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.flags[i][j] != 0 {
				continue
			}
			// D1 rolls random(0,100) < 100 before each check; always true, but it consumes RNG.
			switch g.dungeon[i][j] {
			case 10, 12, 13, 15, 16, 21, 22:
				if g.random(100) < 100 {
					if x := g.hWallOk(i, j); x != -1 {
						g.horizWall(i, j, x)
					}
				}
			}
			switch g.dungeon[i][j] {
			case 8, 9, 11, 14, 15, 16, 21, 23:
				if g.random(100) < 100 {
					if y := g.vWallOk(i, j); y != -1 {
						g.vertWall(i, j, y)
					}
				}
			}
		}
	}
}

// tileFix ports L4tileFix (transcribed from drlg_l4.cpp). It picks wall pieces; what is walkable comes from
// L4dungeon, but AddWall needs the wall ends it produces.
func (g *l4) tileFix() {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.at(i, j) == 2 && g.at(i+1, j) == 6 {
				g.set(i+1, j, 5)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 13)
			}
			if g.at(i, j) == 1 && g.at(i, j+1) == 2 {
				g.set(i, j+1, 14)
			}
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.at(i, j) == 2 && g.at(i+1, j) == 6 {
				g.set(i+1, j, 2)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 9 {
				g.set(i+1, j, 11)
			}
			if g.at(i, j) == 9 && g.at(i+1, j) == 6 {
				g.set(i+1, j, 12)
			}
			if g.at(i, j) == 14 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 13)
			}
			if g.at(i, j) == 6 && g.at(i+1, j) == 14 {
				g.set(i+1, j, 15)
			}
			if g.at(i, j) == 6 && g.at(i, j+1) == 13 {
				g.set(i, j+1, 16)
			}
			if g.at(i, j) == 1 && g.at(i, j+1) == 9 {
				g.set(i, j+1, 10)
			}
			if g.at(i, j) == 6 && g.at(i, j-1) == 1 {
				g.set(i, j-1, 1)
			}
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.at(i, j) == 13 && g.at(i, j+1) == 30 {
				g.set(i, j+1, 27)
			}
			if g.at(i, j) == 27 && g.at(i+1, j) == 30 {
				g.set(i+1, j, 19)
			}
			if g.at(i, j) == 1 && g.at(i, j+1) == 30 {
				g.set(i, j+1, 27)
			}
			if g.at(i, j) == 27 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 16)
			}
			if g.at(i, j) == 19 && g.at(i+1, j) == 27 {
				g.set(i+1, j, 26)
			}
			if g.at(i, j) == 27 && g.at(i+1, j) == 30 {
				g.set(i+1, j, 19)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 15 {
				g.set(i+1, j, 14)
			}
			if g.at(i, j) == 14 && g.at(i+1, j) == 15 {
				g.set(i+1, j, 14)
			}
			if g.at(i, j) == 22 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 16)
			}
			if g.at(i, j) == 27 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 16)
			}
			if g.at(i, j) == 6 && g.at(i+1, j) == 27 && g.at(i+1, j+1) != 0 {
				g.set(i+1, j, 22)
			}
			if g.at(i, j) == 22 && g.at(i+1, j) == 30 {
				g.set(i+1, j, 19)
			}
			if g.at(i, j) == 21 && g.at(i+1, j) == 1 && g.at(i+1, j-1) == 1 {
				g.set(i+1, j, 13)
			}
			if g.at(i, j) == 14 && g.at(i+1, j) == 30 && g.at(i, j+1) == 6 {
				g.set(i+1, j, 28)
			}
			if g.at(i, j) == 16 && g.at(i+1, j) == 6 && g.at(i, j+1) == 30 {
				g.set(i, j+1, 27)
			}
			if g.at(i, j) == 16 && g.at(i, j+1) == 30 && g.at(i+1, j+1) == 30 {
				g.set(i, j+1, 27)
			}
			if g.at(i, j) == 6 && g.at(i+1, j) == 30 && g.at(i+1, j-1) == 6 {
				g.set(i+1, j, 21)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 27 && g.at(i+1, j+1) == 9 {
				g.set(i+1, j, 29)
			}
			if g.at(i, j) == 9 && g.at(i+1, j) == 15 {
				g.set(i+1, j, 14)
			}
			if g.at(i, j) == 15 && g.at(i+1, j) == 27 && g.at(i+1, j+1) == 2 {
				g.set(i+1, j, 29)
			}
			if g.at(i, j) == 19 && g.at(i+1, j) == 18 {
				g.set(i+1, j, 24)
			}
			if g.at(i, j) == 9 && g.at(i+1, j) == 15 {
				g.set(i+1, j, 14)
			}
			if g.at(i, j) == 19 && g.at(i+1, j) == 19 && g.at(i+1, j-1) == 30 {
				g.set(i+1, j, 24)
			}
			if g.at(i, j) == 24 && g.at(i, j-1) == 30 && g.at(i, j-2) == 6 {
				g.set(i, j-1, 21)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 30 {
				g.set(i+1, j, 28)
			}
			if g.at(i, j) == 15 && g.at(i+1, j) == 30 {
				g.set(i+1, j, 28)
			}
			if g.at(i, j) == 28 && g.at(i, j+1) == 30 {
				g.set(i, j+1, 18)
			}
			if g.at(i, j) == 28 && g.at(i, j+1) == 2 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 19 && g.at(i+2, j) == 2 && g.at(i+1, j-1) == 18 && g.at(i+1, j+1) == 1 {
				g.set(i+1, j, 17)
			}
			if g.at(i, j) == 19 && g.at(i+2, j) == 2 && g.at(i+1, j-1) == 22 && g.at(i+1, j+1) == 1 {
				g.set(i+1, j, 17)
			}
			if g.at(i, j) == 19 && g.at(i+2, j) == 2 && g.at(i+1, j-1) == 18 && g.at(i+1, j+1) == 13 {
				g.set(i+1, j, 17)
			}
			if g.at(i, j) == 21 && g.at(i+2, j) == 2 && g.at(i+1, j-1) == 18 && g.at(i+1, j+1) == 1 {
				g.set(i+1, j, 17)
			}
			if g.at(i, j) == 21 && g.at(i+1, j+1) == 1 && g.at(i+1, j-1) == 22 && g.at(i+2, j) == 3 {
				g.set(i+1, j, 17)
			}
			if g.at(i, j) == 15 && g.at(i+1, j) == 28 && g.at(i+2, j) == 30 && g.at(i+1, j-1) == 6 {
				g.set(i+1, j, 23)
			}
			if g.at(i, j) == 14 && g.at(i+1, j) == 28 && g.at(i+2, j) == 1 {
				g.set(i+1, j, 23)
			}
			if g.at(i, j) == 15 && g.at(i+1, j) == 27 && g.at(i+1, j+1) == 30 {
				g.set(i+1, j, 29)
			}
			if g.at(i, j) == 28 && g.at(i, j+1) == 9 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 21 && g.at(i+1, j-1) == 21 {
				g.set(i+1, j, 24)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 27 && g.at(i+1, j+1) == 30 {
				g.set(i+1, j, 29)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 18 {
				g.set(i+1, j, 25)
			}
			if g.at(i, j) == 21 && g.at(i+1, j) == 9 && g.at(i+2, j) == 2 {
				g.set(i+1, j, 11)
			}
			if g.at(i, j) == 19 && g.at(i+1, j) == 10 {
				g.set(i+1, j, 17)
			}
			if g.at(i, j) == 15 && g.at(i, j+1) == 3 {
				g.set(i, j+1, 4)
			}
			if g.at(i, j) == 22 && g.at(i, j+1) == 9 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 18 && g.at(i, j+1) == 30 {
				g.set(i, j+1, 18)
			}
			if g.at(i, j) == 24 && g.at(i-1, j) == 30 {
				g.set(i-1, j, 19)
			}
			if g.at(i, j) == 21 && g.at(i, j+1) == 2 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 21 && g.at(i, j+1) == 9 {
				g.set(i, j+1, 10)
			}
			if g.at(i, j) == 22 && g.at(i, j+1) == 30 {
				g.set(i, j+1, 18)
			}
			if g.at(i, j) == 21 && g.at(i, j+1) == 30 {
				g.set(i, j+1, 18)
			}
			if g.at(i, j) == 16 && g.at(i, j+1) == 2 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 13 && g.at(i, j+1) == 2 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 22 && g.at(i, j+1) == 2 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 21 && g.at(i+1, j) == 18 && g.at(i+2, j) == 30 {
				g.set(i+1, j, 24)
			}
			if g.at(i, j) == 21 && g.at(i+1, j) == 9 && g.at(i+1, j+1) == 1 {
				g.set(i+1, j, 16)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 27 && g.at(i+1, j+1) == 2 {
				g.set(i+1, j, 29)
			}
			if g.at(i, j) == 23 && g.at(i, j+1) == 2 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 23 && g.at(i, j+1) == 9 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 25 && g.at(i, j+1) == 2 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 22 && g.at(i+1, j) == 9 {
				g.set(i+1, j, 11)
			}
			if g.at(i, j) == 23 && g.at(i+1, j) == 9 {
				g.set(i+1, j, 11)
			}
			if g.at(i, j) == 15 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 16)
			}
			if g.at(i, j) == 11 && g.at(i+1, j) == 15 {
				g.set(i+1, j, 14)
			}
			if g.at(i, j) == 23 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 16)
			}
			if g.at(i, j) == 21 && g.at(i+1, j) == 27 {
				g.set(i+1, j, 26)
			}
			if g.at(i, j) == 21 && g.at(i+1, j) == 18 {
				g.set(i+1, j, 24)
			}
			if g.at(i, j) == 26 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 16)
			}
			if g.at(i, j) == 29 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 16)
			}
			if g.at(i, j) == 29 && g.at(i, j+1) == 2 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 1 && g.at(i, j-1) == 15 {
				g.set(i, j-1, 10)
			}
			if g.at(i, j) == 18 && g.at(i, j+1) == 2 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 23 && g.at(i, j+1) == 30 {
				g.set(i, j+1, 18)
			}
			if g.at(i, j) == 18 && g.at(i, j+1) == 9 {
				g.set(i, j+1, 10)
			}
			if g.at(i, j) == 14 && g.at(i+1, j) == 30 && g.at(i+1, j+1) == 30 {
				g.set(i+1, j, 23)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 28 && g.at(i+1, j-1) == 6 {
				g.set(i+1, j, 23)
			}
			if g.at(i, j) == 23 && g.at(i+1, j) == 18 && g.at(i, j-1) == 6 {
				g.set(i+1, j, 24)
			}
			if g.at(i, j) == 14 && g.at(i+1, j) == 23 && g.at(i+2, j) == 30 {
				g.set(i+1, j, 28)
			}
			if g.at(i, j) == 14 && g.at(i+1, j) == 28 && g.at(i+2, j) == 30 && g.at(i+1, j-1) == 6 {
				g.set(i+1, j, 23)
			}
			if g.at(i, j) == 23 && g.at(i+1, j) == 30 {
				g.set(i+1, j, 19)
			}
			if g.at(i, j) == 29 && g.at(i+1, j) == 30 {
				g.set(i+1, j, 19)
			}
			if g.at(i, j) == 29 && g.at(i, j+1) == 30 {
				g.set(i, j+1, 18)
			}
			if g.at(i, j) == 19 && g.at(i+1, j) == 30 {
				g.set(i+1, j, 19)
			}
			if g.at(i, j) == 21 && g.at(i+1, j) == 30 {
				g.set(i+1, j, 19)
			}
			if g.at(i, j) == 26 && g.at(i+1, j) == 30 {
				g.set(i+1, j, 19)
			}
			if g.at(i, j) == 16 && g.at(i, j+1) == 30 {
				g.set(i, j+1, 18)
			}
			if g.at(i, j) == 13 && g.at(i, j+1) == 9 {
				g.set(i, j+1, 10)
			}
			if g.at(i, j) == 25 && g.at(i, j+1) == 30 {
				g.set(i, j+1, 18)
			}
			if g.at(i, j) == 18 && g.at(i, j+1) == 2 {
				g.set(i, j+1, 15)
			}
			if g.at(i, j) == 11 && g.at(i+1, j) == 3 {
				g.set(i+1, j, 5)
			}
			if g.at(i, j) == 19 && g.at(i+1, j) == 9 {
				g.set(i+1, j, 11)
			}
			if g.at(i, j) == 19 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 13)
			}
			if g.at(i, j) == 19 && g.at(i+1, j) == 13 && g.at(i+1, j-1) == 6 {
				g.set(i+1, j, 16)
			}
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.at(i, j) == 21 && g.at(i, j+1) == 24 && g.at(i, j+2) == 1 {
				g.set(i, j+1, 17)
			}
			if g.at(i, j) == 15 && g.at(i+1, j+1) == 9 && g.at(i+1, j-1) == 1 && g.at(i+2, j) == 16 {
				g.set(i+1, j, 29)
			}
			if g.at(i, j) == 2 && g.at(i-1, j) == 6 {
				g.set(i-1, j, 8)
			}
			if g.at(i, j) == 1 && g.at(i, j-1) == 6 {
				g.set(i, j-1, 7)
			}
			if g.at(i, j) == 6 && g.at(i+1, j) == 15 && g.at(i+1, j+1) == 4 {
				g.set(i+1, j, 10)
			}
			if g.at(i, j) == 1 && g.at(i, j+1) == 3 {
				g.set(i, j+1, 4)
			}
			if g.at(i, j) == 1 && g.at(i, j+1) == 6 {
				g.set(i, j+1, 4)
			}
			if g.at(i, j) == 9 && g.at(i, j+1) == 3 {
				g.set(i, j+1, 4)
			}
			if g.at(i, j) == 10 && g.at(i, j+1) == 3 {
				g.set(i, j+1, 4)
			}
			if g.at(i, j) == 13 && g.at(i, j+1) == 3 {
				g.set(i, j+1, 4)
			}
			if g.at(i, j) == 1 && g.at(i, j+1) == 5 {
				g.set(i, j+1, 12)
			}
			if g.at(i, j) == 1 && g.at(i, j+1) == 16 {
				g.set(i, j+1, 13)
			}
			if g.at(i, j) == 6 && g.at(i, j+1) == 13 {
				g.set(i, j+1, 16)
			}
			if g.at(i, j) == 25 && g.at(i, j+1) == 9 {
				g.set(i, j+1, 10)
			}
			if g.at(i, j) == 13 && g.at(i, j+1) == 5 {
				g.set(i, j+1, 12)
			}
			if g.at(i, j) == 28 && g.at(i, j-1) == 6 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 23)
			}
			if g.at(i, j) == 19 && g.at(i+1, j) == 10 {
				g.set(i+1, j, 17)
			}
			if g.at(i, j) == 21 && g.at(i+1, j) == 9 {
				g.set(i+1, j, 11)
			}
			if g.at(i, j) == 11 && g.at(i+1, j) == 3 {
				g.set(i+1, j, 5)
			}
			if g.at(i, j) == 10 && g.at(i+1, j) == 4 {
				g.set(i+1, j, 12)
			}
			if g.at(i, j) == 14 && g.at(i+1, j) == 4 {
				g.set(i+1, j, 12)
			}
			if g.at(i, j) == 27 && g.at(i+1, j) == 9 {
				g.set(i+1, j, 11)
			}
			if g.at(i, j) == 15 && g.at(i+1, j) == 4 {
				g.set(i+1, j, 12)
			}
			if g.at(i, j) == 21 && g.at(i+1, j) == 1 {
				g.set(i+1, j, 16)
			}
			if g.at(i, j) == 11 && g.at(i+1, j) == 4 {
				g.set(i+1, j, 12)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 3 {
				g.set(i+1, j, 5)
			}
			if g.at(i, j) == 9 && g.at(i+1, j) == 3 {
				g.set(i+1, j, 5)
			}
			if g.at(i, j) == 14 && g.at(i+1, j) == 3 {
				g.set(i+1, j, 5)
			}
			if g.at(i, j) == 15 && g.at(i+1, j) == 3 {
				g.set(i+1, j, 5)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 5 && g.at(i+1, j-1) == 16 {
				g.set(i+1, j, 12)
			}
			if g.at(i, j) == 2 && g.at(i+1, j) == 4 {
				g.set(i+1, j, 12)
			}
			if g.at(i, j) == 9 && g.at(i+1, j) == 4 {
				g.set(i+1, j, 12)
			}
			if g.at(i, j) == 1 && g.at(i, j-1) == 8 {
				g.set(i, j-1, 9)
			}
			if g.at(i, j) == 28 && g.at(i+1, j) == 23 && g.at(i+1, j+1) == 3 {
				g.set(i+1, j, 16)
			}
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.at(i, j) == 21 && g.at(i+1, j) == 10 {
				g.set(i+1, j, 17)
			}
			if g.at(i, j) == 17 && g.at(i+1, j) == 4 {
				g.set(i+1, j, 12)
			}
			if g.at(i, j) == 10 && g.at(i+1, j) == 4 {
				g.set(i+1, j, 12)
			}
			if g.at(i, j) == 17 && g.at(i, j+1) == 5 {
				g.set(i, j+1, 12)
			}
			if g.at(i, j) == 29 && g.at(i, j+1) == 9 {
				g.set(i, j+1, 10)
			}
			if g.at(i, j) == 13 && g.at(i, j+1) == 5 {
				g.set(i, j+1, 12)
			}
			if g.at(i, j) == 9 && g.at(i, j+1) == 16 {
				g.set(i, j+1, 13)
			}
			if g.at(i, j) == 10 && g.at(i, j+1) == 16 {
				g.set(i, j+1, 13)
			}
			if g.at(i, j) == 16 && g.at(i, j+1) == 3 {
				g.set(i, j+1, 4)
			}
			if g.at(i, j) == 11 && g.at(i, j+1) == 5 {
				g.set(i, j+1, 12)
			}
			if g.at(i, j) == 10 && g.at(i+1, j) == 3 && g.at(i+1, j-1) == 16 {
				g.set(i+1, j, 12)
			}
			if g.at(i, j) == 16 && g.at(i, j+1) == 5 {
				g.set(i, j+1, 12)
			}
			if g.at(i, j) == 1 && g.at(i, j+1) == 6 {
				g.set(i, j+1, 4)
			}
			if g.at(i, j) == 21 && g.at(i+1, j) == 13 && g.at(i, j+1) == 10 {
				g.set(i+1, j+1, 12)
			}
			if g.at(i, j) == 15 && g.at(i+1, j) == 10 {
				g.set(i+1, j, 17)
			}
			if g.at(i, j) == 22 && g.at(i, j+1) == 11 {
				g.set(i, j+1, 17)
			}
			if g.at(i, j) == 15 && g.at(i+1, j) == 28 && g.at(i+2, j) == 16 {
				g.set(i+1, j, 23)
			}
			if g.at(i, j) == 28 && g.at(i+1, j) == 23 && g.at(i+1, j+1) == 1 && g.at(i+2, j) == 6 {
				g.set(i+1, j, 16)
			}
		}
	}
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			if g.at(i, j) == 15 && g.at(i+1, j) == 28 && g.at(i+2, j) == 16 {
				g.set(i+1, j, 23)
			}
			if g.at(i, j) == 21 && g.at(i+1, j-1) == 21 && g.at(i+1, j+1) == 13 && g.at(i+2, j) == 2 {
				g.set(i+1, j, 17)
			}
			if g.at(i, j) == 19 && g.at(i+1, j) == 15 && g.at(i+1, j+1) == 12 {
				g.set(i+1, j, 17)
			}
		}
	}
}

// GenerateL4 ports DRLG_L4 (Hell) down to the grid: floor from L4dungeon like the cathedral, plus the walls AddWall
// and theme rooms add. ponytail: level 15's pentagram and level 16's Diablo quarters come with milestone 7;
// shadows, corner pieces and floor variations (Shadows, Corners, Subs) don't change what's walkable.
func GenerateL4(rng *rand.Rand, dlvl int) *Level {
	for {
		g := &l4{drlg: newDrlg(rng)}
		for {
			g.dung = [20][20]byte{}
			g.firstRoom()
			for n := 0; n < 20; n++ { // L4FixRim
				g.dung[n][0], g.dung[0][n] = 0, 0
			}
			if g.area() >= 173 {
				break
			}
		}
		g.uShape()
		g.makeDungeon()
		g.makeDmt()
		g.tileFix()
		g.addWall()
		g.floodTVal(6)
		ux, uy := g.placeMiniSet(l4USTAIRS, 200, 8)
		if ux < 0 {
			continue
		}
		dx, dy := -1, -1
		if dlvl != 16 {
			if dx, dy = g.placeMiniSet(l4DSTAIRS, 200, 8); dx < 0 {
				continue
			}
		}
		wx, wy := -1, -1
		if dlvl == 13 {
			if wx, wy = g.placeMiniSet(l4TWARP, 200, 8); wx < 0 {
				continue
			}
		}
		if dlvl != 16 {
			g.placeThemeRooms(false, 7, 10, 6, 8, true, func(t themeLoc) {
				g.drawThemeRoom(t, 2, 1, 6, [4]byte{9, 16, 15, 12})
				for i := t.x; i < t.x+t.w; i++ {
					g.edges[i][t.y] |= edgeTop
					g.edges[i][t.y+t.h-1] |= edgeTop
				}
				for j := t.y; j < t.y+t.h; j++ {
					g.edges[t.x][j] |= edgeLeft
					g.edges[t.x+t.w-1][j] |= edgeLeft
				}
				r, b := t.x+t.w-1, t.y+t.h-1
				g.edges[r][t.y] &^= edgeTop
				g.edges[r][b] = edgePost
				g.edges[t.x][b] &^= edgeLeft
				if g.random(2) == 0 {
					m := t.y + t.h/2
					g.set(r, m-1, 53)
					g.set(r, m, 6)
					g.set(r, m+1, 52)
					g.set(r-1, m-1, 54)
					g.edges[r][m] = 0
				} else {
					m := t.x + t.w/2
					g.set(m-1, b, 57)
					g.set(m, b, 6)
					g.set(m+1, b, 56)
					g.set(m, b-1, 59)
					g.set(m-1, b-1, 58)
					g.edges[m][b] = 0
				}
			})
		}
		lvl := g.level()
		lvl.dilate(&g.fine)
		for j := 0; j < 40; j++ {
			for i := 0; i < 40; i++ {
				e := g.edges[i][j]
				if e&(edgeTop|edgeLeft|edgePost) != 0 {
					lvl.Cells[2*i][2*j] = CellWall
				}
				if e&edgeTop != 0 {
					lvl.Cells[2*i+1][2*j] = CellWall
				}
				if e&edgeLeft != 0 {
					lvl.Cells[2*i][2*j+1] = CellWall
				}
			}
		}
		lvl.StairsUp = lvl.nearestFloor(2*ux+4, 2*uy+4)
		if dx >= 0 {
			lvl.StairsDn = lvl.nearestFloor(2*dx+5, 2*dy+5)
		}
		if wx >= 0 {
			lvl.Warp = lvl.nearestFloor(2*wx+4, 2*wy+4)
		}
		if dlvl == 16 {
			ok := false
			if lvl.Chamber, ok = lvl.sealChamber(); !ok {
				continue
			}
		}
		return lvl
	}
}

// sealChamber walls in a 3×3 room for Diablo inside a 7×7 open area as far from the stairs as possible, so the walls
// cut nothing else off. ponytail: stands in for D1's four quadrant set pieces (diab1-4.DUN live in the MPQ).
func (l *Level) sealChamber() ([2]int, bool) {
	best, bestDist := [2]int{}, -1
	for x := 3; x < LevelSize-3; x++ {
		for y := 3; y < LevelSize-3; y++ {
			open := true
			for dx := -3; dx <= 3 && open; dx++ {
				for dy := -3; dy <= 3 && open; dy++ {
					open = l.Cells[x+dx][y+dy] == CellFloor
				}
			}
			if d := max(iabs(x-l.StairsUp[0]), iabs(y-l.StairsUp[1])); open && d > bestDist {
				best, bestDist = [2]int{x, y}, d
			}
		}
	}
	if bestDist < 0 {
		return best, false
	}
	for dx := -2; dx <= 2; dx++ {
		for dy := -2; dy <= 2; dy++ {
			if max(iabs(dx), iabs(dy)) == 2 {
				l.Cells[best[0]+dx][best[1]+dy] = CellWall
			}
			l.Held[best[0]+dx][best[1]+dy] = true
		}
	}
	return best, true
}
