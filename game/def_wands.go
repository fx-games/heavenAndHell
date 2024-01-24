package game

import (
	"fmt"
	"math/rand"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/util"
)

func (g *GameInstance) addRandomWand(source *rand.Rand, emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) {
	var newConsumable *GameItem
	roll := source.Intn(100)
	if roll < 50 {
		wandType := []func(targetedAction *common.TargetedAction) *GameItem{
			g.NewFiniteTargetedWand,
			g.NewEtherealTargetedWand,
			g.NewInfiniteTargetedWand,
		}
		randomTargetedAction := g.GetRandomTargetedAction(source, dungeonLevel)
		wandTypeRoll := source.Intn(100)
		wandTypeIndex := 2
		if wandTypeRoll < 45 {
			wandTypeIndex = 0
		} else if wandTypeRoll < 90 {
			wandTypeIndex = 1
		}
		randomTargetableFunc := wandType[wandTypeIndex]
		newConsumable = randomTargetableFunc(randomTargetedAction)
	} else {
		wandType := []func(untargetedAction *common.UntargetedAction) *GameItem{
			g.NewFiniteUntargetedWand,
			g.NewEtherealUntargetedWand,
			g.NewInfiniteUntargetedWand,
		}
		randomUntargetedAction := g.GetRandomUntargetedAction(source, dungeonLevel)
		wandTypeRoll := source.Intn(100)
		wandTypeIndex := 2
		if wandTypeRoll < 45 {
			wandTypeIndex = 0
		} else if wandTypeRoll < 90 {
			wandTypeIndex = 1
		}
		randomUntargetableFunc := wandType[wandTypeIndex]
		newConsumable = randomUntargetableFunc(randomUntargetedAction)
	}
	/* scrapped for now, reasons: need an extra UI step (menu), name too long for UI display
	   else {
	       randomTargetedAction := g.GetRandomTargetedAction(source, dungeonLevel)
	       randomUntargetedAction := g.GetRandomUntargetedAction(source, dungeonLevel)
	       newConsumable = g.NewTwinWand(randomTargetedAction, randomUntargetedAction)
	   }
	*/

	emptyMap.AddItem(newConsumable, pos)
}

func (g *GameInstance) NewInfiniteTargetedWand(targetedAction *common.TargetedAction) *GameItem {
	return g.NewTargetedWand(targetedAction, -1, false)
}

func (g *GameInstance) NewFiniteTargetedWand(targetedAction *common.TargetedAction) *GameItem {
	return g.NewTargetedWand(targetedAction, 3, false)
}
func (g *GameInstance) NewEtherealTargetedWand(targetedAction *common.TargetedAction) *GameItem {
	return g.NewTargetedWand(targetedAction, 5, true)
}
func (g *GameInstance) NewInfiniteUntargetedWand(untargetedAction *common.UntargetedAction) *GameItem {
	return g.NewUntargetedWand(untargetedAction, -1, false)
}

func (g *GameInstance) NewFiniteUntargetedWand(untargetedAction *common.UntargetedAction) *GameItem {
	return g.NewUntargetedWand(untargetedAction, 3, false)
}

func (g *GameInstance) NewEtherealUntargetedWand(untargetedAction *common.UntargetedAction) *GameItem {
	return g.NewUntargetedWand(untargetedAction, 5, true)
}
func (g *GameInstance) NewTwinWand(action *common.TargetedAction, untargetedAction *common.UntargetedAction) *GameItem {
	itemName := fmt.Sprintf("a wand of %s and %s", action.Name(), untargetedAction.Name())
	baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, itemName, common.IconItemWand, common.SlotNameNotEquippable, 150)
	baseItem.SetCategory(common.ItemCategoryWand)
	baseItem.SetQualityLevel(1)

	menuForActions := make([]util.MenuItem, 0)
	gameItem := &GameItem{
		BaseItem:         baseItem,
		targetedAction:   action,
		untargetedAction: untargetedAction,
		actionCharges:    5,
	}
	menuForActions = append(menuForActions, util.MenuItem{
		Text: action.Name(),
		Action: func() {
			g.activateItemActionForPlayer(gameItem)
		},
	})
	menuForActions = append(menuForActions, util.MenuItem{
		Text: untargetedAction.Name(),
		Action: func() {
			g.activateItemActionForPlayer(gameItem)
		},
	})
	baseItem.SetContextActions(menuForActions)

	return gameItem
}
func (g *GameInstance) NewTargetedWand(targetedAction *common.TargetedAction, charges int, destroyAfterUse bool) *GameItem {
	itemName := fmt.Sprintf("a wand of %s", targetedAction.Name())
	baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, itemName, common.IconItemWand, common.SlotNameNotEquippable, 150)
	baseItem.SetCategory(common.ItemCategoryWand)
	baseItem.SetQualityLevel(1)

	menuForActions := make([]util.MenuItem, 0)
	gameItem := &GameItem{
		BaseItem:        baseItem,
		targetedAction:  targetedAction,
		actionCharges:   charges,
		destroyAfterUse: destroyAfterUse,
	}
	menuForActions = append(menuForActions, util.MenuItem{
		Text: targetedAction.Name(),
		Action: func() {
			g.activateItemActionForPlayer(gameItem)
		},
	})
	baseItem.SetContextActions(menuForActions)
	return gameItem
}
func (g *GameInstance) NewUntargetedWand(untargetedAction *common.UntargetedAction, charges int, destroyAfterUse bool) *GameItem {
	itemName := fmt.Sprintf("a wand of %s", untargetedAction.Name())
	baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, itemName, common.IconItemWand, common.SlotNameNotEquippable, 150)
	baseItem.SetCategory(common.ItemCategoryWand)
	baseItem.SetQualityLevel(1)

	menuForActions := make([]util.MenuItem, 0)
	gameItem := &GameItem{
		BaseItem:         baseItem,
		untargetedAction: untargetedAction,
		actionCharges:    charges,
		destroyAfterUse:  destroyAfterUse,
	}
	menuForActions = append(menuForActions, util.MenuItem{
		Text: untargetedAction.Name(),
		Action: func() {
			g.activateItemActionForPlayer(gameItem)
		},
	})
	baseItem.SetContextActions(menuForActions)

	return gameItem
}
