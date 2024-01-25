package game

import (
	"cmp"
	"fmt"
	"math"
	"rpg/common"
	"rpg/dungen"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/ldtk_go"
	"slices"
	"sort"
	"strconv"
	"strings"
)

func (g *GameInstance) generateMap(targetMap string, mapTemplates *ldtk_go.Project, dungeonLevel int) *gridmap.GridMap[*common.Actor, common.Item, common.Object] {
	var dungeonDef LevelDefinition
	if strings.HasPrefix(targetMap, "!spec_") {
		dungeonDef = getLevelDefinitionByName(g.randomSource, SpecialLevelName(targetMap))
	} else {
		dungeonDef = getLevelDefinition(g.randomSource, dungeonLevel)
	}

	generatedLayout := g.createRandomLayout(mapTemplates, dungeonDef.TemplateRules)

	genWidth, genHeight := generatedLayout.GetSize()
	emptyMap := gridmap.NewEmptyMap[*common.Actor, common.Item, common.Object](genWidth, genHeight)
	emptyMap.SetCardinalMovementOnly(g.configuration.CardinalMovementOnly)
	emptyMap.SetName(targetMap)
	emptyMap.SetMetaString(generatedLayout.GetMetaInfo())

	g.addBasicTiles(emptyMap, generatedLayout, dungeonDef.GetTilemapForBasicTiles(generatedLayout, dungeonLevel))

	transitionLocations, remainingStairLocations := g.addStairs(emptyMap, generatedLayout, dungeonDef)
	emptyMap.AddToMetaInfo(fmt.Sprintf("remaining stair locations: %d", len(remainingStairLocations)))

	poiPositions := g.decorateRooms(emptyMap, generatedLayout, dungeonLevel)

	g.addShrine(emptyMap, poiPositions, dungeonLevel)

	randomSource := g.randomSource
	enemySpawnPos := make(map[geometry.Point]bool)
	generatedLayout.TraverseTilesRandomly(randomSource, func(pos geometry.Point) {
		if generatedLayout.IsEmptySpace(pos) {
			enemySpawnPos[pos] = true
		}
	})

	keys := getSortedKeys(enemySpawnPos)
	randomIndices := randomSource.Perm(len(keys))

	for _, index := range randomIndices {
		pos := keys[index]
		// 5% chance to spawn an enemy
		distToStairs := minDist(transitionLocations, pos)
		rollForEnemy := randomSource.Intn(100)
		//println(fmt.Sprintf("pos: %v, rollForEnemy: %d, distToEntrance: %d", pos, rollForEnemy, distToEntrance))
		if rollForEnemy < 5 && distToStairs > 7 && emptyMap.CanPlaceActorHere(pos) {
			g.addRandomEnemy(emptyMap, dungeonLevel, pos)
		} else if randomSource.Intn(100) < 1 && distToStairs > 7 && emptyMap.CanPlaceItemHere(pos) {
			g.addLootToMap(emptyMap, dungeonLevel, pos)
		}
	}

	emptyMap.SetDisplayName(dungeonDef.LevelName)
	emptyMap.Print()
	return emptyMap
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

func (g *GameInstance) addShrine(emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], poiPositions []geometry.Point, dungeonLevel int) {
	if len(poiPositions) == 0 {
		return
	}
	randomIndex := g.randomSource.Intn(len(poiPositions))
	poiPos := poiPositions[randomIndex]
	g.addRandomShrine(g.randomSource, emptyMap, dungeonLevel, poiPos)
}

func (g *GameInstance) decorateRooms(emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], generatedLayout *dungen.DungeonMap, dungeonLevel int) []geometry.Point {
	var poiPositions []geometry.Point
	for _, room := range generatedLayout.AllRooms() {
		roomPoIs := room.GetPOIPositions()
		if len(roomPoIs) > 0 {
			poiPositions = append(poiPositions, roomPoIs...)
		}
		if room.HasDecoratedTemplate() {
			decoLevel := room.GetDecorationLevelName()
			deco := g.getDecorationLevel(decoLevel)
			g.decorateRoomFromTemplate(emptyMap, deco, room)
			emptyMap.AddToMetaInfo(fmt.Sprintf("DecoRoom: %s", deco.Identifier))
		}
	}
	return poiPositions
}

func (g *GameInstance) addStairs(emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], generatedLayout *dungen.DungeonMap, levelDef LevelDefinition) ([]geometry.Point, []geometry.Point) {
	randomSource := g.randomSource
	ladderUpTile := gridmap.Tile{
		DefinedIcon:        common.IconStairsUp,
		DefinedDescription: "a ladder leading up",
		IsWalkable:         true,
		IsTransparent:      true,
		Special:            gridmap.SpecialTileStairsUp,
	}
	ladderDownTile := gridmap.Tile{
		DefinedIcon:        common.IconStairsDown,
		DefinedDescription: "a ladder leading down",
		IsWalkable:         true,
		IsTransparent:      true,
		Special:            gridmap.SpecialTileStairsDown,
	}

	stairPositions := generatedLayout.GetPotentialStairPositions()
	var transitionLocations []geometry.Point
	if levelDef.AddDefaultDungeonTransitions {
		if len(stairPositions) < 2 {
			panic("not enough stair positions") // TODO: do something more sensible
		}

		entranceIndex := randomSource.Intn(len(stairPositions))
		entrance := stairPositions[entranceIndex]
		// remove the entrance from the list of possible exits
		stairPositions = append(stairPositions[:entranceIndex], stairPositions[entranceIndex+1:]...)

		slices.SortStableFunc(stairPositions, func(i, j geometry.Point) int {
			return cmp.Compare(i.ManhattanDistanceTo(entrance), j.ManhattanDistanceTo(entrance))
		})

		var exitIndex int
		// 50/50 chance for random entrance and exit
		if randomSource.Intn(2) == 0 { // largest distance
			exitIndex = len(stairPositions) - 1
		} else { // random pairs, but not the same
			exitIndex = randomSource.Intn(len(stairPositions))
		}
		exit := stairPositions[exitIndex]
		// remove the exit from the list of possible exits
		stairPositions = append(stairPositions[:exitIndex], stairPositions[exitIndex+1:]...)

		mapEntryPosition := entrance
		emptyMap.SetTile(mapEntryPosition, ladderUpTile)
		emptyMap.AddNamedLocation("ladder_up", mapEntryPosition)
		dungeonLevel := levelDef.DungeonLevel
		if dungeonLevel == 1 {
			emptyMap.AddTransitionAt(mapEntryPosition, gridmap.Transition{
				TargetMap:      string(LevelNameTown),
				TargetLocation: "to_cathedral",
			})
		} else {
			emptyMap.AddTransitionAt(mapEntryPosition, gridmap.Transition{
				TargetMap:      levelName(dungeonLevel - 1),
				TargetLocation: "ladder_down",
			})
		}

		mapExitPosition := exit

		emptyMap.SetTile(mapExitPosition, ladderDownTile)
		emptyMap.AddNamedLocation("ladder_down", mapExitPosition)
		emptyMap.AddTransitionAt(mapExitPosition, gridmap.Transition{
			TargetMap:      levelName(dungeonLevel + 1),
			TargetLocation: "ladder_up",
		})

		transitionLocations = append(transitionLocations, mapEntryPosition, mapExitPosition)

		slices.SortStableFunc(stairPositions, func(i, j geometry.Point) int {
			iDist := i.ManhattanDistanceTo(entrance) + i.ManhattanDistanceTo(exit)
			jDist := j.ManhattanDistanceTo(entrance) + j.ManhattanDistanceTo(exit)
			return cmp.Compare(iDist, jDist)
		})
	}

	if len(levelDef.Transitions) > 0 {
		if len(levelDef.Transitions) > len(stairPositions) {
			println(fmt.Sprintf("ERR: not enough stair positions for all transitions on map %s", emptyMap.GetName()))
		} else {
			for _, additionalTransition := range levelDef.Transitions {
				//pop pos
				furthestIndex := len(stairPositions) - 1
				stairPos := stairPositions[furthestIndex]
				stairPositions = stairPositions[:furthestIndex]
				locationName := additionalTransition.LocationName
				transitionTo := additionalTransition.Target
				var tile gridmap.Tile
				if additionalTransition.IsUp {
					tile = ladderUpTile
				} else {
					tile = ladderDownTile
				}
				emptyMap.SetTile(stairPos, tile)
				emptyMap.AddNamedLocation(locationName, stairPos)
				emptyMap.AddTransitionAt(stairPos, transitionTo)
				transitionLocations = append(transitionLocations, stairPos)
			}
		}

	}

	return transitionLocations, stairPositions
}

type BasicTileMapping struct {
	FloorTile       int32
	ChooseWallTile  func(mapPos geometry.Point) int32
	ChooseFloorTile func(mapPos geometry.Point) int32
	ClosedDoor      int32
	OpenDoor        int32
}

func (g *GameInstance) addBasicTiles(emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], layout *dungen.DungeonMap, tileMap BasicTileMapping) []geometry.Point {
	wallTileFromIcon := func(icon int32) gridmap.Tile {
		return gridmap.Tile{
			DefinedIcon:        icon,
			DefinedDescription: "a wall",
			IsWalkable:         false,
			IsTransparent:      false,
		}
	}
	floorTileFromIcon := func(icon int32) gridmap.Tile {
		return gridmap.Tile{
			DefinedIcon:        icon,
			DefinedDescription: "a floor",
			IsWalkable:         true,
			IsTransparent:      true,
		}
	}

	sizeX, sizeY := layout.GetSize()
	var allDoorPositions []geometry.Point
	for x := 0; x < sizeX; x++ {
		for y := 0; y < sizeY; y++ {
			mapPos := geometry.Point{X: x, Y: y}
			switch layout.GetTile(x, y) {
			case dungen.Wall:
				emptyMap.SetTile(mapPos, wallTileFromIcon(tileMap.ChooseWallTile(mapPos)))
			case dungen.Door:
				emptyMap.SetTile(mapPos, floorTileFromIcon(tileMap.FloorTile))
				emptyMap.AddObject(common.NewDoor(tileMap.OpenDoor, tileMap.ClosedDoor), mapPos)
				allDoorPositions = append(allDoorPositions, mapPos)
			case dungen.Room:
				fallthrough
			case dungen.Corridor:
				fallthrough
			default:
				emptyMap.SetTile(mapPos, floorTileFromIcon(tileMap.ChooseFloorTile(mapPos)))
			}
		}
	}
	return allDoorPositions
}

func (g *GameInstance) createRandomLayout(mapTemplates *ldtk_go.Project, levelRules dungen.TemplateRules) *dungen.DungeonMap {
	randomSource := g.randomSource

	scratchSpaceWidth := 40
	scratchSpaceHeight := 40
	dunGen := dungen.NewAccretionGenerator(randomSource)
	dunGen.LoadTemplatesFrom(mapTemplates, levelRules.TemplatePrefix)
	dunGen.SetLevelRules(levelRules)

	//dunGen.SetDebugSteps(true)
	dunGen.SetRoomCount(16)
	generatedLayout := dunGen.Generate(scratchSpaceWidth, scratchSpaceHeight)
	for len(generatedLayout.AllRooms()) < 10 {
		generatedLayout = dunGen.Generate(scratchSpaceWidth, scratchSpaceHeight)
	}
	return generatedLayout
}

func (g *GameInstance) getDecorationLevel(decoLevel string) *ldtk_go.Level {
	var validLevels []*ldtk_go.Level
	for _, level := range g.ldtkProject.Levels {
		if strings.HasPrefix(level.Identifier, decoLevel) {
			validLevels = append(validLevels, level)
		}
	}
	if len(validLevels) == 0 {
		panic("no valid decoration levels found")
	}
	randomDeco := validLevels[g.randomSource.Intn(len(validLevels))]
	println("deco level: " + randomDeco.Identifier)
	return randomDeco
}

func getSortedKeys(pos map[geometry.Point]bool) []geometry.Point {
	keys := make([]geometry.Point, len(pos))
	i := 0
	for k := range pos {
		keys[i] = k
		i++
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i].X < keys[j].X || (keys[i].X == keys[j].X && keys[i].Y < keys[j].Y)
	})
	return keys
}

func levelFromName(targetMap string) int {
	nameParts := strings.Split(targetMap, "_")
	level, _ := strconv.Atoi(nameParts[len(nameParts)-1])
	return level
}

func levelName(level int) string {
	//!gen_dungeon_level_1
	return fmt.Sprintf("!gen_dungeon_level_%d", level)
}
