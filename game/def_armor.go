package game

import (
	"fmt"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
)

/*

Starting Items
Dmg.: 2-6
AC: 1-5

Mid Game Items
Dmg.: 6-12
AC: 6-23

End Game Items
Dmg.: 12-30
AC: 24-100


Armor Rating	Damage Reduction in %
1	0.01
5	0.05
25	0.20
50	0.33
100	0.50
200	0.67
1000	0.91
*/

func (g *GameInstance) NewRandomArmorPiece(dungeonLevel int) *GameItem {
	// get defs
	itemDefinitions := g.GetItemDefinitions()

	// filter for dungeon level
	var filteredArmorDefinitions []ItemDefinition
	armorTypeMap := make(map[common.ArmorType][]ItemDefinition)
	for _, itemDef := range itemDefinitions {

		if !itemDef.IsArmor() || !itemDef.CanAppearAt(g.randomSource, dungeonLevel) {
			continue
		}

		filteredArmorDefinitions = append(filteredArmorDefinitions, itemDef)
		armorTypeMap[itemDef.Armor.ArmorType] = append(armorTypeMap[itemDef.Armor.ArmorType], itemDef)
	}

	if len(filteredArmorDefinitions) == 0 {
		g.ErrLog(fmt.Sprintf("no armor found for dungeon level %d", dungeonLevel))
		return nil
	}
	// group by armor type
	availableArmorTypes := make([]common.ArmorType, 0)
	for armorType, _ := range armorTypeMap {
		availableArmorTypes = append(availableArmorTypes, armorType)
	}

	// pick random armor type
	armorType := availableArmorTypes[g.randomSource.Intn(len(availableArmorTypes))]

	// pick random armor definition
	armorDefinition := armorTypeMap[armorType][g.randomSource.Intn(len(armorTypeMap[armorType]))]

	// create item
	return g.NewItemFromDefinition(armorDefinition, dungeonLevel)
}

func (g *GameInstance) addRandomArmor(emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) *GameItem {
	armorPiece := g.NewRandomArmorPiece(dungeonLevel)
	emptyMap.AddItem(armorPiece, pos)
	return armorPiece
}