package game

import (
	"fmt"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
)

func (g *GameInstance) NewRandomAccessory(dungeonLevel int) *GameItem {
	// get defs
	itemDefinitions := g.GetItemDefinitions()

	// filter for dungeon level
	var filtered []ItemDefinition
	for _, itemDef := range itemDefinitions {

		if !itemDef.IsAccessory() || !itemDef.CanAppearAt(g.randomSource, dungeonLevel) {
			continue
		}

		filtered = append(filtered, itemDef)
	}

	if len(filtered) == 0 {
		g.ErrLog(fmt.Sprintf("no accessory found for dungeon level %d", dungeonLevel))
		return nil
	}
	// pick random type
	accessory := filtered[g.randomSource.Intn(len(filtered))]
	return g.NewItemFromDefinition(accessory, dungeonLevel)
}

func (g *GameInstance) addRandomAccessory(emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) *GameItem {
	accessory := g.NewRandomAccessory(dungeonLevel)
	emptyMap.AddItem(accessory, pos)
	return accessory
}
