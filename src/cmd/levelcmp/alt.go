package main

import (
	"math/rand"
	"strings"

	"rpg/src/dungen"
	"rpg/src/recfile"
)

// altRecords are the stand-ins as levels.rec records: one layout generator plus the passes (dungen.Passes) that
// give it a D1 level type's look. Sizes are megatiles, centred on D1's 40x40 and drawn two by two cells.
var altRecords = [5]string{
	1: `generator: bsp
width: 40
height: 40
min_leaf: 9
min_room: 3
grow: 8
hall_chance: 0.15
spine: true
arch_chance: 0.12
loop_chance: 0.6
loop_distance: 8
posts: 0.02
door_chance: 0.55`,
	2: `generator: megadungeon
width: 35
height: 35
room_tries: 25
min_room_size: 3
room_size_spread: 8
straight_passage_chance: 0.7
imperfect_connect_chance: 0.6
room_ratio: 0.6
door_distance: 4
min_rooms: 8
inner_room_chance: 0.45
posts: 0.03
door_chance: 0.5`,
	3: `generator: drunkard
width: 36
height: 36
cover: 0.38
walk_length: 1
dla: true
lava_water: 0.26
lava_rivers: 2
door_chance: 0`,
	4: `generator: bsp
width: 19
height: 19
min_leaf: 6
min_room: 3
grow: 14
mirror: true
posts: 0.16
door_chance: 0`,
}

func buildAlt(ltype int, seed int64) (*grid, error) {
	var r recfile.Record
	for _, l := range strings.Split(altRecords[ltype]+"\ncanvas: 40\nscale: 2", "\n") {
		k, v, _ := strings.Cut(l, ": ")
		r = append(r, recfile.Field{Name: k, Value: v})
	}
	cfg, err := dungen.WithPasses(dungen.Layouts[r[0].Value]).Parse(r)
	if err != nil {
		return nil, err
	}
	dm, err := cfg.(dungen.Layout).Build(rand.New(rand.NewSource(seed)), nil)
	if err != nil {
		return nil, err
	}
	w, h := dm.GetSize()
	g := newGrid(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch dm.GetTile(x, y) {
			case dungen.Room, dungen.Corridor, dungen.Shallows:
				g.set(x, y, cFloor)
			case dungen.Door:
				g.set(x, y, cDoor)
			case dungen.Lava, dungen.Lake:
				g.set(x, y, cWater)
			}
		}
	}
	return g, nil
}
