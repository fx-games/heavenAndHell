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
    readAction.SetNoTimeNeeded()
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
    action := g.getRandomTeleportAction()
    action.SetMonetaryValue(50)
    return g.NewScroll(action)
}

func (g *GameInstance) NewScrollWordOfRecall() *GameItem {
    action := g.getPlayerRecallAction()
    action.SetMonetaryValue(75)
    return g.NewScroll(action)
}

func (g *GameInstance) NewScrollBlessing() *GameItem {
    interval := util.Tuple[int, int]{Item1: 6, Item2: 8}
    action := common.NewUntargetedAction(ActionNameBless, 39)
    action.SetMonetaryValue(180)
    action.SetEffect(func(user *common.Actor) {
        modifier := AddToBaseAttribute(g.randomSource, stats.BaseArmorClass, interval)
        timedModifier := NewTimedModifier(string(ActionNameBless), user, modifier, 8)
        timedModifier.Apply()
        g.timekeeper.AddTimed(timedModifier)
    })
    action.SetDescription(fmt.Sprintf("Increase armor class by %d-%d", interval.Item1, interval.Item2))
    return g.NewScroll(action)
}

func (g *GameInstance) NewScrollEnchantWeapon() *GameItem {
    action := common.NewUntargetedAction(ActionNameEnchantWeapon, 40)
    action.SetMonetaryValue(4000)
    action.SetEffect(func(user *common.Actor) {
        isWeapon := func(element common.Item) bool { return element.IsWeapon() }
        selected := func(itemKey rune) {
            inventory := g.GetWorldState().GetPlayer().GetInventory()
            selectedItem := inventory.GetItemByLetter(itemKey)
            weaponStats := selectedItem.GetWeaponStats()
            weaponStats.Plus += 1
            g.Log(fmt.Sprintf("Your [:darkText]%s[:white] glows [:blue]blue[:white] for a moment.", selectedItem.Name()))
        }
        g.openListInventoryForSelection(isWeapon, selected)
    })
    action.SetDescription("Increase weapon damage by 1")
    return g.NewScroll(action)
}

func (g *GameInstance) NewScrollEnchantArmor() *GameItem {
    action := common.NewUntargetedAction(ActionNameEnchantArmor, 40)
    action.SetMonetaryValue(4000)
    action.SetEffect(func(user *common.Actor) {
        isArmor := func(element common.Item) bool { return element.IsArmor() }
        selected := func(itemKey rune) {
            inventory := g.GetWorldState().GetPlayer().GetInventory()
            selectedItem := inventory.GetItemByLetter(itemKey)
            weaponStats := selectedItem.GetArmorStats()
            weaponStats.Plus += 1
            g.Log(fmt.Sprintf("Your [:darkText]%s[:white] glows [:blue]blue[:white] for a moment.", selectedItem.Name()))
        }
        g.openListInventoryForSelection(isArmor, selected)
    })
    action.SetDescription("Increase armor class by 1")
    return g.NewScroll(action)
}

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

