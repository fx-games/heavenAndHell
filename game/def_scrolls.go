package game

import (
    "fmt"
    "rpg/common"
    "rpg/util"
)

func (g *GameInstance) NewRandomScroll(dungeonLevel int) *GameItem {
    // get defs
    itemDefinitions := g.GetItemDefinitions()

    // filter for dungeon level
    var filtered []ItemDefinition
    for _, itemDef := range itemDefinitions {

        if !itemDef.IsScroll() || !itemDef.CanAppearAt(g.randomSource, dungeonLevel) {
            continue
        }

        filtered = append(filtered, itemDef)
    }

    if len(filtered) == 0 {
        g.ErrLog(fmt.Sprintf("no scroll found for dungeon level %d", dungeonLevel))
        return nil
    }

    // pick random definition
    definition := filtered[g.randomSource.Intn(len(filtered))]

    // create item
    return g.NewItemFromDefinition(definition, dungeonLevel)
}


func (g *GameInstance) NewHiddenNote(itemName string, messagText []util.LabelText, onRead func()) *GameItem {
    action := common.NewUntargetedAction("read", common.IconItemScroll)
    //action.SetDescription(fmt.Sprintf("TODO: Description for %s", def.Name))

    action.SetEffect(g.getReadMessage(messagText, onRead))
    action.SetNoTimeNeeded()

    scroll := g.NewScroll(action)
    scroll.SetName(itemName)
    scroll.SetUnlimitedCharges()
    scroll.ShowNameOnlyInTooltip()
    scroll.Hide()
    return scroll
}
/*

Consumables:
Heal potions
Mana potions
Increase Armor
Increase Damage
Increase Speed
Phase Door
Torches
Word of Recall
Enchant Weapon/Armor
Deep Descent


*/
func (g *GameInstance) NewScroll(untargetedAction *common.UntargetedAction) *GameItem {
    itemName := fmt.Sprintf("a scroll of %s", untargetedAction.Name())
    baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, itemName, common.IconItemScroll)
    baseItem.SetCategory(common.ItemCategoryScroll)
    baseItem.SetQualityLevel(1)

    gameItem := &GameItem{
        BaseItem:         baseItem,
        untargetedAction: untargetedAction,
        actionCharges:    1,
        destroyAfterUse: true,
    }
    return gameItem
}

