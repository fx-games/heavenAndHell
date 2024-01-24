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

 */
func addRandomWeapon(state actions.WorldState, source *rand.Rand, emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) *GameItem {
	createWeaponFunctions := []func(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem{
		NewSword,
		NewAxe,
		NewMace,
		NewBow,
		NewStaff,
	}
	creationFunction := createWeaponFunctions[source.Intn(len(createWeaponFunctions))]
	item := creationFunction(state, source, dungeonLevel)
	emptyMap.AddItem(item, pos)
	return item
}

func NewSword(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a sword", common.IconItemSword, common.SlotNameOneHandedWeapon, 150)
	baseItem.SetCategory(common.ItemCategoryWeapon)
	baseItem.SetQualityLevel(qLevel)

	weaponPart := &common.WeaponStats{
		Damage:            stats.NewInterval(2, 6),
		Durability:        stats.NewResource(0, flatFunc(20), 20),
		Type:              common.WeaponTypeSword,
		MeleeRange:        1,
		AttackSpeedFactor: 1,
	}

	baseItem.SetWeaponComponent(weaponPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}
func NewMace(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a mace", common.IconItemMace, common.SlotNameOneHandedWeapon, 150)
	baseItem.SetCategory(common.ItemCategoryWeapon)
	baseItem.SetQualityLevel(1)

	weaponPart := &common.WeaponStats{
		Damage:                      stats.NewInterval(1, 6),
		Durability:                  stats.NewResource(0, flatFunc(20), 20),
		Type:                        common.WeaponTypeAxe,
		MeleeRange:                  1,
		AttackSpeedFactor:           1,
		DamagePerLocationPrediction: getSmallAxeDamageLocations(state),
	}

	baseItem.SetWeaponComponent(weaponPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}
func NewAxe(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a hand axe", common.IconItemAxe, common.SlotNameOneHandedWeapon, 150)
	baseItem.SetCategory(common.ItemCategoryWeapon)
	baseItem.SetQualityLevel(1)

	weaponPart := &common.WeaponStats{
		Damage:                      stats.NewInterval(1, 6),
		Durability:                  stats.NewResource(0, flatFunc(20), 20),
		Type:                        common.WeaponTypeAxe,
		MeleeRange:                  1,
		AttackSpeedFactor:           1,
		DamagePerLocationPrediction: getSmallAxeDamageLocations(state),
	}

	baseItem.SetWeaponComponent(weaponPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewStaff(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a quarterstaff", common.IconItemStaff, common.SlotNameTwoHandedWeapon, 150)
	baseItem.SetCategory(common.ItemCategoryWeapon)
	baseItem.SetQualityLevel(1)

	weaponPart := &common.WeaponStats{
		Damage:                      stats.NewInterval(1, 6),
		Durability:                  stats.NewResource(0, flatFunc(20), 20),
		Type:                        common.WeaponTypeStaff,
		MeleeRange:                  1,
		AttackSpeedFactor:           1,
		DamagePerLocationPrediction: getSmallAxeDamageLocations(state),
	}

	baseItem.SetWeaponComponent(weaponPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewBow(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a bow", common.IconItemBow, common.SlotNameTwoHandedWeapon, 150)
	baseItem.SetCategory(common.ItemCategoryWeapon)
	baseItem.SetQualityLevel(1)

	weaponPart := &common.WeaponStats{
		Damage:            stats.NewInterval(1, 6),
		Durability:        stats.NewResource(0, flatFunc(20), 20),
		Type:              common.WeaponTypeBow,
		AttackSpeedFactor: 1,
	}

	baseItem.SetWeaponComponent(weaponPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}
