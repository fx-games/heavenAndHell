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
type Population struct {
	Type     int
	SetLevel string // the quest whose set level this is (levels.rec's "quest:"), "" for a dungeon level
	// LibraryStands is the chance in percent that a free cell of a library theme room gets a bookstand
	// (levels.rec's "library_stands:"); 0 keeps D1's 1 in 1/2/2/5 by level type.
	LibraryStands int
}

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
	p.SetLevel, _ = dungeon.Field(r, "quest")
	if p.LibraryStands, err = dungeon.Int(r, "library_stands", 0); err != nil {
		return p, false, err
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
	g := &gen{quests: q, currlevel: dlvl, ltype: p.Type, cells: m, libraryStands: p.LibraryStands}
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
	if p.SetLevel != "" {
		return g.populateSetLevel(p.SetLevel, trigs), nil
	}
	for _, r := range m.themeRooms { // walled in by the passes: rooms of their own, as DRLG_PlaceThemeRooms marks them
		g.mRectTrans(r[0], r[1], r[0]+r[2]-1, r[1]+r[3]-1)
		g.themes = append(g.themes, themeLoc{room: rect{r[0], r[1], r[2], r[3]}, ttval: g.transVal - 1})
	}
	for i, p := range m.pieces { // Make_SetPC: monsters stay out of the set pieces; the first is D1's setpc
		r := rect{p.X / 2, p.Y / 2, (p.Map.Width + 1) / 2, (p.Map.Height + 1) / 2}
		if i == 0 {
			g.setPiece = r
		}
		g.makeSetPC(r)
	}
	g.pieceQuests()
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

// cellFlags stands in for D1's piece tables on a laid-out level: walls and closed doors block everything, walls
// may hold a trap, pillars and lava block the way, and everything else is open.
func (g *gen) cellFlags(x, y int) hhm.Flags {
	switch g.cellAt(x, y) {
	case Wall: // D1's trap walls are every other wall piece, and none in the caves
		return hhm.Flags{BlocksMovement: true, BlocksView: true, BlocksMissiles: true, HoldsTrap: g.ltype != rules.DTypeCaves && (x+y)%2 == 0}
	case Door: // a closed door's piece is solid in D1: it parts two rooms
		return hhm.Flags{BlocksMovement: true, BlocksView: true, BlocksMissiles: true}
	case Pillar:
		return hhm.Flags{BlocksMovement: true, BlocksMissiles: true}
	case Fence:
		return hhm.Flags{BlocksMovement: true}
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
				g.findTrans(x, y)
				g.transVal++
			}
		}
	}
}

// addCellObjs ports AddL1Objs/AddL2Objs/AddL3Objs on cells: a light on each of the cathedral's lamps, and a door
// object on every door, turned the way its wall runs.
func (g *gen) addCellObjs(ltype int) {
	// [0] in a wall running north-south (the way through runs east-west), [1] across it; the caves' are the other way
	doors := map[int][2]int{rules.DTypeCathedral: {rules.ObjL1LDOOR, rules.ObjL1RDOOR},
		rules.DTypeCatacombs: {rules.ObjL2LDOOR, rules.ObjL2RDOOR}, rules.DTypeCaves: {rules.ObjL3RDOOR, rules.ObjL3LDOOR}}
	for y := 0; y < LevelSize; y++ { // AddL1Objs: a light on each lamp (piece 270)
		for x := 0; x < LevelSize; x++ {
			if ltype == rules.DTypeCathedral && g.cells.Role(pt(x, y)) == "lamp" {
				g.addObject(rules.ObjL1LIGHT, x+CellOrigin, y+CellOrigin)
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

// addCellTorches ports AddL2Torches on cells. D1 hangs a torch on one wall piece of each wall megatile facing
// the floor (1 in 3) and stands one in front of another (1 in 10); on cells that is a wall cell every eight along
// a wall with floor east of it (TORCHL2) or south of it (TORCHR2), and a torch on the floor west or north of one
// (TORCHL/TORCHR), which hangTorchesOnWalls hangs on the wall. Sides and spacing are D1's.
func (g *gen) addCellTorches() {
	for j := 0; j < 112; j++ {
		for i := 0; i < 112; i++ {
			if g.flags[i][j]&bflagPopulated != 0 || g.cellAt(i, j) != Wall {
				continue
			}
			n, e, so, w := g.cellFloor(i, j-1), g.cellFloor(i+1, j), g.cellFloor(i, j+1), g.cellFloor(i-1, j)
			if e && !n && !so && j%8 == 0 && g.rnd(3) == 0 { // D1's piece 1: floor east (and maybe west)
				g.addObject(rules.ObjTORCHL2, i, j)
			}
			if so && !e && !w && i%8 == 0 && g.rnd(3) == 0 { // piece 5: floor south (and maybe north)
				g.addObject(rules.ObjTORCHR2, i, j)
			}
			if w && !n && !e && !so && j%4 == 0 && g.rnd(10) == 0 && g.dObj(i-1, j) == 0 && g.dObj(i, j) == 0 { // floor west only: hung back east
				g.addObject(rules.ObjTORCHL, i-1, j)
			}
			if n && !e && !so && !w && i%4 == 0 && g.rnd(10) == 0 && g.dObj(i, j-1) == 0 && g.dObj(i, j) == 0 { // floor north only: hung back south
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
			if g.cellAt(x, y) != Wall || (i+j)%2 != 0 || g.rnd(freq) != 0 || g.nearThemeRoom(i, j) { // every other: D1's count
				continue
			}
			switch { // D1's straight walls only (megatiles 1 and 2 beside floor 6)
			case g.cellFloor(x+1, y) && g.cellAt(x, y+1) == Wall && g.cellAt(x, y-1) == Wall:
				g.addObject([3]int{rules.ObjTORTURE1, rules.ObjTORTURE2, rules.ObjTORTURE5}[g.rnd(3)], x+1, y)
			case g.cellFloor(x, y+1) && g.cellAt(x+1, y) == Wall && g.cellAt(x-1, y) == Wall:
				g.addObject([2]int{rules.ObjTORTURE3, rules.ObjTORTURE4}[g.rnd(2)], x, y) // hung on the wall itself
			}
		}
	}
}

// pieceEntities calls fn for every entity of kind the level's set pieces carry, at its piece coordinates.
func (g *gen) pieceEntities(kind string, fn func(p Stamped, e *hhm.Entity, x, y int)) {
	for _, p := range g.cells.pieces {
		for _, e := range p.Map.Entities {
			if e.Kind == kind {
				fn(p, e, p.X+e.X+CellOrigin, p.Y+e.Y+CellOrigin)
			}
		}
	}
}

// pieceLoc is where a set piece names a spot (a unique's name, a quest's key), in piece coordinates.
func (g *gen) pieceLoc(name string) (x, y int, ok bool) {
	g.pieceEntities("Location", func(_ Stamped, e *hhm.Entity, ex, ey int) {
		if e.Str("Name") == name {
			x, y, ok = ex, ey, true
		}
	})
	return
}

// pieceMonsters ports SetMapMonsters for every set piece: the monsters its map places.
func (g *gen) pieceMonsters() {
	g.pieceEntities("Monster", func(_ Stamped, e *hhm.Entity, x, y int) {
		if t := g.data.MonsterByID(e.Str("ID")); t >= 0 {
			g.placeMonster(len(g.monsters), g.addMonsterType(t, placeSpecial), x, y)
		}
	})
}

// pieceObjects ports LoadMapObjects for every set piece: its objects, each with the part of the level (in
// megatiles) and the lever group it opens.
func (g *gen) pieceObjects() {
	byName := map[string]int{}
	for k, o := range g.data.Objects {
		byName[o.Name] = k
	}
	g.initObjFlag = true
	g.pieceEntities("Object", func(p Stamped, e *hhm.Entity, x, y int) {
		k, ok := byName[e.Str("ID")]
		if !ok {
			panic(fmt.Sprintf("set piece %s: unknown object %q", p.Map.Name, e.Str("ID")))
		}
		g.addObject(k, x, y)
		o := g.objAt(x, y)
		if o == nil {
			return
		}
		var r [4]int
		if n, _ := fmt.Sscan(e.Str("Range"), &r[0], &r[1], &r[2], &r[3]); n == 4 {
			o.Var1, o.Var2, o.Var3, o.Var4 = p.X/2+r[0], p.Y/2+r[1], p.X/2+r[2], p.Y/2+r[3]
		}
		for _, q := range g.cells.pieces {
			if q.Map.Name == e.Str("Opens") {
				o.Var1, o.Var2, o.Var3, o.Var4 = q.X/2, q.Y/2, (q.X+q.Map.Width)/2, (q.Y+q.Map.Height)/2
			}
		}
		if e.Int("Lever") != 0 {
			o.Var8 = e.Int("Lever")
		}
	})
	g.initObjFlag = false
}

// pieceItems ports the quest items a set piece lies with (the Anvil of Fury on its island).
func (g *gen) pieceItems() {
	g.pieceEntities("Item", func(_ Stamped, e *hhm.Entity, x, y int) { g.spawnQuestItem(e.Str("ID"), x, y, 1) })
}

// pieceQuests points each quest a set piece names a spot for (its entrance) at it, as DRLG_CheckQuests does.
func (g *gen) pieceQuests() {
	for q, key := range rules.QuestKeys {
		if x, y, ok := g.pieceLoc(key); ok && q < len(g.quests) {
			g.quests[q].X, g.quests[q].Y = x, y
		}
	}
}

// populateSetLevel ports LoadSetMap's population of a quest set level laid out whole from its map: the quest
// steps entering it takes, the map's monsters and uniques and objects with their levers, no random ones.
func (g *gen) populateSetLevel(quest string, trigs []Trigger) *Level {
	sl := rules.SLNone
	for q, key := range rules.QuestKeys {
		if key == quest && q < len(g.quests) {
			sl = g.quests[q].SetLevel
		}
	}
	if sl == rules.SLNone {
		panic(fmt.Sprintf("quest %q has no D1 set level", quest))
	}
	g.setlevel, g.setLvlNum = true, sl
	switch sl {
	case rules.SLSkelKing:
		if q := &g.quests[rules.QSkelKing]; q.Active == rules.QuestInit {
			q.Active, q.Var1 = rules.QuestActive, 1
		}
	case rules.SLPoisonWater:
		if q := &g.quests[rules.QPWater]; q.Active == rules.QuestInit {
			q.Active = rules.QuestActive
		}
	case rules.SLVileBetrayer:
		if q := &g.quests[rules.QBetrayer]; q.Active == rules.QuestDone {
			q.Var2 = 4
		} else if q.Active == rules.QuestActive {
			q.Var2 = 3
		}
	}
	for i := range trigs {
		trigs[i].Msg = TrigReturnLevel
	}
	g.trigs = trigs
	g.getLevelMTypes(sl)
	g.addMonsterType(rules.MtGOLEM, placeSpecial)
	for i := 0; i < 4; i++ {
		g.monsters[g.addMonster(1, 0, 0, 0, false)].Reserved = true
	}
	if sl == rules.SLVileBetrayer {
		for _, u := range []int{umtLazarus, umtRedVex, umtBlackJade} {
			g.addMonsterType(g.uniqueType(u), placeUnique)
		}
		for _, u := range []int{umtLazarus, umtRedVex, umtBlackJade} {
			g.placeUniqueMonst(u, 0, 0)
		}
	}
	g.pieceMonsters()
	g.initMonsters(sl)
	g.clrAllObjects()
	g.pieceObjects()
	g.addCellObjs(g.ltype)
	g.initItems()
	return g.output(sl)
}

// available ports QuestStatus.
func (g *gen) available(i int) bool {
	q := &g.quests[i]
	return !g.setlevel && g.currlevel == q.Level && q.Active != rules.QuestNotAvail
}
