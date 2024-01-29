package game

import (
	"math/rand"
	"rpg/actions"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
)

func addRandomAccessory(state actions.WorldState, source *rand.Rand, emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) *GameItem {
	createFunctions := []func(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem{
		NewRing,
		NewAmulet,
		NewRingOfRegeneration,
		NewRingOfProtection,
		NewRingOfDamage,
	}
	creationFunction := createFunctions[source.Intn(len(createFunctions))]
	item := creationFunction(state, source, dungeonLevel)
	emptyMap.AddItem(item, pos)
	return item
}

func NewRingOfRegeneration(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a ring of regeneration", common.IconItemRing2)
	baseItem.SetCategory(common.ItemCategoryAccessory)
	baseItem.SetQualityLevel(1)

	baseItem.SetEquippedModifiers([]*stats.Modifier{
		{
			Name:      "of regeneration",
			Attribute: stats.HealthRegeneration,
			Value:     1,
			Mode:      stats.ModifyAdditive,
			IsPercentBased: false,
		},
	})

	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewRingOfDamage(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a ring of damage", common.IconItemRing2)
	baseItem.SetCategory(common.ItemCategoryAccessory)
	baseItem.SetQualityLevel(qLevel)
	damageAdded := source.Intn(2) + 1
	baseItem.SetEquippedModifiers([]*stats.Modifier{
		{
			Name:           "of damage",
			Attribute:      stats.BaseDamage,
			Value: 			float64(damageAdded),
			Mode:           stats.ModifyAdditive,
			IsPercentBased: false,
		},
	})

	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewRingOfProtection(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a ring of protection", common.IconItemRing2)
	baseItem.SetCategory(common.ItemCategoryAccessory)
	baseItem.SetQualityLevel(qLevel)
	acAdded := source.Intn(3) + 3
	baseItem.SetEquippedModifiers([]*stats.Modifier{
		{
			Name:           "of protection",
			Attribute:      stats.BaseArmorClass,
			Value: 			float64(acAdded),
			Mode:           stats.ModifyAdditive,
			IsPercentBased: false,
		},
	})

	return &GameItem{
		BaseItem: baseItem,
	}
}
func NewRing(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a ring", common.IconItemRing1)
	baseItem.SetCategory(common.ItemCategoryAccessory)
	baseItem.SetQualityLevel(1)

	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewAmulet(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "an amulet", common.IconItemAmulet2)
	baseItem.SetCategory(common.ItemCategoryAccessory)
	baseItem.SetQualityLevel(1)

	armorPart := &common.ArmorStats{
		BaseArmor:  0,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeAmulet,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}
