package d1

// drlg_l3.cpp's HELLFIRE branches: the Nest (dungeon levels 17-20) runs the Caves' generator with its own stairs,
// slime pools and decoration, and none of the Caves' rivers, fences or theme rooms.

func isNest(dlvl int) bool { return dlvl >= 17 && dlvl <= 20 }

func (g *gen) l6PlaceStairs(entry Entry) bool {
	up := &l6UP
	if g.currlevel == 17 {
		up = &l6HOLDWARP
	}
	x, y, ok := g.placeMiniSet(up, 199, false)
	if !ok {
		return false
	}
	if entry == EntryMain || entry == EntryTWarpDn {
		g.view = point{2*x + 17, 2*y + 19}
	}
	if g.currlevel != 20 {
		if x, y, ok = g.placeMiniSet(&l6DOWN, 199, false); !ok {
			return false
		}
		if entry == EntryPrev {
			g.view = point{2*x + 19, 2*y + 17}
		}
	}
	return true
}

func (g *gen) l6PlaceSlimePool() bool {
	n := 0
	for _, s := range [4]struct {
		m   *miniset
		per int
	}{{&l6Hive41, 30}, {&l6Hive42, 40}, {&l6Hive39, 50}, {&l6Hive40, 60}} {
		if g.l3PlaceMiniSetRandom(s.m, s.per) {
			n++
		}
	}
	return n >= 3
}

func (g *gen) l6Decorate() {
	sets := func(list ...any) {
		for i := 0; i < len(list); i += 2 {
			g.l3PlaceMiniSetRandom(list[i].(*miniset), list[i+1].(int))
		}
	}
	swaps := func(per int, pairs ...byte) {
		for i := 0; i < len(pairs); i += 2 {
			g.l3PlaceMiniSetRandom1x1(pairs[i], pairs[i+1], per)
		}
	}
	sets(&l6ISLE1, 70, &l6ISLE2, 70, &l6ISLE3, 30, &l6ISLE4, 30, &l6ISLE1, 100, &l6ISLE2, 100, &l6ISLE5, 90)
	swaps(20, 8, 25, 8, 26, 8, 27, 8, 28)
	sets(&l6Hive29, 10, &l6Hive30, 15, &l6Hive31, 20, &l6Hive32, 25, &l6Hive33, 30, &l6Hive34, 35, &l6Hive35, 40,
		&l6Hive36, 45, &l6Hive37, 50, &l6Hive38, 55, &l6Hive38, 10, &l6Hive37, 15, &l6Hive36, 20, &l6Hive35, 25,
		&l6Hive34, 30, &l6Hive33, 35, &l6Hive32, 40, &l6Hive31, 45, &l6Hive30, 50, &l6Hive29, 55,
		&l6Hive9, 40, &l6Hive10, 45)
	swaps(25, 7, 29, 7, 30, 7, 31, 7, 32, 9, 33, 9, 34, 9, 35, 9, 36, 9, 37, 10, 39, 10, 40, 10, 41, 10, 42, 10, 43,
		9, 45, 9, 46, 10, 47, 10, 48, 11, 38, 11, 44, 11, 49, 11, 50)
}
