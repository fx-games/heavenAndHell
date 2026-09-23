package common

import (
	"fmt"
	"math"
	"math/rand"
	"path"
	"slices"

	"rpg/files"
	"rpg/src/defs"
	"rpg/src/dungen"
	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/hhm"
	"rpg/src/recfile"
	"rpg/src/rules"
)

type MapGenerator struct {
	randomSource *rand.Rand
	factory      *Factory
	seed         int64
	snapshots    map[string]*LevelSnapshot
	seeds        [rules.NumLevels]int32
	quests       *rules.Quests
	generated    map[string]*dungen.Level // the generated D1 levels by map name, for changeMap
	walledIn     map[string][]*Actor      // monsters D1 placed inside walls that a map change opens later, by map name
	levels       map[string]dungeon.Level // the mod's levels.rec, by map id
	levelOrder   []string                 // their ids in file order
	rosters      map[string]*roster       // where each level's generator placed its monsters, for Respawn (respawn.go)
	generations  map[string]int           // how often each level was forgotten (PersistLevels: false), saved
	doorTiles    map[string][3]int32      // each dungen level's floor, open and closed door tiles, for its secret doors
}

// LoadLevels reads the active mod's levels.rec (docs/LEVELS.md). Called after mod selection, never at package init:
// the whole table is content.
func (g *MapGenerator) LoadLevels() error {
	f, err := files.Open(path.Join("assets", "data", "levels.rec"))
	if err != nil {
		return fmt.Errorf("levels.rec: %w", err)
	}
	defer f.Close()
	levels, err := dungeon.LoadLevels(recfile.Read(f), lookupStrategy)
	if err != nil {
		return err
	}
	for _, l := range levels {
		_, drawn := l.Config.(drawnConfig)
		if l.Tiles == "drawn" && !drawn || l.Tiles != "drawn" && !slices.Contains(defs.TilemapNames(), l.Tiles) {
			return fmt.Errorf("levels.rec: %s: tiles %q: want one of %v, or drawn for a drawn level", l.Map, l.Tiles, defs.TilemapNames())
		}
		if err := g.factory.checkPatches(l); err != nil {
			return err
		}
		for _, e := range l.Exits {
			if _, err := compileQuestCondition(e.OpenWhen); e.OpenWhen != "" && err != nil {
				return fmt.Errorf("levels.rec: %s: exit %s: open_when: %w", l.Map, e.Location, err)
			}
		}
	}
	g.SetLevels(levels)
	return nil
}

// SetLevels is LoadLevels for a caller that already has the table (tests, and any tool that builds one level).
func (g *MapGenerator) SetLevels(levels []dungeon.Level) {
	dungen.ReserveRequiredRooms(levels)
	g.levels, g.levelOrder = map[string]dungeon.Level{}, nil
	for _, l := range levels {
		g.levels[l.Map] = l
		g.levelOrder = append(g.levelOrder, l.Map)
	}
}

// Levels is the mod's level list in file order.
func (g *MapGenerator) Levels() []dungeon.Level {
	out := make([]dungeon.Level, 0, len(g.levelOrder))
	for _, id := range g.levelOrder {
		out = append(out, g.levels[id])
	}
	return out
}

func NewMapGenerator(randomSource *rand.Rand) *MapGenerator {
	return &MapGenerator{
		randomSource: randomSource,
	}
}
func (g *MapGenerator) SetFactory(factory *Factory) {
	g.factory = factory
}

// GenerateMap builds level id with the strategy its record names, then lays its declared exits on it.
func (g *MapGenerator) GenerateMap(id string) *gridmap.GridMap[*Actor, *Item, Object] {
	lvl, ok := g.levels[id]
	if !ok {
		panic(fmt.Sprintf("levels.rec declares no level %q", id))
	}
	// the level's own random source, so a load rebuilds what was saved (respawn.go)
	shared := g.randomSource
	g.randomSource = g.levelRand(id)
	defer func() { g.randomSource = shared }()
	delete(g.rosters, id)
	m := levelStrategies[lvl.Generator].Generate(g, lvl)
	if lvl.Name != "" {
		m.SetDisplayName(lvl.Name)
	}
	g.factory.State.wireExits(m)
	return m
}

// LevelStrategy is how a level gets built: every generator a mod may name in levels.rec is one, drawn, D1's and
// dungen's alike. Parse reads its knobs when the mod loads, so a bad levels.rec fails then and not on the stairs
// down; Generate builds the playable level from them.
type LevelStrategy interface {
	dungeon.Parser
	Generate(g *MapGenerator, lvl dungeon.Level) *gridmap.GridMap[*Actor, *Item, Object]
}

// levelStrategies are all of them, by the name levels.rec's generator: field uses.
var levelStrategies = map[string]LevelStrategy{"drawn": drawnStrategy{}, "d1_set": d1SetStrategy{dungen.D1Set}}

func init() {
	for name, p := range dungen.D1 {
		levelStrategies[name] = d1Strategy{p}
	}
	for name, p := range dungen.Layouts {
		levelStrategies[name] = layoutStrategy{p}
	}
}

func lookupStrategy(name string) (dungeon.Parser, bool) {
	s, ok := levelStrategies[name]
	return s, ok
}

// drawnConfig is the "drawn" generator's: a level drawn by hand in the mod's map set, under Template.
type drawnConfig struct{ Template string }

type drawnStrategy struct{}

func (drawnStrategy) Parse(r recfile.Record) (dungeon.Config, error) {
	t, ok := dungeon.Field(r, "template")
	if !ok || t == "" {
		return nil, fmt.Errorf("levels.rec: drawn needs a template (the map file's name)")
	}
	return drawnConfig{Template: t}, nil
}

func (drawnStrategy) Generate(g *MapGenerator, lvl dungeon.Level) *gridmap.GridMap[*Actor, *Item, Object] {
	return g.factory.State.loadDrawnLevel(lvl, lvl.Config.(drawnConfig).Template)
}

func minDist(locations []geometry.Point, pos geometry.Point) float64 {
	minimumDist := math.MaxFloat64
	for _, loc := range locations {
		dist := float64(loc.ChebyshevDistanceTo(pos))
		if dist < minimumDist {
			minimumDist = dist
		}
	}
	return minimumDist
}

// doorKinds are the door object kinds (decision 12: "Doors stay H&H doors" - the terrain cell is plain floor,
// an H&H Door object sits on it and swaps its own icon open/closed).
var doorKinds = map[int]bool{rules.ObjL1LDOOR: true, rules.ObjL1RDOOR: true, rules.ObjL2LDOOR: true, rules.ObjL2RDOOR: true,
	rules.ObjL3LDOOR: true, rules.ObjL3RDOOR: true}

// addBasicTiles ports the wall/floor/lava/fence/door terrain from a generated dungen.Level (piece coordinates 16..95,
// exposed as cells 0..79) onto the gridmap. Doors are placed as objects afterward, once Level.Objects is walked.

func (g *MapGenerator) addBasicTiles(emptyMap *gridmap.GridMap[*Actor, *Item, Object], lvl *dungen.Level) {
	doorAt := map[geometry.Point]bool{}
	for _, o := range lvl.Objects {
		if doorKinds[o.Kind] {
			doorAt[geometry.Point{X: o.X - dungen.CellOrigin, Y: o.Y - dungen.CellOrigin}] = true
		}
	}
	g.setTiles(emptyMap, lvl, 0, 0, dungen.LevelSize-1, dungen.LevelSize-1, func(p geometry.Point) bool { return doorAt[p] })
}

// setTiles draws cells (x1, y1)..(x2, y2) from the level, with an H&H door where door says so.
func (g *MapGenerator) setTiles(m *gridmap.GridMap[*Actor, *Item, Object], lvl *dungen.Level, x1, y1, x2, y2 int, door func(geometry.Point) bool) {
	isWall := func(p geometry.Point) bool {
		if p.X < 0 || p.Y < 0 || p.X >= dungen.LevelSize || p.Y >= dungen.LevelSize {
			return true
		}
		return lvl.CellSolid(p.X, p.Y) && !door(p)
	}
	tileMap, ok := defs.Tilemap(g.levels[m.GetName()].Tiles, g.randomSource, isWall)
	if !ok {
		panic(fmt.Sprintf("levels.rec: %s: unknown tile set %q (have %v)", m.GetName(), g.levels[m.GetName()].Tiles, defs.TilemapNames()))
	}
	tiles := g.factory.State.drawnMaps.Tileset
	for x := max(x1, 0); x <= min(x2, dungen.LevelSize-1); x++ {
		for y := max(y1, 0); y <= min(y2, dungen.LevelSize-1); y++ {
			mapPos := geometry.Point{X: x, Y: y}
			if _, ok := m.GetTransitionAt(mapPos); ok { // a map change keeps the stairs wireD1Triggers put there
				continue
			}
			if door(mapPos) {
				m.SetTile(mapPos, gridmap.Tile{DefinedIcon: tileMap.FloorTile, DefinedDescription: "a floor", IsWalkable: true, IsTransparent: true, IsMissilePassable: true})
				if !m.IsObjectAt(mapPos) {
					m.AddObject(NewDoor(tileMap.OpenDoor, tileMap.ClosedDoor), mapPos)
				}
				continue
			}
			// the cell as its megatile's map part draws it; plain walls and floors take the tile set's variations
			t := TileFromDrawing(tiles, lvl.CellTile(x, y))
			switch t.DefinedIcon {
			case defs.IconLava:
				t = g.lavaTile(m)
			case tileMap.WallTile:
				t.DefinedIcon = tileMap.ChooseWallTile(mapPos)
			case tileMap.FloorTile:
				t.DefinedIcon = tileMap.ChooseFloorTile(mapPos)
			}
			m.SetTile(mapPos, t)
		}
	}
}

func lava() gridmap.Tile {
	return gridmap.Tile{DefinedIcon: defs.IconLava, DefinedDescription: "lava", IsTransparent: true, IsMissilePassable: true}
}
func poisonedWater() gridmap.Tile {
	return gridmap.Tile{DefinedIcon: defs.IconWater, DefinedDescription: "poisoned water", IsTransparent: true, IsMissilePassable: true, Special: gridmap.SpecialTileWater}
}
func cleanWater() gridmap.Tile {
	return gridmap.Tile{DefinedIcon: defs.IconWater, DefinedDescription: "clean water", IsTransparent: true, IsMissilePassable: true, Special: gridmap.SpecialTileWater}
}

// lavaTile is what a cave's lava pieces show: lava, except in the Poisoned Water Supply, where D1's palette
// (L3pfoul.pal, then L3pwater.pal once the quest is done) turns them into foul water and then clean water, tinted
// by State.GetWaterColor.
func (g *MapGenerator) lavaTile(m *gridmap.GridMap[*Actor, *Item, Object]) gridmap.Tile {
	if !g.factory.State.onSetLevelOf(m, rules.QPWater) {
		return lava()
	}
	if g.factory.State.QuestActive(rules.QPWater) == rules.QuestDone {
		return cleanWater()
	}
	return poisonedWater()
}

// cleanWater ports the Poisoned Water Supply's palette change once the quest is done.
func (g *MapGenerator) cleanWater(m *gridmap.GridMap[*Actor, *Item, Object]) {
	for y := 0; y < dungen.LevelSize; y++ {
		for x := 0; x < dungen.LevelSize; x++ {
			if p := (geometry.Point{X: x, Y: y}); m.GetCell(p).TileType.DefinedDescription == poisonedWater().DefinedDescription {
				m.SetTile(p, cleanWater())
			}
		}
	}
}

// changeMap ports ObjChangeMap on a built map: megatiles (x1, y1)..(x2, y2) switch to the level's alternate
// layout and are redrawn, with the doors AddL1Objs/AddL2Objs add. Nothing happens if they already switched.
func (g *MapGenerator) changeMap(m *gridmap.GridMap[*Actor, *Item, Object], x1, y1, x2, y2 int) {
	lvl := g.generated[m.GetName()]
	if lvl == nil || !lvl.ChangeMap(x1, y1, x2, y2) {
		return
	}
	g.setTiles(m, lvl, 2*x1, 2*y1, 2*x2+1, 2*y2+1, func(p geometry.Point) bool {
		if obj, ok := m.TryGetObjectAt(p); ok { // a door built before the change stays a door, not a wall under its object
			_, isDoor := obj.(*Door)
			return isDoor
		}
		return lvl.CellDoor(p.X, p.Y)
	})
	waiting := g.walledIn[m.GetName()][:0]
	for _, a := range g.walledIn[m.GetName()] {
		if p := a.MapPosition(); canPlaceMonster(m, p) {
			m.AddActor(a, p)
			if g.factory.State.GetMap() == m { // a level being built gets its actors timed on arrival
				g.factory.State.OnActorSpawned(a)
			}
		} else {
			waiting = append(waiting, a)
		}
	}
	g.walledIn[m.GetName()] = waiting
}

// TileFromDrawing turns a drawn tile into the map tile it means: its tiles.rec record's name, icon and tags when
// the tileset says it stands for one, else the tileset's markers on the bare picture.
func TileFromDrawing(ts *hhm.Tileset, id int) gridmap.Tile {
	t := hhm.DrawnTile(ts, defs.TileRecords, id)
	special := gridmap.SpecialTileNone
	if t.Flags.Water {
		special = gridmap.SpecialTileWater
	} else if t.Flags.Damaging {
		special = gridmap.SpecialTileDamaging
	}
	return gridmap.Tile{
		DefinedIcon:        t.Icon,
		DefinedDescription: t.Name,
		IsWalkable:         !t.Flags.BlocksMovement,
		IsTransparent:      !t.Flags.BlocksView,
		IsMissilePassable:  !t.Flags.BlocksMissiles,
		Special:            special,
	}
}
