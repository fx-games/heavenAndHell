package dungen

import (
	"math/rand"

	"rpg/src/geometry"
)

var (
	dirs4 = []geometry.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}}
	dirs8 = append([]geometry.Point{{X: 1, Y: 1}, {X: 1, Y: -1}, {X: -1, Y: 1}, {X: -1, Y: -1}}, dirs4...)
)

// addLakes ports Brogue's designLakes/fillLakes (Architect.c) to a grown level: count blobs of shrinking size,
// each tried at up to 10 spots and kept only on plain room floor where the level stays connected, then ringed
// with shallows one can wade through (Brogue's wreath). A room with hand-drawn decoration keeps its floor.
func (m *DungeonMap) addLakes(rnd *rand.Rand, count int) {
	keep := map[geometry.Point]bool{}
	for _, r := range m.rooms {
		if r.HasDecoratedTemplate() {
			for _, p := range r.GetAbsoluteRoomTiles() {
				keep[p] = true
			}
		}
	}
	for k := 0; k < count; k++ {
		w, h := max(4, m.width/3-2*k), max(4, m.height/3-k) // Brogue shrinks its canvas by 2x1 per lake
		blob := caBlob(rnd, w, h, 55)
		for try := 0; try < 10; try++ {
			off := geometry.Point{X: rnd.Intn(m.width - w + 1), Y: rnd.Intn(m.height - h + 1)}
			var cells []geometry.Point
			for _, b := range blob {
				if p := b.Add(off); m.GetTileAt(p) == Room && !keep[p] {
					cells = append(cells, p)
				}
			}
			if len(cells) >= 4 && m.connectedWithout(cells) {
				for _, p := range cells {
					m.tiles[p.X+p.Y*m.width] = Lake
				}
				break
			}
		}
	}
	var shallows []geometry.Point
	for y := 0; y < m.height; y++ {
		for x := 0; x < m.width; x++ {
			p := geometry.Point{X: x, Y: y}
			if m.GetTileAt(p) != Room || keep[p] {
				continue
			}
			for _, d := range dirs8 {
				if n := p.Add(d); m.Contains(n) && m.GetTileAt(n) == Lake {
					shallows = append(shallows, p)
					break
				}
			}
		}
	}
	for _, p := range shallows {
		m.tiles[p.X+p.Y*m.width] = Shallows
	}
}

// connectedWithout reports whether every walkable cell still reaches every other with cells under water.
func (m *DungeonMap) connectedWithout(cells []geometry.Point) bool {
	flooded := map[geometry.Point]bool{}
	for _, c := range cells {
		flooded[c] = true
	}
	var start []geometry.Point
	total := 0
	for y := 0; y < m.height; y++ {
		for x := 0; x < m.width; x++ {
			if p := (geometry.Point{X: x, Y: y}); m.IsWalkable(p) && !flooded[p] {
				total++
				if start == nil {
					start = []geometry.Point{p}
				}
			}
		}
	}
	seen := map[geometry.Point]bool{}
	for q := start; len(q) > 0; q = q[1:] {
		if seen[q[0]] {
			continue
		}
		seen[q[0]] = true
		for _, d := range dirs4 {
			if n := q[0].Add(d); m.IsWalkable(n) && !flooded[n] && !seen[n] {
				q = append(q, n)
			}
		}
	}
	return len(seen) == total
}

// caBlob grows a cellular-automata blob in a w x h canvas (Brogue's createBlobOnGrid: fill% seeded, 55 for lakes, five
// rounds, born with 5+ live neighbours, surviving with 4+) and returns its largest connected part.
func caBlob(rnd *rand.Rand, w, h, fill int) []geometry.Point {
	alive := make([]bool, w*h)
	for i := range alive {
		alive[i] = rnd.Intn(100) < fill
	}
	at := func(g []bool, x, y int) bool { return x >= 0 && y >= 0 && x < w && y < h && g[x+y*w] }
	for round := 0; round < 5; round++ {
		next := make([]bool, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				n := 0
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if (dx != 0 || dy != 0) && at(alive, x+dx, y+dy) {
							n++
						}
					}
				}
				next[x+y*w] = n >= 5 || alive[x+y*w] && n >= 4
			}
		}
		alive = next
	}
	var best []geometry.Point
	seen := make([]bool, w*h)
	for i := range alive {
		if !alive[i] || seen[i] {
			continue
		}
		part := []geometry.Point{{X: i % w, Y: i / w}}
		seen[i] = true
		for j := 0; j < len(part); j++ {
			for _, d := range dirs4 {
				if n := part[j].Add(d); at(alive, n.X, n.Y) && !seen[n.X+n.Y*w] {
					seen[n.X+n.Y*w] = true
					part = append(part, n)
				}
			}
		}
		if len(part) > len(best) {
			best = part
		}
	}
	return best
}

// addRivers runs count rivers out of the level's lakes, D1's lava rivers for water: each leaves a random lake cell
// for the map edge or another lake, stepping toward its goal or, with chance meander, sideways, and turns a
// width x width square at every step into Lake. Rock it crosses turns to water too. Where a river cuts the
// walkable ground apart, ford lays shallows across it.
func (m *DungeonMap) addRivers(rnd *rand.Rand, count, width int, meander float64) {
	var lakes []geometry.Point
	for i, t := range m.tiles {
		if t == Lake {
			lakes = append(lakes, geometry.Point{X: i % m.width, Y: i / m.width})
		}
	}
	if len(lakes) == 0 || count <= 0 {
		return
	}
	for r := 0; r < count; r++ {
		p := lakes[rnd.Intn(len(lakes))]
		goal := [4]geometry.Point{{X: rnd.Intn(m.width)}, {X: rnd.Intn(m.width), Y: m.height - 1},
			{Y: rnd.Intn(m.height)}, {X: m.width - 1, Y: rnd.Intn(m.height)}}[rnd.Intn(4)]
		if rnd.Intn(2) == 0 { // another lake, if one lies far enough off
			if o := lakes[rnd.Intn(len(lakes))]; o.ManhattanDistanceTo(p) > (m.width+m.height)/4 {
				goal = o
			}
		}
		left := false // off the lake it started in: meeting water again ends the river
		for step := 0; p != goal && step < 4*(m.width+m.height); step++ {
			d := geometry.Point{X: sign(goal.X - p.X)}
			if dy := sign(goal.Y - p.Y); dy != 0 && (d.X == 0 || rnd.Intn(abs(goal.X-p.X)+abs(goal.Y-p.Y)) < abs(goal.Y-p.Y)) {
				d = geometry.Point{Y: dy}
			}
			if rnd.Float64() < meander {
				d = geometry.Point{X: d.Y, Y: d.X} // a quarter turn, either way
				if rnd.Intn(2) == 0 {
					d = geometry.Point{X: -d.X, Y: -d.Y}
				}
			}
			if n := p.Add(d); m.Contains(n) {
				p = n
			}
			if left && m.GetTileAt(p) == Lake {
				break
			}
			left = left || m.GetTileAt(p) != Lake
			for y := p.Y - (width-1)/2; y <= p.Y+width/2; y++ {
				for x := p.X - (width-1)/2; x <= p.X+width/2; x++ {
					if m.Contains(geometry.Point{X: x, Y: y}) {
						m.tiles[x+y*m.width] = Lake
					}
				}
			}
		}
	}
	m.ford()
}

func abs(v int) int { return max(v, -v) }

// ford lays shallows across water that cuts the walkable ground apart: from the part the first walkable cell is
// in, the way to the next part that crosses the fewest water cells has those cells turned to shallows, until
// every walkable cell is reachable.
func (m *DungeonMap) ford() {
	for {
		reached := make([]bool, len(m.tiles))
		var frontier []int
		for i := range m.tiles {
			if m.IsWalkable(geometry.Point{X: i % m.width, Y: i / m.width}) {
				frontier = []int{i}
				reached[i] = true
				break
			}
		}
		for j := 0; j < len(frontier); j++ { // the part it is in
			for _, d := range dirs4 {
				if n := (geometry.Point{X: frontier[j] % m.width, Y: frontier[j] / m.width}).Add(d); m.IsWalkable(n) && !reached[n.X+n.Y*m.width] {
					reached[n.X+n.Y*m.width] = true
					frontier = append(frontier, n.X+n.Y*m.width)
				}
			}
		}
		// breadth first across water, one ring of lake cells at a time, until another part turns up
		from := make([]int, len(m.tiles))
		for i := range from {
			from[i] = -1
		}
		end := -1
		for ring := frontier; len(ring) > 0 && end < 0; {
			var next []int
			for j := 0; j < len(ring) && end < 0; j++ {
				for _, d := range dirs4 {
					n := (geometry.Point{X: ring[j] % m.width, Y: ring[j] / m.width}).Add(d)
					if !m.Contains(n) || reached[n.X+n.Y*m.width] {
						continue
					}
					k := n.X + n.Y*m.width
					if m.tiles[k] == Lake {
						reached[k], from[k] = true, ring[j]
						next = append(next, k)
					} else if m.IsWalkable(n) {
						end, from[k] = k, ring[j]
						break
					}
				}
			}
			ring = next
		}
		if end < 0 {
			return
		}
		for k := from[end]; k >= 0 && m.tiles[k] == Lake; k = from[k] {
			m.tiles[k] = Shallows
		}
	}
}
