package common

import (
	"math"
	"math/rand"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/ldtk"
	"strconv"
	"strings"
)

type MapGenerator struct {
	randomSource *rand.Rand
	factory      *Factory
	seed         int64
	snapshots    map[string]*LevelSnapshot
	d1Seeds      [d1.NumLevels]int32
	d1Quests     *d1.Quests
	levels       map[string]*d1.Level // the generated D1 levels by map name, for changeD1Map
	walledIn     map[string][]*Actor  // monsters D1 placed inside walls that a map change opens later, by map name
}

func NewMapGenerator(randomSource *rand.Rand) *MapGenerator {
	return &MapGenerator{
		randomSource: randomSource,
	}
}
func (g *MapGenerator) SetFactory(factory *Factory) {
	g.factory = factory
}

func (g *MapGenerator) GenerateMap(targetMap string, dungeonLevel int) *gridmap.GridMap[*Actor, *Item, Object] {
	return g.generateD1Map(targetMap, dungeonLevel)
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

// d1DoorKinds are the door object kinds (decision 12: "Doors stay H&H doors" - the terrain cell is plain floor,
// an H&H Door object sits on it and swaps its own icon open/closed).
var d1DoorKinds = map[int]bool{d1.ObjL1LDOOR: true, d1.ObjL1RDOOR: true, d1.ObjL2LDOOR: true, d1.ObjL2RDOOR: true,
	d1.ObjL3LDOOR: true, d1.ObjL3RDOOR: true}

// d1LavaRange/d1FenceRange are the raw D1 megatile numbers (Level.Dungeon, before lPass3) that River/PoolFix/Fence
// paint for Caves lava and wooden fences; walls, floors and everything else fall outside them.
func d1IsLavaMega(v byte) bool  { return v >= 15 && v <= 45 }
func d1IsFenceMega(v byte) bool { return v >= 121 && v <= 153 }

// d1CathedralSeeThrough is what the Cathedral megatiles with pieces D1's sight passes are drawn as: arches, fences,
// the pillar and the lamp (drlg_l1_data.go's t1*Arch*, t1*Fence*, t1Pillar, l1LAMPS).
var d1CathedralSeeThrough = func() map[byte]gridmap.Tile {
	arch := gridmap.Tile{DefinedIcon: defs.IconArch, DefinedDescription: "an arch", IsTransparent: true}
	grate := gridmap.Tile{DefinedIcon: defs.IconGrate, DefinedDescription: "a grate", IsTransparent: true}
	return map[byte]gridmap.Tile{5: arch, 8: arch, 9: arch, 10: arch, 11: arch, 12: arch, 14: arch, 33: arch, 34: arch,
		39: arch, 40: arch, 42: arch, 147: arch, 149: arch, 153: arch,
		27: grate, 29: grate, 32: grate, 35: grate, 36: grate, 37: grate, 38: grate,
		15:  {DefinedIcon: defs.IconPillar, DefinedDescription: "a pillar", IsTransparent: true},
		128: {DefinedIcon: defs.IconLamp, DefinedDescription: "a lamp", IsTransparent: true}}
}()

// d1CathedralColumns are Cathedral megatiles whose art is a free-standing column, exactly like the arch pieces
// that stand next to them in an arcade, but whose cell blocks sight in D1's own SOL table - so they fell through
// to the plain wall icon and a row of arches ended in a wall. t1Corner (3) is the one that shows: an arcade
// running down a wall corner uses it, while the same arcade mirrored uses t1HArchEnd (8), which does draw as an
// arch. They keep blocking sight, only the icon changes.
// ponytail: t1VCorner (16) and t1HCorner (17) look like columns too, but they cap wall runs rather than arcades,
// so they stay walls; move them here if wall ends should look like arches as well.
var d1CathedralColumns = map[byte]bool{3: true}

// addBasicTiles ports the wall/floor/lava/fence/door terrain from a generated d1.Level (piece coordinates 16..95,
// exposed as cells 0..79) onto the gridmap. Doors are placed as objects afterward, once Level.Objects is walked.
// d1TypeDlvl maps a level TYPE back to a representative dlvl for D1Tilemap, since a set level's own type (Lazarus'
// Lair is Cathedral-typed Vile1/2.DUN, drawn with L1 tiles) can differ from the dungeon depth it's reached from.
var d1TypeDlvl = map[int]int{d1.DTypeCathedral: 1, d1.DTypeCatacombs: 5, d1.DTypeCaves: 9, d1.DTypeHell: 13}

func (g *MapGenerator) addBasicTiles(emptyMap *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level) {
	doorAt := map[geometry.Point]bool{}
	for _, o := range lvl.Objects {
		if d1DoorKinds[o.Kind] {
			doorAt[geometry.Point{X: o.X - d1.CellOrigin, Y: o.Y - d1.CellOrigin}] = true
		}
	}
	g.setD1Tiles(emptyMap, lvl, 0, 0, d1.LevelSize-1, d1.LevelSize-1, func(p geometry.Point) bool { return doorAt[p] })
}

// setD1Tiles draws cells (x1, y1)..(x2, y2) from the level, with an H&H door where door says so.
func (g *MapGenerator) setD1Tiles(m *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level, x1, y1, x2, y2 int, door func(geometry.Point) bool) {
	isWall := func(p geometry.Point) bool {
		if p.X < 0 || p.Y < 0 || p.X >= d1.LevelSize || p.Y >= d1.LevelSize {
			return true
		}
		return lvl.CellSolid(p.X, p.Y) && !door(p)
	}
	tileMap := defs.D1Tilemap(g.randomSource, isWall, d1TypeDlvl[lvl.Type])
	for x := max(x1, 0); x <= min(x2, d1.LevelSize-1); x++ {
		for y := max(y1, 0); y <= min(y2, d1.LevelSize-1); y++ {
			mapPos := geometry.Point{X: x, Y: y}
			if _, ok := m.GetTransitionAt(mapPos); ok { // a map change keeps the stairs wireD1Triggers put there
				continue
			}
			seeThroughTile, seeThrough := d1CathedralSeeThrough[lvl.CellMega(x, y)]
			switch {
			case door(mapPos):
				m.SetTile(mapPos, gridmap.Tile{DefinedIcon: tileMap.FloorTile, DefinedDescription: "a floor", IsWalkable: true, IsTransparent: true})
				if !m.IsObjectAt(mapPos) {
					m.AddObject(NewDoor(tileMap.OpenDoor, tileMap.ClosedDoor), mapPos)
				}
			case lvl.Type == d1.DTypeCaves && d1IsLavaMega(lvl.CellMega(x, y)) && lvl.CellSolid(x, y) && !lvl.CellBlocks(x, y):
				m.SetTile(mapPos, g.d1LavaTile(m))
			case lvl.Type == d1.DTypeCaves && d1IsFenceMega(lvl.CellMega(x, y)) && lvl.CellSolid(x, y) && !lvl.CellBlocks(x, y):
				m.SetTile(mapPos, gridmap.Tile{DefinedIcon: defs.IconWoodenFence, DefinedDescription: "a wooden fence", IsTransparent: true})
			case lvl.Type == d1.DTypeCathedral && seeThrough && lvl.CellSolid(x, y) && !lvl.CellBlocks(x, y):
				m.SetTile(mapPos, seeThroughTile)
			case lvl.Type == d1.DTypeCathedral && d1CathedralColumns[lvl.CellMega(x, y)] && lvl.CellSolid(x, y):
				m.SetTile(mapPos, gridmap.Tile{DefinedIcon: defs.IconArch, DefinedDescription: "an arch"})
			case lvl.CellSolid(x, y): // opaque unless drawn as something see-through: a see-through plain wall hides that you
				// can see and shoot into a room
				m.SetTile(mapPos, gridmap.Tile{DefinedIcon: tileMap.ChooseWallTile(mapPos), DefinedDescription: "a wall"})
			default:
				m.SetTile(mapPos, gridmap.Tile{DefinedIcon: tileMap.ChooseFloorTile(mapPos), DefinedDescription: "a floor", IsWalkable: true, IsTransparent: true})
			}
		}
	}
}

var (
	d1Lava          = gridmap.Tile{DefinedIcon: defs.IconLava, DefinedDescription: "lava", IsTransparent: true}
	d1PoisonedWater = gridmap.Tile{DefinedIcon: defs.IconWater, DefinedDescription: "poisoned water", IsTransparent: true, Special: gridmap.SpecialTileWater}
	d1CleanWater    = gridmap.Tile{DefinedIcon: defs.IconWater, DefinedDescription: "clean water", IsTransparent: true, Special: gridmap.SpecialTileWater}
)

// d1LavaTile is what a cave's lava pieces show: lava, except in the Poisoned Water Supply, where D1's palette
// (L3pfoul.pal, then L3pwater.pal once the quest is done) turns them into foul water and then clean water, tinted
// by State.GetWaterColor.
func (g *MapGenerator) d1LavaTile(m *gridmap.GridMap[*Actor, *Item, Object]) gridmap.Tile {
	if sl := d1SetLevelOf(m.GetName()); sl == nil || sl.SL != d1.SLPoisonWater {
		return d1Lava
	}
	if g.factory.State.QuestActive(d1.QPWater) == d1.QuestDone {
		return d1CleanWater
	}
	return d1PoisonedWater
}

// cleanD1Water ports the Poisoned Water Supply's palette change once the quest is done.
func (g *MapGenerator) cleanD1Water(m *gridmap.GridMap[*Actor, *Item, Object]) {
	for y := 0; y < d1.LevelSize; y++ {
		for x := 0; x < d1.LevelSize; x++ {
			if p := (geometry.Point{X: x, Y: y}); m.GetCell(p).TileType.DefinedDescription == d1PoisonedWater.DefinedDescription {
				m.SetTile(p, d1CleanWater)
			}
		}
	}
}

// changeD1Map ports ObjChangeMap on a built map: megatiles (x1, y1)..(x2, y2) switch to the level's alternate
// layout and are redrawn, with the doors AddL1Objs/AddL2Objs add. Nothing happens if they already switched.
func (g *MapGenerator) changeD1Map(m *gridmap.GridMap[*Actor, *Item, Object], x1, y1, x2, y2 int) {
	lvl := g.levels[m.GetName()]
	if lvl == nil || !lvl.ChangeMap(x1, y1, x2, y2) {
		return
	}
	g.setD1Tiles(m, lvl, 2*x1, 2*y1, 2*x2+1, 2*y2+1, func(p geometry.Point) bool {
		if m.IsObjectAt(p) { // a door built before the change stays a door, not a wall under its object
			_, isDoor := m.ObjectAt(p).(*Door)
			return isDoor
		}
		return lvl.CellDoor(p.X, p.Y)
	})
	waiting := g.walledIn[m.GetName()][:0]
	for _, a := range g.walledIn[m.GetName()] {
		if p := a.MapPosition(); canPlaceD1Monster(m, p) {
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

func LevelFromName(targetMap string) int {
	nameParts := strings.Split(targetMap, "_")
	level, _ := strconv.Atoi(nameParts[len(nameParts)-1])
	return level
}

func MapTileFromLDTK(tileSet *ldtk.Tileset, layer *ldtk.Layer, tile *ldtk.Tile) (geometry.Point, gridmap.Tile) {
	posX, posY := layer.ToGridPosition(tile.Position[0], tile.Position[1])
	pos := geometry.Point{X: posX, Y: posY}
	enums := tileSet.EnumsForTile(tile.ID)

	walkable := !enums.Contains("IsBlockingMovement")
	transparent := !enums.Contains("IsBlockingView")
	isDamaging := enums.Contains("IsDamaging")
	isWater := enums.Contains("IsWater")
	specialTile := gridmap.SpecialTileNone

	if isWater {
		specialTile = gridmap.SpecialTileWater
	} else if isDamaging {
		specialTile = gridmap.SpecialTileDamaging
	}

	tileType := gridmap.Tile{
		DefinedIcon:   int32(tile.ID),
		IsWalkable:    walkable,
		IsTransparent: transparent,
		Special:       specialTile,
	}
	return pos, tileType
}
