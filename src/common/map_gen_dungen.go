package common

import (
	"cmp"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"

	"rpg/files"
	"rpg/src/defs"
	"rpg/src/dungen"
	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/hhm"
	"rpg/src/rules"
)

// layoutStrategy is the LevelStrategy of every dungen layout generator (dungen.Layouts): the generator carves a
// bare dungen.DungeonMap - walls, floors, doors and rooms, no tile art, no actors - and it is laid out here.
type layoutStrategy struct{ dungeon.Parser }

func (layoutStrategy) Generate(g *MapGenerator, lvl dungeon.Level) *gridmap.GridMap[*Actor, *Item, Object] {
	return g.generateDungenMap(lvl)
}

// generateDungenMap builds one dungen level and lays it out as a gridmap.
func (g *MapGenerator) generateDungenMap(lvl dungeon.Level) *gridmap.GridMap[*Actor, *Item, Object] {
	targetMap, dlvl := lvl.Map, lvl.Depth
	cfg := dungen.WithQuests(lvl.Config, func(key string) bool { return g.questHere(key, dlvl) })
	layout, err := cfg.(dungen.Layout).Build(g.randomSource, g.fsys())
	if err != nil {
		panic(err)
	}
	width, height := layout.GetSize()
	m := gridmap.NewEmptyMap[*Actor, *Item, Object](width, height)
	m.SetName(targetMap)
	m.SetMetaString(layout.GetMetaInfo())

	_, trapWalls := dungen.PopulationOf(lvl.Config)
	g.addDungenTiles(m, layout, g.dungenTilemap(lvl, dlvl, layout), dlvl, lvl.Tiles, trapWalls)
	g.paintPieces(m, layout)
	stairs := g.addDungenStairs(m, layout, lvl)
	if pop, ok := dungen.PopulationOf(lvl.Config); ok {
		g.populateD1(m, layout, lvl, pop, stairs)
	} else {
		g.decorateDungenRooms(m, layout, lvl, dlvl)
		for _, pc := range layout.Pieces() { // D1's population places its pieces' creatures itself
			g.spawnDrawnUniques(m, pc.Map.Entities, func(e *hhm.Entity) geometry.Point { return geometry.Point{X: pc.X + e.X, Y: pc.Y + e.Y} })
		}
		g.populateDungenLevel(m, layout, lvl, stairs)
	}

	if snap := g.snapshots[targetMap]; snap != nil {
		snap.apply(g.factory, m)
	}
	return m
}

// revealSecretDoor turns the secret door at p into the door it hid.
func (g *MapGenerator) revealSecretDoor(m *gridmap.GridMap[*Actor, *Item, Object], p geometry.Point) {
	t := g.doorTiles[m.GetName()]
	m.SetTile(p, dungenFloor(t[0]))
	m.AddObject(NewDoor(t[1], t[2]), p)
	m.FoundSecretDoorAt(p)
}

// dungenTilemap resolves the tile set the level names.
func (g *MapGenerator) dungenTilemap(lvl dungeon.Level, dlvl int, layout *dungen.DungeonMap) defs.BasicTileMapping {
	isWall := func(p geometry.Point) bool { return !layout.Contains(p) || layout.GetTileAt(p) == dungen.Wall }
	if tm, ok := defs.Tilemap(lvl.Tiles, g.randomSource, isWall); ok {
		return tm
	}
	panic(fmt.Sprintf("levels.rec: %s: unknown tile set %q (have %v)", lvl.Map, lvl.Tiles, defs.TilemapNames()))
}

func dungenFloor(icon int32) gridmap.Tile {
	return gridmap.Tile{DefinedIcon: icon, DefinedDescription: "a floor", IsWalkable: true, IsTransparent: true, IsMissilePassable: true}
}

// secretDoorChance is how many loop doors are secret at depth dlvl: none on the first level, two in three at the
// bottom (Brogue's finishDoors).
func (g *MapGenerator) secretDoorChance(dlvl int) float64 {
	return min(0.67, max(0, float64(dlvl-1)*0.67/float64(max(g.maxDepth()-1, 1))))
}

// addDungenTiles paints the layout: a wall cell is a wall, a door cell is floor with a door object standing on
// it (decision 12, the same rule D1's side follows), and everything else is floor. A loop door may be a secret
// door instead: a wall until the hero finds it (revealSecretDoor).
func (g *MapGenerator) addDungenTiles(m *gridmap.GridMap[*Actor, *Item, Object], layout *dungen.DungeonMap, tileMap defs.BasicTileMapping, dlvl int, tiles string, trapWalls bool) {
	// the tile set's own records name its plain cells ("Cathedral Wall"), and its trap walls are the wall cells a
	// D1 population may set a trap in (dungen's cellFlags)
	named := func(t gridmap.Tile, rec string) gridmap.Tile {
		if r, ok := defs.TileRecords["tile."+tiles+"_"+rec]; ok {
			t.DefinedDescription = r.Name
		}
		return t
	}
	wall := func(icon int32) gridmap.Tile {
		return gridmap.Tile{DefinedIcon: icon, DefinedDescription: "a wall"}
	}
	floor := func(icon int32) gridmap.Tile { return named(dungenFloor(icon), "floor") }
	if g.doorTiles == nil {
		g.doorTiles = map[string][3]int32{}
	}
	g.doorTiles[m.GetName()] = [3]int32{tileMap.FloorTile, tileMap.OpenDoor, tileMap.ClosedDoor}
	secret := g.secretDoorChance(dlvl)
	pillar := tileMap.WallTile // a pillar levels.rec's "tile:" names no record for
	width, height := layout.GetSize()
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			p := geometry.Point{X: x, Y: y}
			if rec, ok := layout.Art(p); ok {
				m.SetTile(p, tileFromRecord(rec))
				continue
			}
			switch layout.GetTile(x, y) {
			case dungen.Wall:
				if trapWalls && (x+y)%2 == 0 {
					m.SetTile(p, named(named(wall(tileMap.ChooseWallTile(p)), "wall"), "trap_wall"))
				} else {
					m.SetTile(p, named(wall(tileMap.ChooseWallTile(p)), "wall"))
				}
			case dungen.Door:
				if layout.IsLoopDoor(p) && g.randomSource.Float64() < secret {
					m.SetTile(p, wall(tileMap.ChooseWallTile(p)))
					m.SetSecretDoorAt(p)
					continue
				}
				m.SetTile(p, floor(tileMap.FloorTile))
				m.AddObject(NewDoor(tileMap.OpenDoor, tileMap.ClosedDoor), p)
			case dungen.Lake:
				m.SetTile(p, gridmap.Tile{DefinedIcon: defs.IconWater, DefinedDescription: "deep water", IsTransparent: true, IsMissilePassable: true, Special: gridmap.SpecialTileWater})
			case dungen.Lava:
				m.SetTile(p, g.lavaTile(m))
			case dungen.Pillar:
				m.SetTile(p, gridmap.Tile{DefinedIcon: pillar, DefinedDescription: "a pillar", IsTransparent: true})
			case dungen.Shallows:
				m.SetTile(p, gridmap.Tile{DefinedIcon: defs.IconWater, DefinedDescription: "shallow water", IsWalkable: true, IsTransparent: true, IsMissilePassable: true, Special: gridmap.SpecialTileWater})
			default: // Room, Corridor
				m.SetTile(p, floor(tileMap.ChooseFloorTile(p)))
			}
		}
	}
}

func dungenLadderUp() gridmap.Tile {
	return gridmap.Tile{DefinedIcon: defs.IconStairsUp, DefinedDescription: "a ladder leading up",
		IsWalkable: true, IsTransparent: true, IsMissilePassable: true, Special: gridmap.SpecialTileStairsUp}
}

func dungenLadderDown() gridmap.Tile {
	return gridmap.Tile{DefinedIcon: defs.IconStairsDown, DefinedDescription: "a ladder leading down",
		IsWalkable: true, IsTransparent: true, IsMissilePassable: true, Special: gridmap.SpecialTileStairsDown}
}

// addDungenStairs places a stairwell for each exit the level declares and names it by the exit's location:
// ladder_up by the entrance, ladder_down as far from it as the layout allows, any other exit on the spots
// furthest from both. A stairwell leading to a shallower level looks like a way up. It returns where they went
// (monsters keep their distance from them); where each leads is wireExits' business.
func (g *MapGenerator) addDungenStairs(m *gridmap.GridMap[*Actor, *Item, Object], layout *dungen.DungeonMap, lvl dungeon.Level) []geometry.Point {
	spots := layout.GetPotentialStairPositions()
	if len(spots) < 2 {
		// A level too cramped for two stair spots would be a level you cannot leave. Rather than crash on a bad
		// seed, fall back to any two walkable cells far apart.
		spots = append(spots, g.dungenFallbackStairs(m, layout)...)
	}
	var placed []geometry.Point
	pieceLocs := map[string]geometry.Point{} // exits a set piece places: its stairs are drawn in it
	for _, pc := range layout.Pieces() {
		for _, e := range pc.Map.Entities {
			if e.Kind == "Location" {
				pieceLocs[e.Str("Name")] = geometry.Point{X: pc.X + e.X, Y: pc.Y + e.Y}
			}
		}
	}
	put := func(e dungeon.Exit, p geometry.Point) {
		tile := dungenLadderDown()
		if target := g.levels[e.Target]; target.Town || target.Depth < lvl.Depth || e.Location == "ladder_up" {
			tile = dungenLadderUp()
		}
		m.SetTile(p, tile)
		m.AddNamedLocation(e.Location, p)
		spots = removePoint(spots, p)
		placed = append(placed, p)
	}
	// The entrance is in the room the level grew from, when it has a stair spot: the generator puts quest rooms far
	// from that room, so far from the way in.
	entrances := spots
	if rooms := layout.AllRooms(); len(rooms) > 0 {
		if in := slices.DeleteFunc(slices.Clone(spots), func(p geometry.Point) bool { return !rooms[0].Contains(p) }); len(in) > 0 {
			entrances = in
		}
	}
	entrance := entrances[g.randomSource.Intn(len(entrances))]
	for _, l := range g.levels { // where other levels' exits arrive (a quest set level's way in)
		for _, e := range l.Exits {
			if p, ok := pieceLocs[e.Arrival]; ok && e.Target == lvl.Map {
				m.AddNamedLocation(e.Arrival, p)
			}
		}
	}
	var rest []dungeon.Exit
	for _, e := range lvl.Exits {
		if p, ok := pieceLocs[e.Location]; ok {
			m.AddNamedLocation(e.Location, p)
			spots = removePoint(spots, p)
			placed = append(placed, p)
			continue
		}
		if g.factory.State.QuestByKey(e.Location) >= 0 {
			continue // a quest's way in, and its set piece is not on the level
		}
		if _, ok := lvl.Locations[e.Location]; ok {
			continue // levels.rec places it itself
		}
		if e.Location == "ladder_up" {
			put(e, entrance)
		} else {
			rest = append(rest, e)
		}
	}
	for _, e := range rest {
		if len(spots) == 0 {
			slog.Error("no stair position left", "exit", e.Location, "map", m.GetName())
			break
		}
		// ladder_down as far from the entrance as it gets (or, half the time, anywhere); the rest furthest from
		// every stair so far, so a shortcut back to town does not open next to the stairs you came down
		slices.SortStableFunc(spots, func(a, b geometry.Point) int {
			return cmp.Compare(minDist(append(placed, entrance), a), minDist(append(placed, entrance), b))
		})
		p := spots[len(spots)-1]
		if e.Location == "ladder_down" && g.randomSource.Intn(2) == 1 {
			p = spots[g.randomSource.Intn(len(spots))]
		}
		put(e, p)
	}
	return placed
}

// dungenFallbackStairs is the escape hatch for a layout with no room shaped like a stairwell: the walkable cells
// furthest apart, which is worse-looking than a real stair nook but still a level you can leave.
func (g *MapGenerator) dungenFallbackStairs(m *gridmap.GridMap[*Actor, *Item, Object], layout *dungen.DungeonMap) []geometry.Point {
	var open []geometry.Point
	width, height := layout.GetSize()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if p := (geometry.Point{X: x, Y: y}); layout.IsEmptySpace(p) && !m.IsObjectAt(p) {
				open = append(open, p)
			}
		}
	}
	if len(open) < 2 {
		panic("dungen: level has fewer than two walkable cells")
	}
	return []geometry.Point{open[0], open[len(open)-1]}
}

func removePoint(ps []geometry.Point, p geometry.Point) []geometry.Point {
	if i := slices.Index(ps, p); i >= 0 {
		return slices.Delete(slices.Clone(ps), i, i+1)
	}
	return ps
}

// dungenMaxLit is how much of a level is lit at depth 0; it fades to nothing at the bottom of the dungeon.
const dungenMaxLit = 0.8

// decorateDungenRooms paints the hand-drawn art over the rooms whose template asked for it (the room template's
// own tiles, laid over the floor the tile set already painted), then lights some of the level.
func (g *MapGenerator) decorateDungenRooms(m *gridmap.GridMap[*Actor, *Item, Object], layout *dungen.DungeonMap, lvl dungeon.Level, dlvl int) {
	if proj, _ := dungen.Templates(lvl.Config, g.fsys()); proj != nil { // nil for a generator that draws no templates
		for _, room := range layout.AllRooms() {
			if !room.HasDecoratedTemplate() {
				continue
			}
			if deco := g.dungenDecoration(proj, room.GetDecorationLevelName()); deco != nil {
				g.paintDungenDecoration(m, proj, deco, room)
				m.AddToMetaInfo("DecoRoom: " + deco.Name)
			}
		}
	}
	// Light whole spaces up to the depth's share of the floor, each by torches on its walls. The space that would
	// overshoot the share is lit by chance, (share left)/(its size), so a level that is one open cavern is lit as
	// often as its share says.
	spaces := dungenSpaces(layout)
	floor := 0
	for _, sp := range spaces {
		floor += len(sp)
	}
	want := float64(floor) * min(dungenMaxLit, max(0, 1.0-float64(dlvl)/float64(max(g.maxDepth(), 1))))
	lit := 0.0
	for _, i := range g.randomSource.Perm(len(spaces)) {
		size := float64(len(spaces[i]))
		if lit+size > want && g.randomSource.Float64() >= (want-lit)/size {
			break
		}
		lit += size
		g.lightSpace(m, spaces[i])
		if lit >= want {
			break
		}
	}
}

// torchSpacing is how far apart the torches lighting one space hang, at least.
const torchSpacing = 10

// lightSpace lights a space, walls included, and hangs torches on its walls, torchSpacing apart, so its light has
// a source one can see (their flicker is glowAt's). Lighting only what the torches reach left small dark pockets
// in lit rooms, the very look this replaces.
func (g *MapGenerator) lightSpace(m *gridmap.GridMap[*Actor, *Item, Object], space []geometry.Point) {
	var walls []geometry.Point // wall cells facing the space, in the order the space was found
	for _, p := range space {
		m.SetLit(p, true)
		for _, n := range m.GetFilteredNeighbors(p, m.Contains) {
			m.SetLit(n, true)
		}
		for _, d := range []geometry.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
			if w := p.Add(d); m.Contains(w) && !m.IsTileWalkable(w) && !m.IsTransparent(w) && !slices.Contains(walls, w) {
				walls = append(walls, w)
			}
		}
	}
	var torches []geometry.Point
	for _, i := range g.randomSource.Perm(len(walls)) {
		w := walls[i]
		if m.IsSecretDoorAt(w) || slices.ContainsFunc(torches, func(t geometry.Point) bool { return geometry.Distance(t, w) < torchSpacing }) {
			continue
		}
		torches = append(torches, w)
		m.AddDecal(w, defs.IconWallTorch)
	}
}

// dungenSpaces are the parts of a level that are lit, each as a whole: the room floor one can walk without passing
// through an opening. Lighting rooms one by one left the rooms that open onto a lit one without a door dark inside it, and
// the cells no room owns (a knocked-through wall, a passage) dark between lit ones.
// ponytail: templates' CanBeLit is not read; only the Cathedral's set it, so honouring it would darken the rest.
func dungenSpaces(layout *dungen.DungeonMap) [][]geometry.Point {
	// Openings between rooms (doors, and the passages a doorless level cuts instead) bound a space; the ring of
	// neighbours lit around a lit space lights the opening it looks through.
	open := func(p geometry.Point) bool {
		t := layout.GetTile(p.X, p.Y)
		return t == dungen.Room || t == dungen.Shallows
	}
	w, h := layout.GetSize()
	seen := map[geometry.Point]bool{}
	var spaces [][]geometry.Point
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			start := geometry.Point{X: x, Y: y}
			if seen[start] || !open(start) {
				continue
			}
			seen[start] = true
			space := []geometry.Point{start}
			for i := 0; i < len(space); i++ {
				for _, d := range []geometry.Point{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}} {
					if n := space[i].Add(d); n.X >= 0 && n.Y >= 0 && n.X < w && n.Y < h && !seen[n] && open(n) {
						seen[n] = true
						space = append(space, n)
					}
				}
			}
			spaces = append(spaces, space)
		}
	}
	return spaces
}

// dungenDecoration picks one of the decoration levels whose name starts with the prefix the room asked for.
func (g *MapGenerator) dungenDecoration(proj *hhm.Project, prefix string) *hhm.Level {
	candidates := proj.LevelsWithPrefix(prefix)
	if len(candidates) == 0 {
		return nil
	}
	return candidates[g.randomSource.Intn(len(candidates))]
}

func (g *MapGenerator) paintDungenDecoration(m *gridmap.GridMap[*Actor, *Item, Object], proj *hhm.Project, template *hhm.Level, room *dungen.DungeonRoom) {
	for y := 0; y < template.Height; y++ {
		for x := 0; x < template.Width; x++ {
			id := template.TileAt(x, y)
			if id < 0 || id > 223 {
				continue
			}
			p := room.ToAbsolutePosition(geometry.Point{X: x, Y: y})
			if m.IsTileWalkable(p) && room.Contains(p) {
				m.SetTile(p, TileFromDrawing(proj.Tileset, id))
			}
		}
	}
	g.spawnDrawnUniques(m, template.Entities, func(e *hhm.Entity) geometry.Point { return room.ToAbsolutePosition(geometry.Point{X: e.X, Y: e.Y}) })
}

// spawnDrawnUniques places a drawn room's own inhabitants (a decoration's or a set piece's): an NPC entity with a
// UniqueName is the one place that unique turns up.
func (g *MapGenerator) spawnDrawnUniques(m *gridmap.GridMap[*Actor, *Item, Object], entities []*hhm.Entity, at func(*hhm.Entity) geometry.Point) {
	for _, e := range entities {
		name := e.Str("UniqueName")
		if e.Kind != "NPC" || name == "" {
			continue
		}
		p := at(e)
		if a := g.factory.NewMonsterByID(name); a != nil && m.CanPlaceActorHere(p) {
			a.monster.SpawnIndex = -1 // saved whole, like the rest of a dungen level's monsters
			a.SetDrawPosition(p.ToPointF())
			m.AddActor(a, p)
			g.addSpawn(m, a, func() *Actor { return g.factory.NewMonsterByID(name) }, sameName(name))
		}
	}
}

// fsys is the active mod's file system, for the generators that read room templates out of it.
func (g *MapGenerator) fsys() fs.FS { return files.GetFileSystem() }

// maxDepth is the deepest depth any level declares.
func (g *MapGenerator) maxDepth() (d int) {
	for _, l := range g.levels {
		d = max(d, l.Depth)
	}
	return
}

// populateD1 fills a laid-out level by D1's rules for the population's level type (dungen.Populate): its
// monsters, objects, items and theme rooms, rolled from the level's D1 seed. The stairs placed so far are its
// triggers, which D1 keeps clear.
func (g *MapGenerator) populateD1(m *gridmap.GridMap[*Actor, *Item, Object], layout *dungen.DungeonMap, level dungeon.Level, pop dungen.Population, stairs []geometry.Point) {
	msgs := map[string]int{"ladder_up": dungen.TrigPrevLevel, "ladder_down": dungen.TrigNextLevel, "warp": dungen.TrigTownWarpUp}
	var trigs []dungen.Trigger
	for _, p := range stairs {
		msg, ok := msgs[m.GetNamedLocationByPos(p)]
		if !ok {
			msg = dungen.TrigNextLevel // a quest's way in
		}
		trigs = append(trigs, dungen.Trigger{X: p.X + dungen.CellOrigin, Y: p.Y + dungen.CellOrigin, Msg: msg})
	}
	lvl, err := g.factory.Levels.Populate(g.factory.Gen(), g.questTable(), g.levelSeeds(level.Map, level.Depth), level.Depth, pop, layout, trigs)
	if err != nil {
		panic(err)
	}
	if g.generated == nil {
		g.generated = map[string]*dungen.Level{}
	}
	g.generated[level.Map] = lvl
	g.placeMonsters(m, lvl)
	g.placeObjects(m, lvl)
	g.placeItems(m, lvl)
	for _, a := range append(append([]*Actor(nil), m.Actors()...), g.walledIn[level.Map]...) {
		if a.monster != nil {
			g.factory.initTalk(a)
		}
	}
}

// questHere is whether quest key is available on dungeon level dlvl (D1's QuestStatus).
func (g *MapGenerator) questHere(key string, dlvl int) bool {
	q := g.factory.State.QuestByKey(key)
	if q < 0 || g.questTable() == nil || q >= len(g.questTable()) {
		return false
	}
	qu := g.questTable()[q]
	return qu.Level == dlvl && qu.Active != rules.QuestNotAvail
}

// paintPieces draws the level's set pieces over the tile set's walls and floors: each drawn cell as its tile,
// doors left to the layout's door.
func (g *MapGenerator) paintPieces(m *gridmap.GridMap[*Actor, *Item, Object], layout *dungen.DungeonMap) {
	if len(layout.Pieces()) == 0 {
		return
	}
	ts := g.factory.State.drawnMaps.Tileset
	for _, pc := range layout.Pieces() {
		for y := 0; y < pc.Map.Height; y++ {
			for x := 0; x < pc.Map.Width; x++ {
				p := geometry.Point{X: pc.X + x, Y: pc.Y + y}
				if id := pc.Map.TileAt(x, y); id != hhm.Empty && m.Contains(p) && layout.GetTileAt(p) != dungen.Door {
					m.SetTile(p, g.drawnTile(m, ts, id))
				}
			}
		}
	}
}

// drawnTile is a drawn tile as the level shows it: lava is foul water in the Poisoned Water Supply (lavaTile).
func (g *MapGenerator) drawnTile(m *gridmap.GridMap[*Actor, *Item, Object], ts *hhm.Tileset, id int) gridmap.Tile {
	if t := TileFromDrawing(ts, id); t.DefinedIcon != defs.IconLava {
		return t
	}
	return g.lavaTile(m)
}

// tileFromRecord is a tiles.rec record as a map tile: its name, icon and what its tags block.
func tileFromRecord(name string) gridmap.Tile {
	r, ok := defs.TileRecords[name]
	if !ok {
		panic(fmt.Sprintf("levels.rec: decoration: tiles.rec has no %s", name))
	}
	return gridmap.Tile{DefinedIcon: r.Icon, DefinedDescription: r.Name, IsWalkable: !r.Flags.BlocksMovement,
		IsTransparent: !r.Flags.BlocksView, IsMissilePassable: !r.Flags.BlocksMissiles}
}
