package game

import (
    "fmt"
    "math/rand"
    "rpg/common"
    "rpg/geometry"
    "rpg/gridmap"
    "rpg/stats"
    "rpg/util"
)

func (g *GameInstance) addRandomScroll(source *rand.Rand, emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) {
    var newConsumable *GameItem
    scrollCreationFuncs := []func() *GameItem{
        g.NewScrollPhaseDoor,
        g.NewScrollWordOfRecall,
        g.NewScrollBlessing,
    }
    newConsumable = scrollCreationFuncs[source.Intn(len(scrollCreationFuncs))]()
    emptyMap.AddItem(newConsumable, pos)
}


func (g *GameInstance) NewHiddenNote(itemName string, messagText []util.LabelText, onRead func()) *GameItem {
    readAction := g.getReadMessage(messagText, onRead)
    scroll := g.NewScroll(readAction)
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

func (g *GameInstance) NewScrollPhaseDoor() *GameItem {
    return g.NewScroll(g.getRandomTeleportAction())
}

func (g *GameInstance) NewScrollWordOfRecall() *GameItem {
    return g.NewScroll( g.getPlayerRecallAction())
}

func (g *GameInstance) NewScrollBlessing() *GameItem {
    interval := util.Tuple[int, int]{Item1: 6, Item2: 8}
    action := common.NewUntargetedAction(ActionNameBless, 39)
    action.SetEffect(func(user *common.Actor) {
        modifier := AddToBaseAttribute(g.randomSource, stats.BaseArmorClass, interval)
        timedModifier := NewTimedModifier(string(ActionNameBless), user, modifier, 8)
        timedModifier.Apply()
        g.timekeeper.AddTimed(timedModifier)
    })
    action.SetDescription(fmt.Sprintf("Increase armor class by %d-%d", interval.Item1, interval.Item2))
    return g.NewScroll(action)
}

func (g *GameInstance) NewScroll(untargetedAction *common.UntargetedAction) *GameItem {
    itemName := fmt.Sprintf("a scroll of %s", untargetedAction.Name())
    baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, itemName, common.IconItemScroll, common.SlotNameNotEquippable, 150)
    baseItem.SetCategory(common.ItemCategoryScroll)
    baseItem.SetQualityLevel(1)

    menuForActions := make([]util.MenuItem, 0)
    gameItem := &GameItem{
        BaseItem:         baseItem,
        untargetedAction: untargetedAction,
        actionCharges:    1,
        destroyAfterUse: true,
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

