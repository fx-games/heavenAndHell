package common

import (
	"cmp"
	"fmt"
	"io/fs"
	"math"
	"slices"

	"rpg/defs"
	"rpg/dungen"
	"rpg/dungeon"
	"rpg/files"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/ldtk"
)

// This is the other half of GenerateMap's dispatch: a level whose generator belongs to the dungen family is
// carved as a bare dungen.DungeonMap - walls, floors, doors and rooms, no tile art, no actors - and turned into
// a gridmap here.
//
// D1's side does the same job in map_gen_d1.go and shares nothing with this but the gridmap it writes into,
// which is exactly the seam ENGINE_SPLIT.md describes: the two families meet at "a name resolved and the knobs
// parsed", and each builds its own levels its own way past that point.

// generateDungenMap builds one dungen level and lays it out as a gridmap.
func (g *MapGenerator) generateDungenMap(targetMap string, dlvl int, lvl dungeon.Level) *gridmap.GridMap[*Actor, *Item, Object] {
	layout, err := dungen.Build(lvl.Config, g.randomSource, g.fsys())
	if err != nil {
		panic(err)
	}
	width, height := layout.GetSize()
	m := gridmap.NewEmptyMap[*Actor, *Item, Object](width, height)
	m.SetName(targetMap)
	m.SetMetaString(layout.GetMetaInfo())
	m.SetDisplayName(g.dungenLevelName(lvl, dlvl))

	g.addDungenTiles(m, layout, g.dungenTilemap(lvl, dlvl, layout))
	g.addDungenStairs(m, layout, lvl, dlvl)
	g.decorateDungenRooms(m, layout, lvl, dlvl)

	if snap := g.snapshots[targetMap]; snap != nil {
		snap.apply(g.factory, m)
	}
	return m
}

// dungenLevelName is what the level calls itself on arrival: what the mod declared, or a bare depth if it
// declared nothing, because the engine has no business inventing a theme name for someone else's game.
func (g *MapGenerator) dungenLevelName(lvl dungeon.Level, dlvl int) string {
	if lvl.Name != "" {
		return lvl.Name
	}
	return fmt.Sprintf("Level %d", dlvl)
}

// dungenTilemap resolves the tile set the mod named for this level. A level that names none falls back to the
// depth-keyed pick, so a levels.rec with nothing but generators still draws something sensible.
func (g *MapGenerator) dungenTilemap(lvl dungeon.Level, dlvl int, layout *dungen.DungeonMap) defs.BasicTileMapping {
	isWall := func(p geometry.Point) bool { return !layout.Contains(p) || layout.GetTileAt(p) == dungen.Wall }
	if lvl.Tiles != "" {
		if tm, ok := defs.Tilemap(lvl.Tiles, g.randomSource, isWall); ok {
			return tm
		}
		panic(fmt.Sprintf("levels.rec: dlvl %d: unknown tile set %q (have %v)", dlvl, lvl.Tiles, defs.TilemapNames()))
	}
	return defs.D1Tilemap(g.randomSource, isWall, dlvl)
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

var (
	dungenLadderUp = gridmap.Tile{DefinedIcon: defs.IconStairsUp, DefinedDescription: "a ladder leading up",
		IsWalkable: true, IsTransparent: true, Special: gridmap.SpecialTileStairsUp}
	dungenLadderDown = gridmap.Tile{DefinedIcon: defs.IconStairsDown, DefinedDescription: "a ladder leading down",
		IsWalkable: true, IsTransparent: true, Special: gridmap.SpecialTileStairsDown}
)

// addDungenStairs puts the way up and the way down on the level, plus whatever extra transitions the mod
// declared for it, and returns where they all ended up (monsters keep their distance from them).
//
// The deepest level a mod declared gets no way down: the engine does not know how deep a game goes, so it asks
// the level table rather than assuming a next level exists.
func (g *MapGenerator) addDungenStairs(m *gridmap.GridMap[*Actor, *Item, Object], layout *dungen.DungeonMap, lvl dungeon.Level, dlvl int) []geometry.Point {
	spots := layout.GetPotentialStairPositions()
	if len(spots) < 2 {
		// A level too cramped for two stair spots would be a level you cannot leave. Rather than crash on a bad
		// seed, fall back to any two walkable cells far apart.
		spots = append(spots, g.dungenFallbackStairs(m, layout)...)
	}
	var placed []geometry.Point

	entrance := spots[g.randomSource.Intn(len(spots))]
	spots = removePoint(spots, entrance)
	slices.SortStableFunc(spots, func(a, b geometry.Point) int {
		return cmp.Compare(a.ManhattanDistanceTo(entrance), b.ManhattanDistanceTo(entrance))
	})
	exit := spots[len(spots)-1]
	if g.randomSource.Intn(2) == 1 {
		exit = spots[g.randomSource.Intn(len(spots))]
	}
	spots = removePoint(spots, exit)

	m.SetTile(entrance, dungenLadderUp)
	m.AddNamedLocation("ladder_up", entrance)
	up := gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl - 1), TargetLocation: "ladder_down"}
	if dlvl <= 1 {
		up = gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "to_dungeon"}
	}
	m.AddTransitionAt(entrance, up)
	placed = append(placed, entrance)

	if dlvl+1 < len(g.levels) && g.levels[dlvl+1].Generator != "" {
		m.SetTile(exit, dungenLadderDown)
		m.AddNamedLocation("ladder_down", exit)
		m.AddTransitionAt(exit, gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl + 1), TargetLocation: "ladder_up"})
		placed = append(placed, exit)
	} else {
		spots = append(spots, exit) // the bottom of the dungeon: that spot is free for a declared transition
	}

	// The extra ways off this level go on the spots furthest from the two ladders, so a shortcut back to town
	// does not open next to the stairs you came down.
	slices.SortStableFunc(spots, func(a, b geometry.Point) int {
		return cmp.Compare(minDist(placed, a), minDist(placed, b))
	})
	for _, t := range lvl.Transitions {
		if len(spots) == 0 {
			println(fmt.Sprintf("ERR: no stair position left for transition %q on %s", t.LocationName, m.GetName()))
			break
		}
		p := spots[len(spots)-1]
		spots = spots[:len(spots)-1]
		tile := dungenLadderDown
		if t.Up {
			tile = dungenLadderUp
		}
		m.SetTile(p, tile)
		m.AddNamedLocation(t.LocationName, p)
		m.AddTransitionAt(p, gridmap.Transition{TargetMap: t.TargetMap, TargetLocation: t.TargetLocation})
		placed = append(placed, p)
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
	litWanted := int(math.Ceil(float64(len(rooms)) * min(dungenMaxLit, max(0, 1.0-float64(dlvl)/float64(max(len(g.levels)-1, 1))))))
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
}

// fsys is the active mod's file system, for the generators that read room templates out of it.
func (g *MapGenerator) fsys() fs.FS { return files.GetFileSystem() }
