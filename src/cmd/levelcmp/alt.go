package main

import (
	"fmt"
	"math/rand"

	"rpg/src/dungen"
)

// altType is how -alt builds one D1 level type out of package dungen's layout generators. Everything is laid
// out at megatile scale (one cell per D1 megatile, 40x40) and drawn at map-cell scale by toCells, as D1 does.
// The passes run in the order the fields are listed; every level ends connected (connect).
type altType struct {
	layout       dungen.Layout       // carves the level (or, with mirror, one quadrant of it)
	grow         int                 // grow rectangular rooms up to this side until single walls part them; 0: off
	hallChance   float64             // a grown room at least 6 wide and 8 long becomes a pillared hall (two arcades)
	spine        bool                // D1's first cathedral room: up to three pillared chambers on a long pillared hall
	mirror       bool                // mirror the layout left-right and top-bottom (D1 L4 builds a quadrant and mirrors it)
	archChance   float64             // a straight wall between two rooms becomes an arcade (`#.#.#`)
	loopChance   float64             // chance each wall between two spots farther than loopDistance apart on foot opens
	loopDistance int                 // in megatiles walked
	lava         *dungen.NoiseConfig // its lakes and rivers become lava, ringed with floor; fords keep the level whole
	posts        float64             // chance a room megatile with only room or wall round it gets a pillar (D1's floor variants)
	doors        float64             // chance each doorway (a 1-wide gap in a wall, or a door the layout made) is a door
}

// altTypes is the table to tweak: one stand-in per D1 level type. Sizes are megatiles; the layouts are centred
// in D1's 40x40. See the report printed by `go test -v -run Metrics ./src/cmd/levelcmp` for how they compare.
var altTypes = [5]altType{
	1: { // cathedral: big rectangles packed wall to wall, a pillared spine hall with chambers, arcades, loops
		// BSP gives the packing (one room per leaf, a corridor across every split); growing the rooms until one
		// wall parts them turns BSP's gaps into D1's shared walls, and the corridors into arches through them.
		// The spine is D1's l1FirstRoom/l1FillChambers stamped over it, at D1's own coordinates.
		layout: dungen.BSPConfig{Width: 40, Height: 40, MinLeaf: 9, MinRoom: 3},
		grow:   8, hallChance: 0.15, spine: true, archChance: 0.12,
		loopChance: 0.6, loopDistance: 8, posts: 0.02, doors: 0.55,
	},
	2: { // catacombs: square rooms, often nested, in a web of 3-wide corridors, doors where they meet
		// Rooms-and-mazes with the dead ends filled in leaves exactly that; inner_room_chance nests rooms.
		layout: dungen.MegaDungeonConfig{Width: 35, Height: 35, RoomTries: 25, MinRoomSize: 3, RoomSizeSpread: 8,
			StraightPassageChance: 0.7, ImperfectConnectChance: 0.6, RoomRatioInterval: 0.6, DoorDistance: 4,
			RemoveDeadEnds: true, MinRooms: 8, Doors: true, InnerRoomChance: 0.45},
		posts: 0.03, doors: 0.5,
	},
	3: { // caves: a web of 2-3 wide passages round blocky pillars, a few open halls, lava pools and rivers
		// Diffusion-limited aggregation grows that branching, looping web; CA caves (cave) and noise came out
		// too smooth and open at this scale. Lava is a noise map's lakes and rivers.
		layout: dungen.DrunkardConfig{Width: 36, Height: 36, Cover: 0.38, WalkLength: 1, DLA: true},
		lava: &dungen.NoiseConfig{Width: 36, Height: 36, Scale: 6, Rock: 0.99, Water: 0.26, MinFloor: 0,
			Rivers: 2, RiverWidth: 1, RiverMeander: 0.35},
		posts: 0, // DLA's one-megatile rock islands already draw as single pillars
	},
	4: { // hell: one quadrant mirrored both ways; big blocks, wide halls, pillars dotted about, no doors
		// The cathedral's grown BSP on a quarter of the map, mirrored, with two openings carried through each seam.
		layout: dungen.BSPConfig{Width: 19, Height: 19, MinLeaf: 6, MinRoom: 3},
		grow:   14, mirror: true, posts: 0.16,
	},
}

// Megatile kinds.
const (
	mWall = iota
	mRoom
	mCorr
	mDoor
	mLava
	mPost // floor with a pillar in its top-left cell: in a row, an arcade
)

type mega struct {
	w, h int
	t    []int
}

func newMega(w, h int) *mega { return &mega{w: w, h: h, t: make([]int, w*h)} }

func (m *mega) at(x, y int) int {
	if x < 0 || y < 0 || x >= m.w || y >= m.h {
		return mWall
	}
	return m.t[x+y*m.w]
}

func (m *mega) set(x, y, k int) {
	if x >= 0 && y >= 0 && x < m.w && y < m.h {
		m.t[x+y*m.w] = k
	}
}

// open is ground one walks on without a door; walkable adds doors.
func open(k int) bool     { return k == mRoom || k == mCorr || k == mPost }
func walkable(k int) bool { return open(k) || k == mDoor }

type rect struct{ x0, y0, x1, y1, cap int } // inclusive floor bounds, and the side it may grow to

func buildAlt(ltype int, seed int64) (*grid, error) {
	p := altTypes[ltype]
	rnd := rand.New(rand.NewSource(seed))
	dm, err := p.layout.Build(rnd, nil)
	if err != nil {
		return nil, err
	}
	m := fromDungeon(dm)
	if p.grow > 0 {
		halls(m, growRooms(m, dm, p.grow, rnd), p.hallChance, rnd)
	}
	if p.spine {
		spine(m, rnd)
	}
	if p.mirror {
		m = mirror(m, rnd)
	}
	arches(m, p.archChance, rnd)
	loops(m, p.loopChance, p.loopDistance, rnd)
	if p.lava != nil {
		if err := addLava(m, p.lava, rnd); err != nil {
			return nil, err
		}
	}
	posts(m, p.posts, rnd)
	full := newMega(dungen.LevelSize/2, dungen.LevelSize/2) // centred in D1's 40x40
	ox, oy := (full.w-m.w)/2, (full.h-m.h)/2
	if ox < 0 || oy < 0 {
		return nil, fmt.Errorf("layout %dx%d is larger than %dx%d megatiles", m.w, m.h, full.w, full.h)
	}
	for y := 0; y < m.h; y++ {
		for x := 0; x < m.w; x++ {
			full.set(x+ox, y+oy, m.at(x, y))
		}
	}
	connect(full)
	addDoors(full, p.doors, rnd)
	return toCells(full), nil
}

func fromDungeon(dm *dungen.DungeonMap) *mega {
	w, h := dm.GetSize()
	m := newMega(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch dm.GetTile(x, y) {
			case dungen.Room:
				m.set(x, y, mRoom)
			case dungen.Corridor, dungen.Shallows:
				m.set(x, y, mCorr)
			case dungen.Door:
				m.set(x, y, mDoor)
			case dungen.Lake:
				m.set(x, y, mLava)
			}
		}
	}
	return m
}

// toCells draws megatiles as D1 draws them, two by two cells: a wall megatile keeps its wall on its top and left
// cells and gives its bottom and right cells to the floor beside it, so walls are one cell thick and a corridor
// one megatile wide is three cells wide. A door is one door cell in that wall line; it counts as wall for the
// cells around it, so nothing slips past it. A post is a floor megatile with a pillar in its top-left cell.
func toCells(m *mega) *grid {
	g := newGrid(2*m.w, 2*m.h)
	floorLike := func(k int) bool { return k != mWall && k != mDoor }
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			i, j, a, b := x/2, y/2, x%2, y%2
			switch k := m.at(i, j); {
			case k == mPost && a == 0 && b == 0:
			case open(k):
				g.set(x, y, cFloor)
			case k == mLava:
				g.set(x, y, cWater)
			default: // wall or door: the cell is floor where it borders open ground on its right or bottom
				right, below, diag := floorLike(m.at(i+1, j)), floorLike(m.at(i, j+1)), floorLike(m.at(i+1, j+1))
				if a == 1 && right || b == 1 && below || a == 1 && b == 1 && diag {
					g.set(x, y, lavaOr(m, i, j, a, b))
				} else if k == mDoor && a == 0 && b == 0 {
					g.set(x, y, cDoor)
				}
			}
		}
	}
	return g
}

// lavaOr is what a wall megatile's given-away cell shows: the ground beside it that it was given to (right,
// else below, else diagonally), lava when that is lava.
func lavaOr(m *mega, i, j, a, b int) cell {
	next := m.at(i+1, j+1)
	switch floorLike := func(k int) bool { return k != mWall && k != mDoor }; {
	case a == 1 && floorLike(m.at(i+1, j)):
		next = m.at(i+1, j)
	case b == 1 && floorLike(m.at(i, j+1)):
		next = m.at(i, j+1)
	}
	if next == mLava {
		return cWater
	}
	return cFloor
}
