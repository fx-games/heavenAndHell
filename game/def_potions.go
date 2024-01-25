package game

import (
    "fmt"
    "math/rand"
    "rpg/actions"
    "rpg/common"
    "rpg/geometry"
    "rpg/gridmap"
    "rpg/stats"
    "rpg/util"
)

func (g *GameInstance) addRandomPotion(source *rand.Rand, emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) {
    var newConsumable *GameItem
    potionCreationFuncs := []func() *GameItem{
        g.NewHealingPotion,
        g.NewManaPotion,
        g.NewStrengthPotion,
    }
    newConsumable = potionCreationFuncs[source.Intn(len(potionCreationFuncs))]()
    emptyMap.AddItem(newConsumable, pos)
}


func (g *GameInstance) NewHealingPotion() *GameItem {
    healAction := g.getHealHealthBy(10)
    potion := g.NewPotion(healAction)
    potion.SetIcon(common.IconItemPotionRed)
    return potion
}

func (g *GameInstance) NewManaPotion() *GameItem {
    healAction := g.getHealManaBy(10)
    potion := g.NewPotion(healAction)
    potion.SetIcon(common.IconItemPotionBlue)
    return potion
}

func (g *GameInstance) NewBerserkPotion() *GameItem {
    intervalRange := util.Tuple[int, int]{Item1: 6, Item2: 12}
    status := common.StatusBerserk
    turnCount := 6

    applyBerserk := common.NewUntargetedAction(ActionNameBerserk, 34)
    applyBerserk.SetEffect(func(user *common.Actor) {
        applyStatusAction := actions.NewApplyStatusAction(actions.NewActorCause("ApplyStatusEffect", user), user, status, turnCount)
        g.actionBroker.Execute(applyStatusAction)

        modifier := AddToBaseAttribute(g.randomSource, stats.BaseDamage, intervalRange)
        timedModifier := NewTimedModifier(string(ActionNameBuffDamage), user, modifier, turnCount)
        timedModifier.Apply()
        g.timekeeper.AddTimed(timedModifier)
        g.Log("Blood for the blood god!")
    })
    applyBerserk.SetDescription(fmt.Sprintf("Go berserk for %d turns", turnCount))
    potion := g.NewPotion(applyBerserk)
    potion.SetIcon(common.IconItemPotionOrange)
    return potion
}

func (g *GameInstance) NewStrengthPotion() *GameItem {
    intervalRange := util.Tuple[int, int]{Item1: 3, Item2: 6}
    increaseStrengthAction := common.NewUntargetedAction(ActionNameBuffDamage, 34)
    increaseStrengthAction.SetEffect(func(user *common.Actor) {
        modifier := AddToBaseAttribute(g.randomSource, stats.BaseDamage, intervalRange)
        timedModifier := NewTimedModifier(string(ActionNameBuffDamage), user, modifier, 3)
        timedModifier.Apply()
        g.timekeeper.AddTimed(timedModifier)
        g.Log("You feel stronger!")
    })
    increaseStrengthAction.SetDescription(fmt.Sprintf("Increase damage by %d-%d", intervalRange.Item1, intervalRange.Item2))
    potion := g.NewPotion(increaseStrengthAction)
    potion.SetIcon(common.IconItemPotionBrown)
    return potion
}

func (g *GameInstance) NewPotion(untargetedAction *common.UntargetedAction) *GameItem {
    name := fmt.Sprintf("a potion of %s", untargetedAction.Name())
    baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, name, common.IconItemPotionRed, common.SlotNameNotEquippable, 150)
    baseItem.SetCategory(common.ItemCategoryPotion)
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
