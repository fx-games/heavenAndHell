package game

import (
	"fmt"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
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

 */
func (g *GameInstance) addRandomWeapon(emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) *GameItem {
	randomWeapon := g.NewRandomWeapon(dungeonLevel)
	emptyMap.AddItem(randomWeapon, pos)
	return randomWeapon
}


func (g *GameInstance) NewRandomWeapon(dungeonLevel int) *GameItem {
	// get defs
	itemDefinitions := g.GetItemDefinitions()

	// filter for dungeon level
	var filtered []ItemDefinition
	weaponTypeMap := make(map[common.WeaponType][]ItemDefinition)
	for _, itemDef := range itemDefinitions {

		if !itemDef.IsWeapon() || !itemDef.CanAppearAt(g.randomSource, dungeonLevel) {
			continue
		}

		filtered = append(filtered, itemDef)
		weaponTypeMap[itemDef.Weapon.WeaponType] = append(weaponTypeMap[itemDef.Weapon.WeaponType], itemDef)
	}

	if len(filtered) == 0 {
		g.ErrLog(fmt.Sprintf("no armor found for dungeon level %d", dungeonLevel))
		return nil
	}
	// group by type
	availableTypes := make([]common.WeaponType, 0)
	for wType, _ := range weaponTypeMap {
		availableTypes = append(availableTypes, wType)
	}

	// pick random type
	weaponType := availableTypes[g.randomSource.Intn(len(availableTypes))]

	// pick random definition
	armorDefinition := weaponTypeMap[weaponType][g.randomSource.Intn(len(weaponTypeMap[weaponType]))]

	// create item
	return g.NewItemFromDefinition(armorDefinition, dungeonLevel)
}

func (g *GameInstance) NewConfuseDart() *GameItem {
	getPlayer := g.state.GetPlayer

	dart := NewDart(getPlayer, g.confuseOnHitProc)
	dart.SetName("a dart of confusion")
	dart.SetTooltip(toLabelText([]string{
		"Throws a dart that confuses the target for 5 turns.",
		"May break on impact.",
	}))
	return dart
}
func (g *GameInstance) NewDart() *GameItem {
	getPlayer := g.state.GetPlayer

	dart := NewDart(getPlayer, nil)
	dart.SetName("a dart")
	dart.SetTooltip(toLabelText([]string{
		"Throws a dart.",
		"May break on impact.",
	}))
	return dart
}

func NewDart(getPlayer func()*common.Actor, onHit common.HitEffect) *GameItem {
	baseItem := common.NewBaseItem(getPlayer, "a dart", common.IconItemDart)
	baseItem.SetCategory(common.ItemCategoryThrowable)
	baseItem.SetQualityLevel(1)
	throwPart := &common.ThrowStats{
		Damage:            stats.NewInterval(1, 4),
		OnHitEffect: onHit,
		HitZone: OneTargetTile,
		ChanceToBreak: 0.2,
	}
	baseItem.SetThrowComponent(throwPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}
