package defs

import (
	"fmt"
	"math/rand"
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
	return fmt.Sprintf("Level %d\n%s", dlvl, [6]string{"Cathedral", "Catacombs", "Caves", "Hell", "Nest", "Crypt"}[min((dlvl-1)/4, 5)])
}

func LevelNameFromDepth(level int) string {
	//!gen_dungeon_level_1
	return fmt.Sprintf("!gen_dungeon_level_%d", level)
}
