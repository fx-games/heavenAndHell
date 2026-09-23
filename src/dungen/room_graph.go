package dungen

import (
	"cmp"
	"fmt"
	"io/fs"
	"math/rand"
	"slices"

	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/recfile"
)

func init() { Layouts["room_graph"] = roomGraph{} }

// RoomGraphConfig: rooms scattered at random, joined along the Gabriel graph of their centres (a subgraph of the
// Delaunay triangulation that still contains its minimum spanning tree): the spanning tree keeps everything
// reachable, and a share of the remaining edges comes back as loops, so the level is a network rather than a tree.
type RoomGraphConfig struct {
	Width, Height    int
	Rooms            int
	MinRoom, MaxRoom int
	Loops            float64 // share of the non-tree edges dug as well
}

type roomGraph struct{}

func (roomGraph) Parse(r recfile.Record) (dungeon.Config, error) {
	c := RoomGraphConfig{Width: 80, Height: 50, Rooms: 14, MinRoom: 4, MaxRoom: 10, Loops: 0.15}
	if err := parseKnobs(r, []intKnob{{"width", &c.Width}, {"height", &c.Height}, {"rooms", &c.Rooms},
		{"min_room", &c.MinRoom}, {"max_room", &c.MaxRoom}}, []floatKnob{{"loops", &c.Loops}}); err != nil {
		return nil, err
	}
	if c.Rooms < 2 || c.MinRoom < 2 || c.MaxRoom < c.MinRoom {
		return nil, fmt.Errorf("levels.rec: room_graph needs rooms >= 2, min_room >= 2, max_room >= min_room")
	}
	if err := checkSize("room_graph", c.Width, c.Height, c.MaxRoom+2); err != nil {
		return nil, err
	}
	return c, checkFraction("room_graph", "loops", c.Loops)
}

func (c RoomGraphConfig) Build(rnd *rand.Rand, _ fs.FS) (*DungeonMap, error) {
	m := NewDungeonMap(c.Width, c.Height)
	var rects []geometry.Rect
	for try := 0; try < c.Rooms*50 && len(rects) < c.Rooms; try++ {
		w, h := c.MinRoom+rnd.Intn(c.MaxRoom-c.MinRoom+1), c.MinRoom+rnd.Intn(c.MaxRoom-c.MinRoom+1)
		x, y := 1+rnd.Intn(c.Width-w-1), 1+rnd.Intn(c.Height-h-1)
		r := geometry.NewRect(x, y, x+w, y+h)
		gap := geometry.NewRect(x-2, y-2, x+w+2, y+h+2) // a wall and a corridor's width between rooms
		if !slices.ContainsFunc(rects, func(o geometry.Rect) bool { return o.Overlaps(gap) }) {
			rects = append(rects, r)
		}
	}
	if len(rects) < 2 {
		return nil, fmt.Errorf("dungen: room_graph fit %d rooms", len(rects))
	}
	rooms := make([]*DungeonRoom, len(rects))
	for i, r := range rects {
		rooms[i] = addRectRoom(m, rnd, r)
	}
	// ponytail: O(n^3) Gabriel test; fine for dozens of rooms, Bowyer-Watson if a level wants hundreds
	type edge struct{ a, b, d int }
	var edges []edge
	for a := range rooms {
		for b := a + 1; b < len(rooms); b++ {
			pa, pb := center(rooms[a]), center(rooms[b])
			blocked := false
			for k := range rooms {
				pk := center(rooms[k])
				// k lies inside the circle on a-b as diameter when the angle a-k-b is obtuse
				if k != a && k != b && (pa.X-pk.X)*(pb.X-pk.X)+(pa.Y-pk.Y)*(pb.Y-pk.Y) < 0 {
					blocked = true
					break
				}
			}
			if !blocked {
				edges = append(edges, edge{a, b, geometry.DistanceSquared(pa, pb)})
			}
		}
	}
	slices.SortStableFunc(edges, func(x, y edge) int { return cmp.Compare(x.d, y.d) })
	group := make([]int, len(rooms)) // Kruskal's union-find
	for i := range group {
		group[i] = i
	}
	var find func(i int) int
	find = func(i int) int {
		if group[i] != i {
			group[i] = find(group[i])
		}
		return group[i]
	}
	for _, e := range edges {
		tree := find(e.a) != find(e.b)
		if tree {
			group[find(e.a)] = find(e.b)
		}
		if tree || rnd.Float64() < c.Loops {
			carveL(m, rnd, center(rooms[e.a]), center(rooms[e.b]))
		}
	}
	return m, nil
}
