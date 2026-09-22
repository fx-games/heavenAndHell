package common

import (
	"cmp"
	"fmt"
	"io/fs"
	"math"
	"slices"

	"rpg/files"
	"rpg/src/defs"
	"rpg/src/dungen"
	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/ldtk"
)

// This is the other half of GenerateMap's dispatch: a level whose generator belongs to the dungen family is
// carved as a bare dungen.DungeonMap - walls, floors, doors and rooms, no tile art, no actors - and turned into
// a gridmap here.
//
// D1's side does the same job in map_gen_d1.go and shares nothing with this but the gridmap it writes into,
// which is exactly the seam docs/ENGINE_SPLIT.md describes: the two families meet at "a name resolved and the knobs
// parsed", and each builds its own levels its own way past that point.

// generateDungenMap builds one dungen level and lays it out as a gridmap.
func (g *MapGenerator) generateDungenMap(lvl dungeon.Level) *gridmap.GridMap[*Actor, *Item, Object] {
	targetMap, dlvl := lvl.Map, lvl.Depth
	layout, err := dungen.Build(lvl.Config, g.randomSource, g.fsys())
	if err != nil {
		panic(err)
	}
	width, height := layout.GetSize()
	m := gridmap.NewEmptyMap[*Actor, *Item, Object](width, height)
	m.SetName(targetMap)
	m.SetMetaString(layout.GetMetaInfo())

	g.addDungenTiles(m, layout, g.dungenTilemap(lvl, dlvl, layout))
	stairs := g.addDungenStairs(m, layout, lvl)
	g.decorateDungenRooms(m, layout, lvl, dlvl)
	g.populateDungenLevel(m, layout, dlvl, stairs)

	if snap := g.snapshots[targetMap]; snap != nil {
		snap.apply(g.factory, m)
	}
	return m
}

// dungenTilemap resolves the tile set the level names.
func (g *MapGenerator) dungenTilemap(lvl dungeon.Level, dlvl int, layout *dungen.DungeonMap) defs.BasicTileMapping {
	isWall := func(p geometry.Point) bool { return !layout.Contains(p) || layout.GetTileAt(p) == dungen.Wall }
	if tm, ok := defs.Tilemap(lvl.Tiles, g.randomSource, isWall); ok {
		return tm
	}
	panic(fmt.Sprintf("levels.rec: %s: unknown tile set %q (have %v)", lvl.Map, lvl.Tiles, defs.TilemapNames()))
}

// addDungenTiles paints the layout: a wall cell is a wall, a door cell is floor with a door object standing on
// it (decision 12, the same rule D1's side follows), and everything else is floor.
func (g *MapGenerator) addDungenTiles(m *gridmap.GridMap[*Actor, *Item, Object], layout *dungen.DungeonMap, tileMap defs.BasicTileMapping) {
	wall := func(icon int32) gridmap.Tile {
		return gridmap.Tile{DefinedIcon: icon, DefinedDescription: "a wall"}
	}
	floor := func(icon int32) gridmap.Tile {
		return gridmap.Tile{DefinedIcon: icon, DefinedDescription: "a floor", IsWalkable: true, IsTransparent: true}
	}
	width, height := layout.GetSize()
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			p := geometry.Point{X: x, Y: y}
			switch layout.GetTile(x, y) {
			case dungen.Wall:
				m.SetTile(p, wall(tileMap.ChooseWallTile(p)))
			case dungen.Door:
				m.SetTile(p, floor(tileMap.FloorTile))
				m.AddObject(NewDoor(tileMap.OpenDoor, tileMap.ClosedDoor), p)
			default: // Room, Corridor
				m.SetTile(p, floor(tileMap.ChooseFloorTile(p)))
			}
		}
	}
}

func dungenLadderUp() gridmap.Tile {
	return gridmap.Tile{DefinedIcon: defs.IconStairsUp, DefinedDescription: "a ladder leading up",
		IsWalkable: true, IsTransparent: true, Special: gridmap.SpecialTileStairsUp}
}

func dungenLadderDown() gridmap.Tile {
	return gridmap.Tile{DefinedIcon: defs.IconStairsDown, DefinedDescription: "a ladder leading down",
		IsWalkable: true, IsTransparent: true, Special: gridmap.SpecialTileStairsDown}
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
	entrance := spots[g.randomSource.Intn(len(spots))]
	var rest []dungeon.Exit
	for _, e := range lvl.Exits {
		if e.Location == "ladder_up" {
			put(e, entrance)
		} else {
			rest = append(rest, e)
		}
	}
	for _, e := range rest {
		if len(spots) == 0 {
			println(fmt.Sprintf("ERR: no stair position left for exit %q on %s", e.Location, m.GetName()))
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

// decorateDungenRooms lights some of the rooms and paints the hand-drawn art over the rooms whose template asked
// for it. The art is the room template's own LDtk tiles, laid over the floor the tile set already painted.
func (g *MapGenerator) decorateDungenRooms(m *gridmap.GridMap[*Actor, *Item, Object], layout *dungen.DungeonMap, lvl dungeon.Level, dlvl int) {
	proj, _ := dungen.Templates(lvl.Config, g.fsys()) // nil for a generator that draws no templates

	rooms := layout.AllRooms()
	litWanted := int(math.Ceil(float64(len(rooms)) * min(dungenMaxLit, max(0, 1.0-float64(dlvl)/float64(max(g.maxDepth(), 1))))))
	lit := 0
	for _, i := range g.randomSource.Perm(len(rooms)) {
		room := rooms[i]
		if proj != nil && room.HasDecoratedTemplate() {
			if deco := g.dungenDecoration(proj, room.GetDecorationLevelName()); deco != nil {
				g.paintDungenDecoration(m, proj, deco, room)
				m.AddToMetaInfo("DecoRoom: " + deco.Identifier)
			}
		}
		if lit >= litWanted {
			continue
		}
		lit++
		for _, p := range room.GetAbsoluteRoomTiles() {
			m.SetLit(p, true)
			for _, n := range m.GetFilteredNeighbors(p, m.Contains) {
				m.SetLit(n, true)
			}
		}
	}
}

// dungenDecoration picks one of the decoration levels whose name starts with the prefix the room asked for.
func (g *MapGenerator) dungenDecoration(proj *ldtk.Project, prefix string) *ldtk.Level {
	var candidates []*ldtk.Level
	for _, l := range proj.Levels {
		if len(l.Identifier) >= len(prefix) && l.Identifier[:len(prefix)] == prefix {
			candidates = append(candidates, l)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates[g.randomSource.Intn(len(candidates))]
}

func (g *MapGenerator) paintDungenDecoration(m *gridmap.GridMap[*Actor, *Item, Object], proj *ldtk.Project, template *ldtk.Level, room *dungen.DungeonRoom) {
	layer := template.LayerByIdentifier("Tiles")
	if layer == nil {
		return
	}
	tileSet := proj.TilesetByIdentifier("World")
	for _, tile := range layer.Tiles {
		if tile.ID < 0 || tile.ID > 223 {
			continue
		}
		rel, mapTile := MapTileFromLDTK(tileSet, layer, tile)
		p := room.ToAbsolutePosition(rel)
		if m.IsTileWalkable(p) && room.Contains(p) {
			m.SetTile(p, mapTile)
		}
	}
	// A room's own inhabitants: an NPC entity with a UniqueName is the one place that unique turns up.
	entities := template.LayerByIdentifier("Entities")
	if entities == nil {
		return
	}
	for _, e := range entities.Entities {
		name := e.PropertyByIdentifier("UniqueName")
		if e.Identifier != "NPC" || name.IsNull() {
			continue
		}
		x, y := entities.ToGridPosition(e.Position[0], e.Position[1])
		p := room.ToAbsolutePosition(geometry.Point{X: x, Y: y})
		if a := g.factory.NewMonsterByID(name.AsString()); a != nil && m.CanPlaceActorHere(p) {
			a.d1Monster.SpawnIndex = -1 // saved whole, like the rest of a dungen level's monsters
			a.SetDrawPosition(p.ToPointF())
			m.AddActor(a, p)
			id := name.AsString()
			g.addSpawn(m, a, func() *Actor { return g.factory.NewMonsterByID(id) }, sameName(id))
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
