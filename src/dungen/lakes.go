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
