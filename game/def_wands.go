package game

import (
	"fmt"
	"rpg/common"
)

func (g *GameInstance) NewRandomWand(dungeonLevel int) *GameItem {
	source := g.randomSource
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
		randomUntargetedAction := g.GetRandomUntargetedAction(dungeonLevel)
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

	return newConsumable
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

func (g *GameInstance) NewTargetedWand(targetedAction *common.TargetedAction, charges int, destroyAfterUse bool) *GameItem {
	itemName := fmt.Sprintf("a wand of %s", targetedAction.Name())
	baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, itemName, common.IconItemWand)
	baseItem.SetCategory(common.ItemCategoryWand)
	baseItem.SetQualityLevel(1)

	gameItem := &GameItem{
		BaseItem:        baseItem,
		targetedAction:  targetedAction,
		actionCharges:   charges,
		destroyAfterUse: destroyAfterUse,
	}
	return gameItem
}
func (g *GameInstance) NewUntargetedWand(untargetedAction *common.UntargetedAction, charges int, destroyAfterUse bool) *GameItem {
	itemName := fmt.Sprintf("a wand of %s", untargetedAction.Name())
	baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, itemName, common.IconItemWand)
	baseItem.SetCategory(common.ItemCategoryWand)
	baseItem.SetQualityLevel(1)

	gameItem := &GameItem{
		BaseItem:         baseItem,
		untargetedAction: untargetedAction,
		actionCharges:    charges,
		destroyAfterUse:  destroyAfterUse,
	}

	return gameItem
}
