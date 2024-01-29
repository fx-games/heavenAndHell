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
func NewDagger(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a dagger", common.IconItemDagger)
	baseItem.SetCategory(common.ItemCategoryWeapon)
	baseItem.SetQualityLevel(qLevel)

	weaponPart := &common.WeaponStats{
		Damage:            stats.NewInterval(1, 4),
		Durability:        stats.NewResource(0, flatFunc(20), 20),
		Type:              common.WeaponTypeSword,
		MeleeRange:        1,
		AttackSpeedFactor: 1,
	}

	baseItem.SetWeaponComponent(weaponPart)

	throwPart := &common.ThrowStats{
		Damage:            stats.NewInterval(1, 4),
		HitZone: OneTargetTile,
		ChanceToBreak: 0.25,
	}
	baseItem.SetThrowComponent(throwPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}
func NewSword(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a sword", common.IconItemSword)
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

	throwPart := &common.ThrowStats{
		Damage:            stats.NewInterval(1, 5),
		HitZone: OneTargetTile,
		ChanceToBreak: 0.4,
	}
	baseItem.SetThrowComponent(throwPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}
func NewMace(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a mace", common.IconItemMace)
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

	throwPart := &common.ThrowStats{
		Damage:            stats.NewInterval(2, 4),
		HitZone: OneTargetTile,
		ChanceToBreak: 0.1,
	}
	baseItem.SetThrowComponent(throwPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}
func NewAxe(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a hand axe", common.IconItemAxe)
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

	throwPart := &common.ThrowStats{
		Damage:            stats.NewInterval(2, 5),
		HitZone: OneTargetTile,
		ChanceToBreak: 0.25,
	}
	baseItem.SetThrowComponent(throwPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewStaff(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a quarterstaff", common.IconItemStaff)
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

	throwPart := &common.ThrowStats{
		Damage:            stats.NewInterval(1, 2),
		HitZone: OneTargetTile,
		ChanceToBreak: 0.6,
	}
	baseItem.SetThrowComponent(throwPart)

	baseItem.SetWeaponComponent(weaponPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewBow(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a bow", common.IconItemBow)
	baseItem.SetCategory(common.ItemCategoryWeapon)
	baseItem.SetQualityLevel(1)

	weaponPart := &common.WeaponStats{
		Damage:            stats.NewInterval(1, 6),
		Durability:        stats.NewResource(0, flatFunc(20), 20),
		Type:              common.WeaponTypeBow,
		AttackSpeedFactor: 1,
	}
	baseItem.SetWeaponComponent(weaponPart)


	throwPart := &common.ThrowStats{
		Damage:            stats.NewInterval(1, 2),
		HitZone: OneTargetTile,
		ChanceToBreak: 0.6,
	}
	baseItem.SetThrowComponent(throwPart)

	return &GameItem{
		BaseItem: baseItem,
	}
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

func NewDart(getPlayer func()*common.Actor, onHit func(attacker *common.Actor, hitLocation geometry.Point)) *GameItem {
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
