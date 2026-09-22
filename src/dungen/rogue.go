package dungen

import (
	"math/rand"

	"rpg/src/geometry"
)

// RogueGenerator is Rogue 5.4's level layout (rooms.c do_rooms, passages.c do_passages): the map is cut into a
// 3x3 grid of cells, each cell holds one rectangular room or, for a few of them, only a corridor junction (a
// "gone" room), and passages join neighbouring cells - first a random spanning tree, so every cell is reachable,
// then a few extra loops.
type RogueGenerator struct {
	cols, rows    int
	maxGone       int // rooms left out: rnd(maxGone+1), Rogue's rnd(4)
	extraPassages int // loops added after the spanning tree: rnd(extraPassages+1), Rogue's rnd(5)
	doors         bool
	random        *rand.Rand
}

// rogueCell is one of the nine grid cells: a room's floor, or the single spot a gone room's passages meet at.
type rogueCell struct {
	gone  bool
	floor geometry.Rect // Min inclusive, Max exclusive; for a gone room one cell wide
	room  *DungeonRoom
}

func (r *RogueGenerator) Generate() *DungeonMap {
	m := NewDungeonMap(r.cols, r.rows)
	bw, bh := r.cols/3, r.rows/3

	gone := map[int]bool{}
	for left := r.random.Intn(r.maxGone + 1); left > 0; left-- {
		gone[r.random.Intn(9)] = true
	}

	cells := make([]rogueCell, 9)
	for i := range cells {
		top := geometry.Point{X: (i%3)*bw + 1, Y: (i / 3) * bh}
		if gone[i] {
			// A junction somewhere inside the cell, never on the top row (Rogue's "while (r_pos.y == 0)").
			p := geometry.Point{X: top.X + r.random.Intn(bw-2) + 1, Y: top.Y + r.random.Intn(bh-2) + 1}
			cells[i] = rogueCell{gone: true, floor: geometry.NewRect(p.X, p.Y, p.X+1, p.Y+1)}
			m.SetCorridor(p.X, p.Y)
			continue
		}
		// Size and position include the walls, as in Rogue; the floor is what lies inside them.
		// Rogue re-rolls a room on the top row until it leaves row 0 free; rolling below row 0 directly is the
		// same odds without the loop, which never ends when a cell is only as tall as the smallest room.
		size := geometry.Point{X: r.random.Intn(bw-4) + 4, Y: r.random.Intn(bh-4) + 4}
		minY := top.Y
		if minY == 0 {
			minY, size.Y = 1, min(size.Y, bh-1)
		}
		pos := geometry.Point{X: top.X + r.random.Intn(bw-size.X), Y: minY + r.random.Intn(top.Y+bh-size.Y-minY+1)}
		floor := geometry.NewRect(pos.X+1, pos.Y+1, pos.X+size.X-1, pos.Y+size.Y-1)
		room := NewDungeonRoomFromRect(r.random, floor)
		room.SetConnections(map[geometry.Point]RoomConnection{}) // its doors are the passages dug below
		room.SetId(i)
		m.AddRoomAndSetTiles(room)
		cells[i] = rogueCell{floor: floor, room: room}
	}

	r.doPassages(m, cells)
	return m
}

// rogueNeighbours is passages.c's rdes table: which cells border which.
func rogueNeighbours(i int) []int {
	var out []int
	if i%3 > 0 {
		out = append(out, i-1)
	}
	if i%3 < 2 {
		out = append(out, i+1)
	}
	if i >= 3 {
		out = append(out, i-3)
	}
	if i < 6 {
		out = append(out, i+3)
	}
	return out
}

func (r *RogueGenerator) doPassages(m *DungeonMap, cells []rogueCell) {
	connected := map[[2]int]bool{}
	isConnected := func(a, b int) bool { return connected[[2]int{min(a, b), max(a, b)}] }
	join := func(a, b int) {
		connected[[2]int{min(a, b), max(a, b)}] = true
		r.conn(m, cells, min(a, b), max(a, b))
	}

	// The spanning tree: grow from a random cell to a random unjoined neighbour; when the current cell has none,
	// jump to a random cell already in the graph.
	inGraph := map[int]bool{}
	r1 := r.random.Intn(9)
	inGraph[r1] = true
	for len(inGraph) < 9 {
		var free []int
		for _, n := range rogueNeighbours(r1) {
			if !inGraph[n] {
				free = append(free, n)
			}
		}
		if len(free) == 0 {
			for {
				if r1 = r.random.Intn(9); inGraph[r1] {
					break
				}
			}
			continue
		}
		r2 := free[r.random.Intn(len(free))]
		inGraph[r2] = true
		join(r1, r2)
	}

	// The loops: a few more passages between neighbours not yet joined.
	for i := r.random.Intn(r.extraPassages + 1); i > 0; i-- {
		r1 = r.random.Intn(9)
		var free []int
		for _, n := range rogueNeighbours(r1) {
			if !isConnected(r1, n) {
				free = append(free, n)
			}
		}
		if len(free) > 0 {
			join(r1, free[r.random.Intn(len(free))])
		}
	}
}

// conn digs one passage from cell a to cell b, where b is right of a (b == a+1) or below it (b == a+3):
// out through a door in a's facing wall, straight on, one jog sideways at a random point, straight on again,
// and in through a door in b's facing wall. A gone room is its own door.
func (r *RogueGenerator) conn(m *DungeonMap, cells []rogueCell, a, b int) {
	across := b == a+1 // along x; otherwise along y
	dir, side := geometry.Point{Y: 1}, geometry.Point{X: 1}
	if across {
		dir, side = geometry.Point{X: 1}, geometry.Point{Y: 1}
	}
	start := r.doorSpot(m, cells[a], across, true)
	end := r.doorSpot(m, cells[b], across, false)

	along := func(p geometry.Point) int { return p.X*dir.X + p.Y*dir.Y }
	sideways := func(p geometry.Point) int { return p.X*side.X + p.Y*side.Y }
	distance := along(end) - along(start) // cells walked in the main direction
	jog := sideways(end) - sideways(start)
	turnAt := 0 // jog at once: with the two ends one step apart there is no room to walk first
	if distance > 2 {
		turnAt = r.random.Intn(distance-1) + 1
	}
	jogStep := side
	if jog < 0 {
		jogStep = geometry.Point{X: -side.X, Y: -side.Y}
		jog = -jog
	}

	p := start
	for step := 0; step < distance; step++ {
		if step > 0 {
			p = p.Add(dir)
			r.dig(m, p)
		}
		if step == turnAt {
			for ; jog > 0; jog-- {
				p = p.Add(jogStep)
				r.dig(m, p)
			}
		}
	}
	if room := cells[a].room; room != nil {
		room.AddConnectedRoom(start, cells[b].room)
	}
	if room := cells[b].room; room != nil {
		room.AddConnectedRoom(end, cells[a].room)
	}
}

// doorSpot picks where a passage leaves (outward) or enters a cell and opens it: a random spot on the wall facing
// the other cell, or the junction of a gone room.
func (r *RogueGenerator) doorSpot(m *DungeonMap, c rogueCell, across, outward bool) geometry.Point {
	if c.gone {
		return c.floor.Min
	}
	f := c.floor
	var p geometry.Point
	switch {
	case across && outward:
		p = geometry.Point{X: f.Max.X, Y: f.Min.Y + r.random.Intn(f.Max.Y-f.Min.Y)}
	case across:
		p = geometry.Point{X: f.Min.X - 1, Y: f.Min.Y + r.random.Intn(f.Max.Y-f.Min.Y)}
	case outward:
		p = geometry.Point{X: f.Min.X + r.random.Intn(f.Max.X-f.Min.X), Y: f.Max.Y}
	default:
		p = geometry.Point{X: f.Min.X + r.random.Intn(f.Max.X-f.Min.X), Y: f.Min.Y - 1}
	}
	if r.doors {
		m.SetDoor(p.X, p.Y)
	} else {
		m.SetCorridor(p.X, p.Y)
	}
	return p
}

// dig turns a wall into passage and leaves floors and doors it crosses as they are.
func (r *RogueGenerator) dig(m *DungeonMap, p geometry.Point) {
	if m.GetTileAt(p) == Wall {
		m.SetCorridor(p.X, p.Y)
	}
}
