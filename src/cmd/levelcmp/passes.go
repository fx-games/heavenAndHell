package main

import (
	"math/rand"

	"rpg/src/dungen"
)

// growRooms widens each room (to a side between maxSide/2 and maxSide, rolled per room) one row or column at a
// time, in turns, while the strip it takes holds no other room's floor and the strip beyond (its new wall) holds no room floor and at most two corridor cells: rooms end
// up one wall apart, and the corridors that crossed the gap between them become openings in that wall.
func growRooms(m *mega, dm *dungen.DungeonMap, maxSide int, rnd *rand.Rand) []rect {
	var rs []rect
	for _, r := range dm.AllRooms() {
		b := rect{m.w, m.h, -1, -1, maxSide/2 + rnd.Intn(maxSide-maxSide/2+1)}
		for _, p := range r.GetAbsoluteFloorTiles() {
			b.x0, b.y0, b.x1, b.y1 = min(b.x0, p.X), min(b.y0, p.Y), max(b.x1, p.X), max(b.y1, p.Y)
		}
		rs = append(rs, b)
	}
	// strip counts room and corridor cells on the line at offset d beyond side s of r, one longer at each end.
	strip := func(r rect, s, d int) (rooms, corrs int, inside bool) {
		inside = true
		cellAt := func(x, y int) {
			if x < 1 || y < 1 || x >= m.w-1 || y >= m.h-1 {
				inside = false
				return
			}
			switch m.at(x, y) {
			case mRoom, mDoor:
				rooms++
			case mCorr:
				corrs++
			}
		}
		switch s {
		case 0: // north
			for x := r.x0 - 1; x <= r.x1+1; x++ {
				cellAt(x, r.y0-d)
			}
		case 1: // south
			for x := r.x0 - 1; x <= r.x1+1; x++ {
				cellAt(x, r.y1+d)
			}
		case 2: // west
			for y := r.y0 - 1; y <= r.y1+1; y++ {
				cellAt(r.x0-d, y)
			}
		case 3: // east
			for y := r.y0 - 1; y <= r.y1+1; y++ {
				cellAt(r.x1+d, y)
			}
		}
		return
	}
	for grew := true; grew; {
		grew = false
		for i := range rs {
			for s := 0; s < 4; s++ {
				r := rs[i]
				if (s < 2 && r.y1-r.y0+2 > r.cap) || (s >= 2 && r.x1-r.x0+2 > r.cap) {
					continue
				}
				// the strip taken may hold only this room's own corners' neighbours: check the side's span
				roomsIn, _, ok1 := strip(r, s, 1)
				roomsBeyond, corrsBeyond, ok2 := strip(r, s, 2)
				if !ok1 || !ok2 || roomsIn > 0 || roomsBeyond > 0 || corrsBeyond > 2 {
					continue
				}
				switch s {
				case 0:
					r.y0--
				case 1:
					r.y1++
				case 2:
					r.x0--
				case 3:
					r.x1++
				}
				for y := r.y0; y <= r.y1; y++ {
					for x := r.x0; x <= r.x1; x++ {
						m.set(x, y, mRoom)
					}
				}
				rs[i], grew = r, true
			}
		}
	}
	return rs
}

// halls turns, with chance p, a grown room at least 6 wide and 8 long into a pillared hall: an arcade one
// megatile in from each long wall, as D1's l1GenerateHall draws its arch walls.
func halls(m *mega, rs []rect, p float64, rnd *rand.Rand) {
	for _, r := range rs {
		w, h := r.x1-r.x0+1, r.y1-r.y0+1
		if min(w, h) < 6 || max(w, h) < 8 || rnd.Float64() >= p {
			continue
		}
		for y := r.y0; y <= r.y1; y++ {
			for x := r.x0; x <= r.x1; x++ {
				if w <= h && (x == r.x0+1 || x == r.x1-1) || w > h && (y == r.y0+1 || y == r.y1-1) {
					m.set(x, y, mPost)
				}
			}
		}
	}
}

// spine stamps D1's first cathedral room (l1FirstRoom, l1FillChambers, l1GenerateChamber) at D1's coordinates:
// a hall six megatiles wide across the level, arcades on its second and fifth rows, and up to three 10x10
// chambers along it, each with four pillars and arcades where the hall runs into it. Rooms it lands on lose that
// part; connect joins what is left of them.
func spine(m *mega, rnd *rand.Rand) {
	vertical := rnd.Intn(2) == 0
	ch := [3]bool{rnd.Intn(2) != 0, rnd.Intn(2) != 0, rnd.Intn(2) != 0}
	if !ch[0] || !ch[2] {
		ch[1] = true
	}
	set := func(x, y, k int) {
		if vertical {
			x, y = y, x
		}
		m.set(x, y, k)
	}
	x0, x1 := 1, 38
	if !ch[0] {
		x0 += 17
	}
	if !ch[2] {
		x1 -= 16
	}
	for x := x0; x <= x1; x++ {
		for y := 16; y <= 23; y++ {
			k := mRoom
			switch y {
			case 16, 23:
				k = mWall
			case 18, 21:
				k = mPost
			}
			set(x, y, k)
		}
	}
	for i, cx := range [3]int{0, 14, 28} {
		if !ch[i] {
			continue
		}
		const cy = 14
		for y := cy; y <= cy+11; y++ {
			for x := cx; x <= cx+11; x++ {
				k := mRoom
				if x == cx || x == cx+11 || y == cy || y == cy+11 {
					k = mWall
					if hall := x == cx && x0 < cx || x == cx+11 && x1 > cx+11; hall && y >= cy+2 && y <= cy+9 {
						k = mPost // the arcade the hall runs in through, open across its middle aisle
						if y == cy+5 || y == cy+6 {
							k = mRoom
						}
					}
				}
				set(x, y, k)
			}
		}
		for _, p := range [4][2]int{{4, 4}, {7, 4}, {4, 7}, {7, 7}} {
			set(cx+p[0], cy+p[1], mPost)
		}
	}
}

// mirror doubles a quadrant into a 4-way symmetric level and opens the seams: the quadrant's edge walls meet
// back to back, so a few floor cells next to each seam are carried through it (connect makes sure).
func mirror(q *mega, rnd *rand.Rand) *mega {
	m := newMega(2*q.w, 2*q.h)
	for y := 0; y < q.h; y++ {
		for x := 0; x < q.w; x++ {
			k := q.at(x, y)
			m.set(x, y, k)
			m.set(m.w-1-x, y, k)
			m.set(x, m.h-1-y, k)
			m.set(m.w-1-x, m.h-1-y, k)
		}
	}
	var rows, cols []int // where floor meets the right and bottom edges of the quadrant
	for y := 0; y < q.h; y++ {
		if open(q.at(q.w-2, y)) {
			rows = append(rows, y)
		}
	}
	for x := 0; x < q.w; x++ {
		if open(q.at(x, q.h-2)) {
			cols = append(cols, x)
		}
	}
	for _, lines := range [][]int{rows, cols} {
		rnd.Shuffle(len(lines), func(i, j int) { lines[i], lines[j] = lines[j], lines[i] })
	}
	for i := 0; i < min(2, len(rows)); i++ {
		for _, y := range []int{rows[i], m.h - 1 - rows[i]} {
			m.set(q.w-1, y, mRoom)
			m.set(q.w, y, mRoom)
		}
	}
	for i := 0; i < min(2, len(cols)); i++ {
		for _, x := range []int{cols[i], m.w - 1 - cols[i]} {
			m.set(x, q.h-1, mRoom)
			m.set(x, q.h, mRoom)
		}
	}
	return m
}

// arches turns, with chance p, each straight run (3 or more) of wall with room floor on both sides into an
// arcade, D1's arch walls between cathedral rooms.
func arches(m *mega, p float64, rnd *rand.Rand) {
	if p <= 0 {
		return
	}
	for _, vertical := range []bool{false, true} {
		at := func(a, b int) int { // a runs along the wall, b across it
			if vertical {
				return m.at(b, a)
			}
			return m.at(a, b)
		}
		along, across := m.w, m.h
		if vertical {
			along, across = m.h, m.w
		}
		for b := 1; b < across-1; b++ {
			for a := 0; a < along; {
				start := a
				for a < along && at(a, b) == mWall && at(a, b-1) == mRoom && at(a, b+1) == mRoom {
					a++
				}
				if a-start >= 3 && rnd.Float64() < p {
					for i := start; i < a; i++ {
						if vertical {
							m.set(b, i, mPost)
						} else {
							m.set(i, b, mPost)
						}
					}
				}
				if a == start {
					a++
				}
			}
		}
	}
}

// loops opens, with chance p, each wall megatile between floor on two opposite sides (walls on the other two)
// whose sides are more than dist megatiles apart on foot, or not joined at all: accretion's loop_distance and
// loop_chance, so the way back is not always the way in. Openings become doorways; addDoors may shut them.
func loops(m *mega, p float64, dist int, rnd *rand.Rand) {
	if p <= 0 {
		return
	}
	for _, i := range rnd.Perm(len(m.t)) {
		x, y := i%m.w, i/m.w
		if m.t[i] != mWall {
			continue
		}
		var a, b [2]int
		switch {
		case walkable(m.at(x-1, y)) && walkable(m.at(x+1, y)) && m.at(x, y-1) == mWall && m.at(x, y+1) == mWall:
			a, b = [2]int{x - 1, y}, [2]int{x + 1, y}
		case walkable(m.at(x, y-1)) && walkable(m.at(x, y+1)) && m.at(x-1, y) == mWall && m.at(x+1, y) == mWall:
			a, b = [2]int{x, y - 1}, [2]int{x, y + 1}
		default:
			continue
		}
		if rnd.Float64() >= p {
			continue
		}
		if d := walk(m, a, b); d < 0 || d > dist {
			m.t[i] = mCorr
		}
	}
}

// walk is how many megatiles apart a and b are on foot, -1 if they are not joined.
func walk(m *mega, a, b [2]int) int {
	dist := map[[2]int]int{a: 0}
	for q := [][2]int{a}; len(q) > 0; q = q[1:] {
		c := q[0]
		if c == b {
			return dist[c]
		}
		for _, d := range dirs {
			n := [2]int{c[0] + d[0], c[1] + d[1]}
			if _, seen := dist[n]; !seen && walkable(m.at(n[0], n[1])) {
				dist[n] = dist[c] + 1
				q = append(q, n)
			}
		}
	}
	return -1
}

var dirs = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// addLava turns the lakes and rivers of a noise map into lava, rock and floor alike, each lake that reaches the
// cave ringed with floor so the cave runs round it. Where lava cuts the level apart, connect lays a ford.
func addLava(m *mega, cfg *dungen.NoiseConfig, rnd *rand.Rand) error {
	nm, err := cfg.Build(rnd, nil)
	if err != nil {
		return err
	}
	lake := func(x, y int) bool {
		return x >= 1 && y >= 1 && x < m.w-1 && y < m.h-1 && nm.GetTile(x, y) == dungen.Lake
	}
	seen := make([]bool, len(m.t))
	for i := range m.t {
		if seen[i] || !lake(i%m.w, i/m.w) {
			continue
		}
		var pool []int
		seen[i] = true
		for q := []int{i}; len(q) > 0; q = q[1:] {
			pool = append(pool, q[0])
			for _, d := range dirs {
				nx, ny := q[0]%m.w+d[0], q[0]/m.w+d[1]
				if lake(nx, ny) && !seen[nx+ny*m.w] {
					seen[nx+ny*m.w] = true
					q = append(q, nx+ny*m.w)
				}
			}
		}
		touches := false
		for _, c := range pool {
			touches = touches || open(m.t[c]) || open(m.at(c%m.w+1, c/m.w)) || open(m.at(c%m.w-1, c/m.w)) ||
				open(m.at(c%m.w, c/m.w+1)) || open(m.at(c%m.w, c/m.w-1))
		}
		if !touches {
			continue
		}
		for _, c := range pool {
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if x, y := c%m.w+dx, c/m.w+dy; x >= 1 && y >= 1 && x < m.w-1 && y < m.h-1 && m.at(x, y) == mWall {
						m.set(x, y, mRoom) // the rim, or lava below
					}
				}
			}
		}
		for _, c := range pool {
			m.t[c] = mLava
		}
	}
	return nil
}

// posts puts a pillar, with chance p, on room floor with only room floor or walls round it (so pillars never
// touch each other, a doorway or a corridor, and stand a cell off any wall): D1's floor-tile variants with a
// pillar or a stub in them (l1Substitution, l2Substitution, l4Substitution's 95-97).
func posts(m *mega, p float64, rnd *rand.Rand) {
	if p <= 0 {
		return
	}
	for y := 0; y < m.h; y++ {
		for x := 0; x < m.w; x++ {
			ok := m.at(x, y) == mRoom
			for dy := -1; dy <= 1 && ok; dy++ {
				for dx := -1; dx <= 1; dx++ {
					k := m.at(x+dx, y+dy)
					ok = ok && (k == mRoom || k == mWall)
				}
			}
			if ok && rnd.Float64() < p {
				m.set(x, y, mPost)
			}
		}
	}
}

// connect makes every walkable megatile reachable from every other: from the part holding the first one, the
// cheapest way to the next part (walking free, a ford across lava 1, a hole through a wall 3) is opened, until
// nothing is left out. It is the guarantee behind every pass above.
func connect(m *mega) {
	for {
		start := -1
		for i, k := range m.t {
			if walkable(k) {
				start = i
				break
			}
		}
		if start < 0 {
			return
		}
		cost := func(k int) int {
			switch {
			case walkable(k):
				return 0
			case k == mLava:
				return 1
			}
			return 3
		}
		// Dijkstra with buckets, from the start's part; the first walkable cell reached at a cost is another part
		dist, from := make([]int, len(m.t)), make([]int, len(m.t))
		for i := range dist {
			dist[i], from[i] = -1, -1
		}
		dist[start] = 0
		buckets := [][]int{{start}}
		end := -1
		for c := 0; c < len(buckets) && end < 0; c++ {
			for j := 0; j < len(buckets[c]) && end < 0; j++ {
				i := buckets[c][j]
				if dist[i] != c {
					continue
				}
				if c > 0 && walkable(m.t[i]) {
					end = i
					break
				}
				for _, d := range dirs {
					x, y := i%m.w+d[0], i/m.w+d[1]
					if x < 0 || y < 0 || x >= m.w || y >= m.h {
						continue
					}
					n := x + y*m.w
					if nd := c + cost(m.t[n]); dist[n] < 0 || nd < dist[n] {
						dist[n], from[n] = nd, i
						for len(buckets) <= nd {
							buckets = append(buckets, nil)
						}
						buckets[nd] = append(buckets[nd], n)
					}
				}
			}
		}
		if end < 0 {
			return
		}
		for i := from[end]; i >= 0 && !walkable(m.t[i]); i = from[i] {
			if m.t[i] == mLava {
				m.t[i] = mRoom // a ford
			} else {
				m.t[i] = mCorr
			}
		}
	}
}

// addDoors makes each doorway a door with chance p and an open gap otherwise: the layout's own doors, and each
// corridor cell between walls on two opposite sides and floor on the other two, one of them a room's.
func addDoors(m *mega, p float64, rnd *rand.Rand) {
	for y := 0; y < m.h; y++ {
		for x := 0; x < m.w; x++ {
			switch m.at(x, y) {
			case mDoor:
				if rnd.Float64() >= p {
					m.set(x, y, mCorr)
				}
				continue
			case mCorr:
			default:
				continue
			}
			n, s, w, e := m.at(x, y-1), m.at(x, y+1), m.at(x-1, y), m.at(x+1, y)
			ns := n == mWall && s == mWall && open(w) && open(e) && (w == mRoom || e == mRoom)
			we := w == mWall && e == mWall && open(n) && open(s) && (n == mRoom || s == mRoom)
			if (ns || we) && rnd.Float64() < p {
				m.set(x, y, mDoor)
			}
		}
	}
}
