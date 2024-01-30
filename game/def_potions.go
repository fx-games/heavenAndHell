package game

import (
	"fmt"
	"rpg/common"
	"rpg/stats"
)


func (g *GameInstance) NewRandomPotion(dungeonLevel int) *GameItem {
	// get defs
	itemDefinitions := g.GetItemDefinitions()

	// filter for dungeon level
	var filtered []ItemDefinition
	for _, itemDef := range itemDefinitions {

		if !itemDef.IsPotion() || !itemDef.CanAppearAt(g.randomSource, dungeonLevel) {
			continue
		}

		filtered = append(filtered, itemDef)
	}

	if len(filtered) == 0 {
		g.ErrLog(fmt.Sprintf("no potion found for dungeon level %d", dungeonLevel))
		return nil
	}

	// pick random definition
	definition := filtered[g.randomSource.Intn(len(filtered))]

	// create item
	return g.NewItemFromDefinition(definition, dungeonLevel)
}


func (g *GameInstance) NewPotion(untargetedAction *common.UntargetedAction) *GameItem {
	name := fmt.Sprintf("a potion of %s", untargetedAction.Name())
	baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, name, common.IconItemPotionRed)
	baseItem.SetCategory(common.ItemCategoryPotion)
	baseItem.SetQualityLevel(1)

	gameItem := &GameItem{
		BaseItem:         baseItem,
		untargetedAction: untargetedAction,
		actionCharges:    1,
		destroyAfterUse:  true,
	}
	return gameItem
}

func (g *GameInstance) NewFlaskOfOil() *GameItem {
	name := fmt.Sprintf("a flask of oil")
	baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, name, common.IconItemFlaskOfOil)
	baseItem.SetCategory(common.ItemCategoryPotion)
	baseItem.SetQualityLevel(1)

	gameItem := &GameItem{
		BaseItem:        baseItem,
		actionCharges:   1,
		destroyAfterUse: true,
	}

	hitZone := DijkstraAoE(g.GetWorldState().GetMap(), 1)

	impactDamage := stats.NewInterval(1, 4)

	throwInfos := &common.ThrowStats{
		Damage:        impactDamage,
		OnHitEffect:   g.fireZone,
		HitZone:       hitZone,
		ChanceToBreak: 1.0,
	}
	gameItem.SetThrowComponent(throwInfos)
	gameItem.SetTooltip(toLabelText([]string{
		"Can be thrown to create a pool of oil,",
		fmt.Sprintf("and will deal %s damage on impact.", impactDamage.ToString()),
		"If you have a light-source, the oil is considered lit,",
		"and it will deal 6 damage to any creature standing in the fire.",
	}))

	return gameItem
}

