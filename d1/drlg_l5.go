package d1

// drlg_l1.cpp's HELLFIRE branches: the Crypt (dungeon levels 21-24) runs the Cathedral's generator with its own
// stairs, set rooms and decoration passes.

var (
	l5STAIRSUP   = miniset{4, 5, [6][6]byte{{22, 22, 22, 22}, {22, 22, 22, 22}, {2, 2, 2, 2}, {13, 13, 13, 13}, {13, 13, 13, 13}}, [6][6]byte{{0, 54, 23, 0}, {0, 53, 18, 0}, {55, 56, 57, 0}, {58, 59, 60, 0}}}
	l5STAIRSDOWN = miniset{4, 5, [6][6]byte{{13, 13, 13, 13}, {13, 13, 13, 13}, {13, 13, 13, 13}, {13, 13, 13, 13}, {13, 13, 13, 13}}, [6][6]byte{{0, 0, 52, 0}, {0, 48, 51, 0}, {0, 47, 50, 0}, {45, 46, 49, 0}}}
	l5STAIRSTOWN = miniset{4, 5, [6][6]byte{{22, 22, 22, 22}, {22, 22, 22, 22}, {2, 2, 2, 2}, {13, 13, 13, 13}, {13, 13, 13, 13}}, [6][6]byte{{0, 62, 23, 0}, {0, 61, 18, 0}, {63, 64, 65, 0}, {66, 67, 68, 0}}}
	l5VWALL      = miniset{1, 3, [6][6]byte{{1}, {1}, {1}}, [6][6]byte{{91}, {90}, {89}}}
	l5HWALL      = miniset{3, 1, [6][6]byte{{2, 2, 2}}, [6][6]byte{{94, 93, 92}}}
)

func isCrypt(dlvl int) bool { return dlvl >= 21 }

// l5Floor3 is a 3×3 floor miniset whose center becomes t.
func l5Floor3(t byte) miniset {
	f := [6]byte{13, 13, 13}
	return miniset{3, 3, [6][6]byte{f, f, f}, [6][6]byte{{}, {0, t}}}
}

// l5Swap is a run of drlg_l1_crypt_rndset calls on 1×1 minisets: search, replace.
type l5Swap [][2]byte

func l5Range(from, to byte, n int) (s l5Swap) {
	for i := 0; i < n; i++ {
		s = append(s, [2]byte{from + byte(i), to + byte(i)})
	}
	return
}

var (
	l5Statues  = l5Swap{{1, 199}, {1, 201}, {2, 200}, {2, 202}}                // drlg_l1_crypt_pattern1
	l5Cracked  = l5Range(1, 112, 17)                                           // pattern2
	l5Broken   = l5Range(1, 129, 17)                                           // pattern3
	l5Leaking  = l5Range(1, 146, 17)                                           // pattern4
	l5Floor    = l5Swap{{13, 97}, {13, 98}, {13, 99}, {13, 100}}               // pattern7
	l5Pattern5 = l5Swap{{13, 163}, {13, 164}, {13, 165}, {13, 166}}            // after pattern5's six 3×3 sets
	l5Subs1    = l5Swap{{11, 185}, {12, 187}, {11, 186}, {12, 188}, {89, 173}, // pattern6
		{89, 174}, {90, 175}, {90, 176}, {91, 177}, {91, 178}, {92, 179}, {92, 180}, {92, 181}, {92, 182}, {92, 183},
		{92, 184}, {98, 189}, {98, 190}, {97, 191}, {15, 192}, {99, 193}, {99, 194}, {100, 195}, {101, 196},
		{101, 197}, {101, 198}}
)

// l5LavaShadows is drlg_l1_crypt_lavafloor: tile -> floor replacements to its west, north-west and north.
var l5LavaShadows = map[byte][3]byte{
	5: {203, 204, 205}, 7: {206, 207}, 8: {203, 204}, 9: {206, 207, 205}, 10: {0, 0, 205}, 11: {203, 204},
	12: {0, 0, 205}, 14: {203, 204}, 15: {206, 207}, 17: {206, 207}, 95: {203, 204}, 96: {0, 0, 208},
	116: {203, 204, 205}, 118: {206, 207}, 119: {203, 204}, 120: {206, 207, 205}, 121: {0, 0, 205}, 122: {211, 212},
	123: {0, 0, 205}, 125: {203, 204}, 126: {206, 207}, 128: {206, 207}, 133: {203, 204, 205}, 135: {206, 207},
	136: {203, 204}, 137: {213, 214, 205}, 138: {0, 0, 205}, 139: {215, 216}, 140: {0, 0, 217}, 142: {203, 204},
	143: {213, 214}, 145: {213, 214}, 150: {203, 204, 217}, 152: {206, 207}, 153: {203, 204}, 154: {206, 207, 205},
	155: {0, 0, 205}, 156: {203, 204}, 157: {0, 0, 217}, 159: {203, 204}, 160: {206, 207}, 162: {209, 210},
	167: {209, 210}, 185: {203, 204}, 186: {203, 204}, 187: {0, 0, 208}, 192: {209, 210},
}

func (g *gen) l5LavaFloor() {
	for j := 1; j < 40; j++ {
		for i := 1; i < 40; i++ {
			s, ok := l5LavaShadows[g.d(i, j)]
			if !ok {
				continue
			}
			for n, p := range [3]point{{i - 1, j}, {i - 1, j - 1}, {i, j - 1}} {
				if s[n] != 0 && g.d(p.x, p.y) == t1Floor {
					g.sd(p.x, p.y, s[n])
				}
			}
		}
	}
}

// l5RndSet ports drlg_l1_crypt_rndset.
func (g *gen) l5RndSet(m *miniset, rndper int) {
	in := func(x, y int) bool { return x >= 0 && y >= 0 && x < 40 && y < 40 }
	for sy := 0; sy < 40-m.h; sy++ {
		for sx := 0; sx < 40-m.w; sx++ {
			if !m.matches(g, sx, sy, false) {
				continue
			}
			if r := m.replace[0][0]; r >= 84 && r <= 100 {
				// D1's check copied from the Caves compares the west neighbour against 100 every time
				blocked := false
				for _, p := range [4]point{{sx - 1, sy}, {sx + 1, sy}, {sx, sy + 1}, {sx, sy - 1}} {
					if in(p.x, p.y) && in(sx-1, sy) && g.d(p.x, p.y) >= 84 && g.d(sx-1, sy) <= 100 {
						blocked = true
					}
				}
				if blocked {
					continue
				}
			}
			if g.rnd(100) < rndper {
				m.place(g, sx, sy, false)
			}
		}
	}
}

func (g *gen) l5Swaps(s l5Swap, rndper int) {
	for _, p := range s {
		g.l5RndSet(&miniset{w: 1, h: 1, search: [6][6]byte{{p[0]}}, replace: [6][6]byte{{p[1]}}}, rndper)
	}
}

func (g *gen) l5Pattern5(rndper int) {
	for t := byte(167); t <= 172; t++ {
		m := l5Floor3(t)
		g.l5RndSet(&m, rndper)
	}
	g.l5Swaps(l5Pattern5, rndper)
}

// l5Substitution is DRLG_L5's decoration pass for the Crypt, in place of DRLG_L5Subs, shadows, lamps and floor.
func (g *gen) l5Substitution() {
	g.l5Swaps(l5Statues, 10)
	g.l5Swaps(l5Swap{{11, 95}}, 95)
	g.l5Swaps(l5Swap{{12, 96}}, 95)
	g.l5RndSet(&l5VWALL, 100)
	g.l5RndSet(&l5HWALL, 100)
	lava := l5Floor3(101)
	g.l5RndSet(&lava, 60)
	g.l5LavaFloor()
	if g.currlevel == 21 {
		g.l5Swaps(l5Cracked, 30)
		g.l5Swaps(l5Broken, 15)
		g.l5Swaps(l5Leaking, 5)
		g.l5LavaFloor()
		g.l5Swaps(l5Floor, 10)
		g.l5Swaps(l5Subs1, 5)
		g.l5Pattern5(20)
		return
	}
	r := map[int][3]int{22: {10, 20, 10}, 23: {15, 30, 15}, 24: {20, 30, 20}}[g.currlevel] // pattern6, 5, 4
	g.l5Swaps(l5Floor, 10)
	g.l5Swaps(l5Subs1, r[0])
	g.l5Pattern5(r[1])
	g.l5Swaps(l5Cracked, 30)
	g.l5Swaps(l5Broken, 20)
	g.l5Swaps(l5Leaking, r[2])
	g.l5LavaFloor()
}

// l5SetRoom ports drlg_l1_set_crypt_room and drlg_l1_set_corner_room: Na-Krul's room on 24, the Cornerstone of
// the World on 21, in a chamber picked the way the Cathedral picks its set piece's.
func (g *gen) l5SetRoom() {
	var name string
	switch g.currlevel {
	case 24:
		name = "l5/uberroom"
	case 21:
		name = "l5/cornerstone"
	default:
		return
	}
	d := g.loadDun(name)
	x, y := g.l1SelectChamber()
	g.placeDunTiles(d, x, y, t1Floor)
	g.setPiece = rect{x, y, d.w(), d.h()}
}

func (g *gen) l5FixDirtTiles() {
	for j := 0; j < 39; j++ {
		for i := 0; i < 39; i++ {
			switch g.d(i, j) {
			case t1DirtVwall:
				g.sd(i, j, 83)
			case t1HDirtCorner:
				g.sd(i, j, 85)
			case t1DirtHwallEnd:
				g.sd(i, j, 87)
			case t1DirtVwallEnd:
				g.sd(i, j, 88)
			case t1DirtHwall:
				g.sd(i, j, 82)
			}
		}
	}
}

func (g *gen) l5PlaceStairs(entry Entry) bool {
	ok := true
	up := &l5STAIRSUP
	if g.currlevel == 21 {
		up = &l5STAIRSTOWN
	}
	if x, y, found := g.placeMiniSet(up, 1600, true); !found {
		ok = false
	} else if entry == EntryMain || entry == EntryTWarpDn {
		g.view = point{2*x + 19, 2*y + 21}
	}
	if g.currlevel != 24 {
		if x, y, found := g.placeMiniSet(&l5STAIRSDOWN, 1600, true); !found {
			ok = false
		} else if entry == EntryPrev {
			g.view = point{2*x + 19, 2*y + 23}
		}
	}
	return ok
}
