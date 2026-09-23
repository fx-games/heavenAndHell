package dungen

import (
	"fmt"

	"rpg/src/dungeon"
	"rpg/src/hhm"
	"rpg/src/recfile"
	"rpg/src/rules"
)

// Population is what fills a laid-out level: D1's rules for one of its level types (levels.rec's
// "population: d1_cathedral", ...), which roll monsters, objects, items and theme rooms on the level's D1 seed.
type Population struct{ Type int }

// populations are the knob's values: the level type whose rules a level is populated by.
var populations = map[string]int{"d1_cathedral": rules.DTypeCathedral, "d1_catacombs": rules.DTypeCatacombs,
	"d1_caves": rules.DTypeCaves, "d1_hell": rules.DTypeHell}

// ParsePopulation reads a record's population knob; ok is false when it names none (the engine's own).
func ParsePopulation(r recfile.Record) (p Population, ok bool, err error) {
	v, ok := dungeon.Field(r, "population")
	if !ok {
		return p, false, nil
	}
	if p.Type = populations[v]; p.Type == 0 {
		return p, false, fmt.Errorf("levels.rec: population %q: want one of d1_cathedral, d1_catacombs, d1_caves, d1_hell", v)
	}
	return p, true, nil
}

// Populate runs D1's LoadGameLevel after the layout, on a level laid out by the passes: the triggers are the
// stairs the caller placed (piece coordinates), the rest is D1's own order of rolls from the level's seed. The
// map must be D1's 80x80 cells drawn at scale 2, so its grid is D1's 40x40 megatiles.
func (l *Levels) Populate(gnr *rules.Generator, q *rules.Quests, seeds [rules.NumLevels]int32, dlvl int, p Population,
	m *DungeonMap, trigs []Trigger) (lvl *Level, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("populating level %d: %v", dlvl, r)
		}
	}()
	if w, h := m.GetSize(); w != LevelSize || h != LevelSize || m.scale != 2 {
		return nil, fmt.Errorf("population: level %d is %dx%d at scale %d, want %dx%d at scale 2", dlvl, w, h, m.scale, LevelSize, LevelSize)
	}
	g := &gen{quests: q, currlevel: dlvl, ltype: p.Type, cells: m, levels: l}
	g.data = gnr.D
	g.igen = &rules.Generator{D: gnr.D, R: &g.rng, UniqueDropped: make([]bool, len(gnr.UniqueDropped))}
	defer func() {
		for u, d := range g.igen.UniqueDropped {
			gnr.UniqueDropped[u] = gnr.UniqueDropped[u] || d
		}
	}()
	seed := seeds[dlvl]
	g.rng.SetSeed(seed)
	g.initLevelMonsters()
	g.initTrans()
	g.floodCells()
	for _, r := range m.themeRooms { // walled in by the passes: rooms of their own, as DRLG_PlaceThemeRooms marks them
		g.mRectTrans(r[0], r[1], r[0]+r[2]-1, r[1]+r[3]-1)
		g.themes = append(g.themes, themeLoc{room: rect{r[0], r[1], r[2], r[3]}, ttval: g.transVal - 1})
	}
	g.trigs = trigs
	g.freeUpstairs()
	g.rng.SetSeed(seed)
	g.getLevelMTypes(rules.SLNone)
	g.initThemes()
	g.rng.SetSeed(seed)
	g.holdThemes()
	g.rng.Seed()
	g.initMonsters(rules.SLNone)
	g.rng.Seed()
	g.initObjects(seeds)
	g.hangTorchesOnWalls()
	g.moveMonstersOffObjects()
	g.initItems()
	g.createThemeRooms()
	return g.output(rules.SLNone), nil
}

// cellAt is the laid-out tile at piece (x, y); off the level is wall.
func (g *gen) cellAt(x, y int) DungeonTile {
	x, y = x-CellOrigin, y-CellOrigin
	if x < 0 || y < 0 || x >= LevelSize || y >= LevelSize {
		return Wall
	}
	return g.cells.GetTile(x, y)
}

// cellFlags stands in for D1's piece tables on a laid-out level: walls block everything and may hold a trap,
// pillars and lava block the way, and everything else is open.
func (g *gen) cellFlags(x, y int) hhm.Flags {
	switch g.cellAt(x, y) {
	case Wall:
		return hhm.Flags{BlocksMovement: true, BlocksView: true, BlocksMissiles: true, HoldsTrap: true}
	case Pillar:
		return hhm.Flags{BlocksMovement: true, BlocksMissiles: true}
	case Lava:
		return hhm.Flags{BlocksMovement: true, Damaging: true}
	case Lake:
		return hhm.Flags{BlocksMovement: true, Water: true}
	}
	return hhm.Flags{}
}

// cellFloor is floor a room is made of: doors part rooms, as D1's door megatiles do.
func (g *gen) cellFloor(x, y int) bool {
	t := g.cellAt(x, y)
	return t == Room || t == Corridor || t == Shallows
}

// floodCells ports FloodTransparencyValues on cells: one room region per area of floor, the walls and doors
// around it taking its value (the last region to touch them wins, as in D1).
func (g *gen) floodCells() {
	for y := CellOrigin; y < CellOrigin+LevelSize; y++ {
		for x := CellOrigin; x < CellOrigin+LevelSize; x++ {
			if g.cellFloor(x, y) && g.trans[x][y] == 0 {
				g.findTrans(x, y, 0)
				g.transVal++
			}
		}
	}
}

// addCellObjs ports AddL1Objs/AddL2Objs/AddL3Objs on cells: a door object on every door, turned the way its
// wall runs, and on the cathedral the 5-9 lamps l1LAMPS stands in open floor (DRLG_L1's lamp minisets).
func (g *gen) addCellObjs(ltype int) {
	doors := map[int][2]int{rules.DTypeCathedral: {rules.ObjL1LDOOR, rules.ObjL1RDOOR},
		rules.DTypeCatacombs: {rules.ObjL2LDOOR, rules.ObjL2RDOOR}, rules.DTypeCaves: {rules.ObjL3LDOOR, rules.ObjL3RDOOR}}
	if ltype == rules.DTypeCathedral {
		for n := g.rnd(5) + 5; n > 0; n-- {
			for try := 0; try < 1000; try++ {
				x, y := g.rnd(80)+16, g.rnd(80)+16
				if g.openAround(x, y) && g.dObj(x, y) == 0 {
					g.addObject(rules.ObjL1LIGHT, x, y)
					break
				}
			}
		}
	}
	d, ok := doors[ltype]
	if !ok {
		return
	}
	for j := 0; j < 112; j++ {
		for i := 0; i < 112; i++ {
			if g.cellAt(i, j) != Door {
				continue
			}
			if g.solid(i-1, j) { // walled west and east: the way through runs north-south
				g.addObject(d[1], i, j)
			} else {
				g.addObject(d[0], i, j)
			}
		}
	}
}

// openAround is floor with floor all round it.
func (g *gen) openAround(x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if !g.cellFloor(x+dx, y+dy) {
				return false
			}
		}
	}
	return true
}

// addCellTorches ports AddL2Torches on cells. D1 hangs a torch on one wall piece of each wall megatile facing
// the floor (1 in 3) and stands one in front of another (1 in 10); on cells that is every other wall cell with
// floor below it (TORCHL2) or to its right (TORCHR2), and a torch on the floor west or north of every other
// (TORCHL/TORCHR), which hangTorchesOnWalls hangs on the wall.
func (g *gen) addCellTorches() {
	for j := 0; j < 112; j++ {
		for i := 0; i < 112; i++ {
			if g.flags[i][j]&bflagPopulated != 0 || g.cellAt(i, j) != Wall || (i+j)%2 != 0 {
				continue
			}
			below, right := g.cellFloor(i, j+1), g.cellFloor(i+1, j)
			if below && !right && g.rnd(3) == 0 {
				g.addObject(rules.ObjTORCHL2, i, j)
			}
			if right && !below && g.rnd(3) == 0 {
				g.addObject(rules.ObjTORCHR2, i, j)
			}
			if g.cellFloor(i-1, j) && g.rnd(10) == 0 && g.dObj(i-1, j) == 0 { // hangTorchesOnWalls hangs it back east
				g.addObject(rules.ObjTORCHL, i-1, j)
			}
			if g.cellFloor(i, j-1) && g.rnd(10) == 0 && g.dObj(i, j-1) == 0 { // and this one back south
				g.addObject(rules.ObjTORCHR, i, j-1)
			}
		}
	}
}

// addCellHookedBodies ports AddHookedBodies on cells: per wall megatile facing floor to its east or south, with
// chance 1/freq, a body hangs on the floor cell in front of it.
func (g *gen) addCellHookedBodies(freq int) {
	for j := 0; j < 40; j++ {
		for i := 0; i < 40; i++ {
			x, y := 16+2*i, 16+2*j
			if g.cellAt(x, y) != Wall || g.rnd(freq) != 0 || g.nearThemeRoom(i, j) {
				continue
			}
			switch {
			case g.cellFloor(x+1, y) && g.cellAt(x, y+1) == Wall:
				g.addObject([3]int{rules.ObjTORTURE1, rules.ObjTORTURE2, rules.ObjTORTURE5}[g.rnd(3)], x+1, y)
			case g.cellFloor(x, y+1) && g.cellAt(x+1, y) == Wall:
				g.addObject([2]int{rules.ObjTORTURE3, rules.ObjTORTURE4}[g.rnd(2)], x, y+1)
			}
		}
	}
}
