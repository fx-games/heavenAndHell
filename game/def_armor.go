package game

import (
	"math/rand"
	"rpg/actions"
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


Armor Rating	Damage Reduction in %
1	0.01
5	0.05
25	0.20
50	0.33
100	0.50
200	0.67
1000	0.91

 */

func addRandomArmor(state actions.WorldState, source *rand.Rand, emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) *GameItem {
	createFunctions := []func(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem{
		NewArmor,
		NewHelmet,
		NewShield,
	}
	creationFunction := createFunctions[source.Intn(len(createFunctions))]
	item := creationFunction(state, source, dungeonLevel)
	emptyMap.AddItem(item, pos)
	return item
}
func NewArmor(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a chain mail", common.IconItemChainMail, common.SlotNameArmorTorso, 150)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(1)

	armorPart := &common.ArmorStats{
		BaseArmor:  7,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeCuirass,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewHelmet(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a helmet", common.IconItemHelmet, common.SlotNameArmorHead, 150)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(1)

	armorPart := &common.ArmorStats{
		BaseArmor:  2,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeHelmet,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewShield(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a shield", common.IconItemShield, common.SlotNameOneHandedWeapon, 150)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(1)

	armorPart := &common.ArmorStats{
		BaseArmor:  6,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeShield,
	}
	weaponPart := &common.WeaponStats{
		Damage:            stats.NewInterval(1, 6).Mul(float64(qLevel)),
		Durability:        stats.NewResource(0, flatFunc(20), 20),
		Type:              common.WeaponTypeShield,
		MeleeRange:        1,
		AttackSpeedFactor: 1 + float64(qLevel)/25,
	}

	baseItem.SetWeaponComponent(weaponPart)
	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}
