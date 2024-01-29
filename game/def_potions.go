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
        g.NewSpeedPotion,
        g.NewBerserkPotion,
    }
    newConsumable = potionCreationFuncs[source.Intn(len(potionCreationFuncs))]()
    emptyMap.AddItem(newConsumable, pos)
}


func (g *GameInstance) NewHealingPotion() *GameItem {
    action := g.getHealHealthBy(10)
    action.SetMonetaryValue(15)
    potion := g.NewPotion(action)
    potion.SetIcon(common.IconItemPotionRed)
    return potion
}

func (g *GameInstance) NewManaPotion() *GameItem {
    action := g.getHealManaBy(10)
    action.SetMonetaryValue(15)
    potion := g.NewPotion(action)
    potion.SetIcon(common.IconItemPotionBlue)
    return potion
}

func (g *GameInstance) NewBerserkPotion() *GameItem {
    intervalRange := util.Tuple[int, int]{Item1: 6, Item2: 12}
    status := common.StatusBerserk
    turnCount := 6

    action := common.NewUntargetedAction(ActionNameBerserk, 34)
    action.SetMonetaryValue(250)
    action.SetEffect(func(user *common.Actor) {
        applyStatusAction := actions.NewApplyStatusAction(actions.NewActorCause("ApplyStatusEffect", user), user, status, turnCount)
        g.actionBroker.Execute(applyStatusAction)

        modifier := AddToBaseAttribute(g.randomSource, stats.BaseDamage, intervalRange)
        timedModifier := NewTimedModifier(string(ActionNameBuffDamage), user, modifier, turnCount)
        timedModifier.Apply()
        g.timekeeper.AddTimed(timedModifier)
        g.Log("Blood for the blood god!")
    })
    action.SetDescription(fmt.Sprintf("Go berserk for %d turns", turnCount))
    potion := g.NewPotion(action)
    potion.SetIcon(common.IconItemPotionOrange)
    return potion
}

func (g *GameInstance) NewConfusePotion() *GameItem {
    status := common.StatusConfused
    turnCount := 6
    action := common.NewUntargetedAction(ActionNameConfuse, 34)
    action.SetMonetaryValue(100)
    action.SetEffect(func(user *common.Actor) {
        applyStatusAction := actions.NewApplyStatusAction(actions.NewActorCause("ApplyStatusEffect", user), user, status, turnCount)
        g.actionBroker.Execute(applyStatusAction)
        g.Log("You feel disoriented!")
    })
    action.SetDescription(fmt.Sprintf("Get confused for %d turns", turnCount))
    potion := g.NewPotion(action)
    potion.SetIcon(common.IconItemPotionWhite)
    return potion
}


func (g *GameInstance) NewSpeedPotion() *GameItem {
    turnCount := 6
    speedBuff := 10
    action := common.NewUntargetedAction(ActionNameBuffSpeed, 34)
    action.SetMonetaryValue(200)
    action.SetEffect(func(user *common.Actor) {
        modifier := &stats.Modifier{
            Attribute:      stats.BaseSpeed,
            Name:           "of haste",
            Value:          float64(speedBuff),
            Mode:           stats.ModifyAdditive,
            IsPercentBased: false,
        }
        timedModifier := NewTimedModifier(string(ActionNameBuffSpeed), user, modifier, turnCount)
        timedModifier.Apply()
        g.timekeeper.AddTimed(timedModifier)
        g.Log("You feel the world around you slow down!")
    })
    action.SetDescription(fmt.Sprintf("Increase speed by %d for %d turns", speedBuff, turnCount))
    potion := g.NewPotion(action)
    potion.SetIcon(common.IconItemPotionBrown)
    return potion
}

func (g *GameInstance) NewStrengthPotion() *GameItem {
    intervalRange := util.Tuple[int, int]{Item1: 3, Item2: 6}
    action := common.NewUntargetedAction(ActionNameBuffDamage, 34)
    action.SetMonetaryValue(200)
    action.SetEffect(func(user *common.Actor) {
        modifier := AddToBaseAttribute(g.randomSource, stats.BaseDamage, intervalRange)
        timedModifier := NewTimedModifier(string(ActionNameBuffDamage), user, modifier, 3)
        timedModifier.Apply()
        g.timekeeper.AddTimed(timedModifier)
        g.Log("You feel stronger!")
    })
    action.SetDescription(fmt.Sprintf("Increase damage by %d-%d", intervalRange.Item1, intervalRange.Item2))
    potion := g.NewPotion(action)
    potion.SetIcon(common.IconItemPotionBrown)
    return potion
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
        destroyAfterUse: true,
    }
    return gameItem
}

func (g *GameInstance) NewFlaskOfOil() *GameItem {
    name := fmt.Sprintf("a flask of oil")
    baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, name, common.IconItemFlaskOfOil)
    baseItem.SetCategory(common.ItemCategoryPotion)
    baseItem.SetQualityLevel(1)

    gameItem := &GameItem{
        BaseItem:         baseItem,
        actionCharges:    1,
        destroyAfterUse: true,
    }

    hitZone := DijkstraAoE(g.GetWorldState().GetMap(), 1)

    impactDamage := stats.NewInterval(1, 4)

    throwInfos := &common.ThrowStats{
        Damage: impactDamage,
        OnHitEffect: func(attacker *common.Actor, targetLocation geometry.Point) {
            if !attacker.GetInventory().HasLightSource() {
                return
            }
            hitLocations := hitZone(attacker.MapPosition(), targetLocation)
            damagePerTile := 6
            damageSpellAction := actions.NewDamageSpellAction(actions.NewActorCause("Flask of Oil", attacker), attacker, damagePerTile, targetLocation, hitLocations)
            g.actionBroker.Execute(damageSpellAction)
        },
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
