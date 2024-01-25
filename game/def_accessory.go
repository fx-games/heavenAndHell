package game

import (
	"image/color"
	"math/rand"
	"rpg/actions"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"rpg/util"
)

func addRandomAccessory(state actions.WorldState, source *rand.Rand, emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) *GameItem {
	createFunctions := []func(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem{
		NewRing,
		NewAmulet,
	}
	creationFunction := createFunctions[source.Intn(len(createFunctions))]
	item := creationFunction(state, source, dungeonLevel)
	emptyMap.AddItem(item, pos)
	return item
}

func NewStarGlass(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "the star-glass", common.IconItemStarGlass, common.SlotNameLightSource, 150)
	baseItem.SetCategory(common.ItemCategoryLightSource)
	baseItem.SetQualityLevel(5)

	baseItem.SetLightComponent(&common.LightStats{
		LightColor: color.RGBA{R: 5, G: 250, B: 255, A: 255},
		FlickerPattern: getSmoothFlickerPattern(),
		FlickerFrameDelay: 0.125,
	})
	lightRadius := 3
	baseItem.AddEquippedModifier(AddToBaseAttribute(source, stats.LightRadius, util.NewTuple(lightRadius, lightRadius)))
	return &GameItem{
		BaseItem:      baseItem,
		actionCharges: -1,
	}
}
func NewLantern(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	charges := 7500
	baseItem := common.NewBaseItem(state.GetPlayer, "a lantern", common.IconItemLantern, common.SlotNameLightSource, 150)
	baseItem.SetCategory(common.ItemCategoryLightSource)
	baseItem.SetQualityLevel(1)
	// real torch
	baseItem.SetLightComponent(&common.LightStats{
		LightColor: color.RGBA{ R: 255, G: 223, B: 117, A: 255},
		FlickerPattern: []float64{1.0,1.0,1.0,1.0,1.0,1.0,0.86,0.92,0.9,0.90,0.92,0.96,0.97,0.99, 1.0, 1.0, 1.0, 0.91, 1.0, 1.0, 1.0, 1.0, 0.91,0.93,0.98},
		FlickerFrameDelay: 0.125,
	})

	lightRadius := 2
	baseItem.AddEquippedModifier(AddToBaseAttribute(source, stats.LightRadius, util.NewTuple(lightRadius, lightRadius)))
	return &GameItem{
		BaseItem:      baseItem,
		actionCharges: charges,
	}
}
func NewTorch(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	charges := 2500
	baseItem := common.NewBaseItem(state.GetPlayer, "a torch", common.IconItemTorch, common.SlotNameLightSource, 150)
	baseItem.SetCategory(common.ItemCategoryLightSource)
	baseItem.SetQualityLevel(1)
	// real torch
	baseItem.SetLightComponent(&common.LightStats{
		LightColor: color.RGBA{ R: 255, G: 223, B: 117, A: 255},
		FlickerPattern: []float64{1.0,1.0,1.0,1.0,1.0,1.0,0.85,0.92,0.9,0.90,0.92,0.96,0.97,0.99, 1.0, 1.0, 1.0, 0.9, 1.0, 1.0, 1.0, 1.0, 0.9,0.93,0.98},
		FlickerFrameDelay: 0.125,
	})

	lightRadius := 1
	baseItem.AddEquippedModifier(AddToBaseAttribute(source, stats.LightRadius, util.NewTuple(lightRadius, lightRadius)))
	return &GameItem{
		BaseItem:      baseItem,
		actionCharges: charges,
	}
}

func getSmoothFlickerPattern() []float64 {
	var smoothPattern []float64
	low := 0.7
	high := 1.0
	current := high
	stepCount := 40
	// we want to go from high -> low -> high
	stepSize := ((high - low) * 2) / float64(stepCount)
	reversedDirection := false
	for i := 0; i < stepCount; i++ {
		smoothPattern = append(smoothPattern, current)
		if !reversedDirection {
			current -= stepSize
			if current < low {
				reversedDirection = true
				current = low
			}
		} else {
			current += stepSize
		}
	}
	return smoothPattern
}


func NewRing(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a ring", common.IconItemRing1, common.SlotNameRing, 150)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(1)
	armorPart := &common.ArmorStats{
		BaseArmor:  0,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeRing,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewAmulet(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "an amulet", common.IconItemAmulet1, common.SlotNameAmulet, 150)
	baseItem.SetCategory(common.ItemCategoryArmor)
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
