package defs

import (
	"fmt"
	"math/rand"
	"sort"
	"rpg/geometry"
)

type LevelName string

const LevelNameTown LevelName = "Town"

func IsTownLevel(levelName string) bool {
	return levelName == string(LevelNameTown)
}

type BasicTileMapping struct {
	FloorTile       int32
	ChooseWallTile  func(mapPos geometry.Point) int32
	ChooseFloorTile func(mapPos geometry.Point) int32
	ClosedDoor      int32
	OpenDoor        int32
}

func getHellTilemap(randomSource *rand.Rand, isWall func(geometry.Point) bool) BasicTileMapping {
	chooseFloorTile := func(mapPos geometry.Point) int32 {
		//roll := randomSource.Intn(20)
		return IconDungeonRedEarthFloor
	}
	chooseWallTile := func(mapPos geometry.Point) int32 {
		return IconDungeonRockyWall
	}

	return BasicTileMapping{
		FloorTile:       IconDungeonRedEarthFloor,
		ChooseWallTile:  chooseWallTile,
		ChooseFloorTile: chooseFloorTile,
		OpenDoor:        IconOpenDoor,
		ClosedDoor:      IconClosedDoor,
	}
}

func getCavesTilemap(randomSource *rand.Rand, isWall func(geometry.Point) bool) BasicTileMapping {
	chooseFloorTile := func(mapPos geometry.Point) int32 {
		//roll := randomSource.Intn(20)
		return IconDirt
	}
	chooseWallTile := func(mapPos geometry.Point) int32 {
		return IconMountain
	}

	return BasicTileMapping{
		FloorTile:       IconDirt,
		ChooseWallTile:  chooseWallTile,
		ChooseFloorTile: chooseFloorTile,
		OpenDoor:        IconOpenDoor,
		ClosedDoor:      IconClosedDoor,
	}
}
func getCathedralTilemap(randomSource *rand.Rand, isWall func(geometry.Point) bool) BasicTileMapping {
	chooseFloorTile := func(mapPos geometry.Point) int32 {
		roll := randomSource.Intn(20)
		if roll == 0 {
			brokenFloorTiles := []int32{65, 66, 67, 68}
			randomTile := brokenFloorTiles[randomSource.Intn(len(brokenFloorTiles))]
			return randomTile
		} else {
			return IconCathedralFloor
		}
	}
	chooseWallTile := func(mapPos geometry.Point) int32 {
		// if the tile below is a wall, we use the normal wall tile
		if isWall(mapPos.Add(geometry.Point{X: 0, Y: 1})) {
			return IconCathedralWall
		}
		return IconCathedralWallWithShading
	}

	return BasicTileMapping{
		FloorTile:       IconCathedralFloor,
		ChooseWallTile:  chooseWallTile,
		ChooseFloorTile: chooseFloorTile,
		OpenDoor:        IconOpenDoor,
		ClosedDoor:      IconClosedDoor,
	}
}

func getCatacombsTilemap(randomSource *rand.Rand, isWall func(geometry.Point) bool) BasicTileMapping {
	chooseFloorTile := func(mapPos geometry.Point) int32 {
		return IconCatacombsFloor
	}
	chooseWallTile := func(mapPos geometry.Point) int32 {
		if isWall(mapPos.Add(geometry.Point{X: 0, Y: 1})) {
			return IconCatacombsWall
		}
		return IconCatacombsWallWithShading
	}

	return BasicTileMapping{
		FloorTile:       IconCatacombsFloor,
		ChooseWallTile:  chooseWallTile,
		ChooseFloorTile: chooseFloorTile,
		OpenDoor:        IconOpenDoor,
		ClosedDoor:      IconClosedDoor,
	}
}

// tilemaps are the engine's named tile sets: the look a mod picks for a level with "tiles:" in levels.rec.
// They are engine code because they are drawing rules (which wall sprite a wall gets, given its neighbours),
// not content; which level wears which is the mod's choice.
var tilemaps = map[string]func(*rand.Rand, func(geometry.Point) bool) BasicTileMapping{
	"cathedral": getCathedralTilemap,
	"catacombs": getCatacombsTilemap,
	"caves":     getCavesTilemap,
	"hell":      getHellTilemap,
}

// TilemapNames lists every tile set a mod may name, sorted, for the error a typo in levels.rec earns.
func TilemapNames() []string {
	out := make([]string, 0, len(tilemaps))
	for n := range tilemaps {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Tilemap resolves one by name. isWall answers for any position, including off-map ones (walls).
func Tilemap(name string, randomSource *rand.Rand, isWall func(geometry.Point) bool) (BasicTileMapping, bool) {
	f, ok := tilemaps[name]
	if !ok {
		return BasicTileMapping{}, false
	}
	return f(randomSource, isWall), true
}

// D1Tilemap picks the tiles of dungeon level dlvl's type.
func D1Tilemap(randomSource *rand.Rand, isWall func(geometry.Point) bool, dlvl int) BasicTileMapping {
	switch (dlvl - 1) / 4 {
	case 0:
		return getCathedralTilemap(randomSource, isWall)
	case 1:
		return getCatacombsTilemap(randomSource, isWall)
	case 2:
		return getCavesTilemap(randomSource, isWall)
	}
	return getHellTilemap(randomSource, isWall)
}

// D1LevelName is the level's name shown on arrival.
func D1LevelName(dlvl int) string {
	return fmt.Sprintf("Level %d\n%s", dlvl, [4]string{"Cathedral", "Catacombs", "Caves", "Hell"}[min((dlvl-1)/4, 3)])
}

func LevelNameFromDepth(level int) string {
	//!gen_dungeon_level_1
	return fmt.Sprintf("!gen_dungeon_level_%d", level)
}
