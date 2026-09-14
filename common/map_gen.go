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

func (g *MapGenerator) addBasicTiles(emptyMap *gridmap.GridMap[*Actor, *Item, Object], lvl *d1.Level, dlvl int) {
	isWall := func(p geometry.Point) bool {
		return p.X >= 0 && p.Y >= 0 && p.X < d1.LevelSize && p.Y < d1.LevelSize && lvl.Cells[p.X][p.Y] != d1.CellFloor
	}
	tileMap := defs.D1Tilemap(g.randomSource, isWall, dlvl)
	for x := 0; x < d1.LevelSize; x++ {
		for y := 0; y < d1.LevelSize; y++ {
			mapPos := geometry.Point{X: x, Y: y}
			switch lvl.Cells[x][y] {
			case d1.CellWall:
				emptyMap.SetTile(mapPos, gridmap.Tile{DefinedIcon: tileMap.ChooseWallTile(mapPos), DefinedDescription: "a wall"})
			case d1.CellLava:
				emptyMap.SetTile(mapPos, gridmap.Tile{DefinedIcon: defs.IconLava, DefinedDescription: "lava", IsTransparent: true})
			case d1.CellFence:
				emptyMap.SetTile(mapPos, gridmap.Tile{DefinedIcon: defs.IconWoodenFence, DefinedDescription: "a wooden fence", IsTransparent: true})
			case d1.CellDoor:
				emptyMap.SetTile(mapPos, gridmap.Tile{DefinedIcon: tileMap.FloorTile, DefinedDescription: "a floor", IsWalkable: true, IsTransparent: true})
				emptyMap.AddObject(NewDoor(tileMap.OpenDoor, tileMap.ClosedDoor), mapPos)
			default:
				emptyMap.SetTile(mapPos, gridmap.Tile{DefinedIcon: tileMap.ChooseFloorTile(mapPos), DefinedDescription: "a floor", IsWalkable: true, IsTransparent: true})
			}
		}
	}
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
