package game

import (
	"cmp"
	"fmt"
	"image/color"
	"math/rand"
	"rpg/actions"
	"rpg/common"
	"rpg/geometry"
	"rpg/stats"
	"rpg/util"
	"slices"
)



const (
	ActionNameHealHealthAndMana common.UntargetedActionName = "Full Healing"
	ActionNameHealHealth common.UntargetedActionName = "Healing"
	ActionNameBerserk    common.UntargetedActionName = "Berserker Rage"
	ActionNameConfuse	common.UntargetedActionName = "Confusion"
	ActionNameHealMana   common.UntargetedActionName = "Restore Mana"
	ActionNameTeleport   common.UntargetedActionName = "Phase Door"
	ActionNameBuffDamage common.UntargetedActionName = "Slicing"
	ActionNameBuffSpeed common.UntargetedActionName = "Haste"
	ActionNameRecall     common.UntargetedActionName = "Word of Recall"
	ActionNameBless         common.UntargetedActionName = "Blessing"
	ActionNameEnchantWeapon common.UntargetedActionName = "Enchant Weapon"
	ActionNameEnchantArmor  common.UntargetedActionName = "Enchant Armor"
	ActionNameMultiShot        common.UntargetedActionName = "Multi-shot"
	ActionNameTelefrag          common.UntargetedActionName = "Telefrag"
	ActionNameReadMessage       common.UntargetedActionName = "Read Message"
)

func (g *GameInstance) GetRandomUntargetedAction(randomSource *rand.Rand, qLevel int) *common.UntargetedAction {
	creationFunctions := []func() *common.UntargetedAction{
		g.getHealHealthAndMana,
		g.getHealHealth,
		g.getRandomTeleportAction,
		g.getRandomTelefragAction,
		func() *common.UntargetedAction {
			return g.getMultiShot(5)
		},
	}
	createUntargetedAction := creationFunctions[randomSource.Intn(len(creationFunctions))]
	return createUntargetedAction()
}


func (g *GameInstance) getReadMessage(textForMessage []util.LabelText, onRead func()) *common.UntargetedAction {
	action := common.NewUntargetedAction(ActionNameReadMessage, 34)
	action.SetEffect(func(user *common.Actor) {
		g.uiController.OpenTextWindow(textForMessage)
		if onRead != nil {
			onRead()
		}
	})
	return action
}
func (g *GameInstance) getHealHealthAndMana() *common.UntargetedAction {
	action := common.NewUntargetedAction(ActionNameHealHealthAndMana, 34)
	action.SetEffect(func(user *common.Actor) {
		healHealthAction := actions.NewChangeRessourceAction(actions.NewActorCause("Full Heal", user), user, stats.Health, user.GetStats().GetAttribute(stats.MaxHealth))
		healManaAction := actions.NewChangeRessourceAction(actions.NewActorCause("Full Heal", user), user, stats.Mana, user.GetStats().GetAttribute(stats.MaxMana))
		g.actionBroker.Execute(healHealthAction)
		g.actionBroker.Execute(healManaAction)
	})
	return action
}
func (g *GameInstance) getHealHealth() *common.UntargetedAction {
	action := common.NewUntargetedAction(ActionNameHealHealth, 34)
	action.SetEffect(func(user *common.Actor) {
		healHealthAction := actions.NewChangeRessourceAction(actions.NewActorCause("Full Heal", user), user, stats.Health, user.GetStats().GetAttribute(stats.MaxHealth))
		g.actionBroker.Execute(healHealthAction)
		g.updateStatsLabel()
	})
	return action
}
func (g *GameInstance) getHealHealthBy(amount int) *common.UntargetedAction {
	action := common.NewUntargetedAction(ActionNameHealHealth, 34)
	action.SetEffect(func(user *common.Actor) {
		healHealthAction := actions.NewChangeRessourceAction(actions.NewActorCause("Heal", user), user, stats.Health, amount)
		g.actionBroker.Execute(healHealthAction)
		g.Log("You feel better")
	})
	action.SetDescription(fmt.Sprintf("Heal %d health", amount))
	return action
}


func (g *GameInstance) getHealManaBy(amount int) *common.UntargetedAction {
	action := common.NewUntargetedAction(ActionNameHealMana, 34)
	action.SetEffect(func(user *common.Actor) {
		healHealthAction := actions.NewChangeRessourceAction(actions.NewActorCause("Mana Surge", user), user, stats.Mana, amount)
		g.actionBroker.Execute(healHealthAction)
		g.Log("You feel a surge of mana")
	})
	action.SetDescription(fmt.Sprintf("Heal %d mana", amount))
	return action
}
func (g *GameInstance) getMultiShot(shotCount int) *common.UntargetedAction {
	action := common.NewUntargetedAction(ActionNameMultiShot, 34)
	action.SetEffect(func(user *common.Actor) {
		if !user.GetEquipment().HasRangedWeaponEquipped() {
			g.Log("No ranged weapon equipped")
			return
		}
		nearbyEnemies := g.getVisibleEnemiesByDistance(user)
		if len(nearbyEnemies) == 0 {
			g.Log("No enemies in sight")
			return
		}
		hitCount := min(shotCount, len(nearbyEnemies))
		for i := 0; i < hitCount; i++ {
			target := nearbyEnemies[i]
			rangedAttackAction := actions.NewRangedAttackAction(actions.NewActorCause("Ranged attack", user), user, target.MapPosition())
			g.actionBroker.Execute(rangedAttackAction)
		}
	})
	return action
}


func (g *GameInstance) getRandomTelefragAction() *common.UntargetedAction {
	action := common.NewUntargetedAction(ActionNameTelefrag, 39)
	action.SetEffect(func(user *common.Actor) {
		currentMap := g.state.GetMap()
		var visEnemies []*common.Actor
		for _, visPos := range user.GetFOV().Visibles {
			if currentMap.IsActorAt(visPos) {
				actorAt := currentMap.ActorAt(visPos)
				if (actorAt.IsHostileTo(user) || g.shiftIsBeingPressedThisTick) && actorAt.IsAlive() && actorAt != user {
					visEnemies = append(visEnemies, actorAt)
				}
			}
		}
		if len(visEnemies) == 0 {
			g.Log("No enemies in sight")
			return
		}
		randomEnemy := visEnemies[g.randomSource.Intn(len(visEnemies))]
		location := randomEnemy.MapPosition()
		isPlayer := user == g.state.GetPlayer()
		moveAction := actions.NewMoveAction(actions.NewActorCause("Telefrag", user), user, location, isPlayer)
		moveAction.DisableAnimation()

		if user == g.state.GetPlayer() {
			g.cancelAutoMove()
			g.CancelTargeting()
			g.actionBroker.CancelAllAnimations()
		}

		changeHealthAmount := -randomEnemy.GetStats().GetRessource(stats.Health).Value()
		killAction := actions.NewChangeRessourceAction(actions.NewActorCause("Telefrag", user), randomEnemy, stats.Health, changeHealthAmount)
		g.actionBroker.Execute(killAction)
		g.actionBroker.Execute(moveAction)
	})
	return action
}

func (g *GameInstance) getDelayedRandomTeleportAction() *common.UntargetedAction {
	action := common.NewUntargetedAction(ActionNameTeleport, 39)
	action.SetDelay(4)
	action.SetEffect(func(user *common.Actor) {
		currentMap := g.state.GetMap()
		origin := user.MapPosition()
		location, found := currentMap.GetRandomLocation(func(location geometry.Point) bool {
			dist := location.ChebyshevDistanceTo(origin)
			return currentMap.IsCurrentlyPassable(location) && dist > 8 && dist < 14
		})
		if !found {
			return
		}
		isPlayer := user == g.state.GetPlayer()
		moveAction := actions.NewMoveAction(actions.NewActorCause("Teleport", user), user, location, isPlayer)
		moveAction.DisableAnimation()

		if user == g.state.GetPlayer() {
			g.cancelAutoMove()
			g.CancelTargeting()
			g.actionBroker.CancelAllAnimations()
		}

		g.actionBroker.QueueForExecution(moveAction)
	})
	return action
}
func (g *GameInstance) getRandomTeleportAction() *common.UntargetedAction {
	action := common.NewUntargetedAction(ActionNameTeleport, 39)
	action.SetEffect(func(user *common.Actor) {
		currentMap := g.state.GetMap()
		origin := user.MapPosition()
		location, found := currentMap.GetRandomLocation(func(location geometry.Point) bool {
			dist := location.ChebyshevDistanceTo(origin)
			return currentMap.IsCurrentlyPassable(location) && dist > 8 && dist < 14
		})
		if !found {
			return
		}
		isPlayer := user == g.state.GetPlayer()
		moveAction := actions.NewMoveAction(actions.NewActorCause("Teleport", user), user, location, isPlayer)
		moveAction.DisableAnimation()

		if user == g.state.GetPlayer() {
			g.cancelAutoMove()
			g.CancelTargeting()
			g.actionBroker.CancelAllAnimations()
		}

		g.actionBroker.QueueForExecution(moveAction)
	})
	return action
}

func (g *GameInstance) getPlayerRecallAction() *common.UntargetedAction {
	action := common.NewUntargetedAction(ActionNameRecall, 39)
	action.SetDelay(4)
	action.SetEffect(func(user *common.Actor) {
		g.recallAtEndOfTurn()
	})
	return action
}


/*
Name() string
	GetTooltipLines() []util.LabelText
	InventoryIcon() int32
	TintColor() color.Color
*/
type TimedModifier struct {
	user      *common.Actor
	modifier  *stats.Modifier
	turnsLeft int
	energy    int
	icon      int32
	name      string
}

func (t *TimedModifier) TintColor() color.Color {
	return color.White
}
func (t *TimedModifier) Name() string {
	return t.name
}
func (t *TimedModifier) String() string {
	return t.name
}
func (t *TimedModifier) InventoryIcon() int32 {
	return t.icon
}
func (t *TimedModifier) GetTooltipLines() []util.LabelText {
	return []util.LabelText{
		{
			Text:               t.modifier.ToString(),
			TextColor:          common.DarkTextColor,
			UseColorCodes:      false,
			TextWithColorCodes: t.Name(),
		},
	}
}
func (t *TimedModifier) AddEnergy(energy int) {
	t.energy += energy
}

func (t *TimedModifier) RemoveEnergy(used int) {
	t.energy -= used
}

func (t *TimedModifier) IsAlive() bool {
	return t.turnsLeft > 0
}

func (t *TimedModifier) GetBaseSpeed() int {
	return 10
}

func (t *TimedModifier) GetEnergy() int {
	return t.energy
}

func (t *TimedModifier) Turn() {
	if t.turnsLeft == 0 {
		return
	}
	t.turnsLeft--
	if t.turnsLeft == 0 {
		t.Remove()
	}
}

func (t *TimedModifier) Apply() {
	t.user.GetStats().AddModifier(t.modifier)
}

func (t *TimedModifier) Remove() {
	t.user.GetStats().RemoveModifier(t.modifier)
}
func (t *TimedModifier) Counter() int {
	return t.turnsLeft
}
func NewTimedModifier(name string, user *common.Actor, modifier *stats.Modifier, turns int) *TimedModifier {
	return &TimedModifier{
		name : name,
		user:      user,
		modifier:  modifier,
		turnsLeft: turns,
	}
}



func (g *GameInstance) activateUntargetedAction(actor *common.Actor, action *common.UntargetedAction) {
	action.Reset()
	if action.IsDelayed() {
		// add to timekeeper
		g.Log(fmt.Sprintf("%s is delayed for %d turns", action.Name(), action.GetDelayInTurns()))
		action.SetCurrentUser(actor)
		g.timekeeper.AddTimed(action)
	} else {
		action.Invoke(actor)
	}
}

func (g *GameInstance) getVisibleEnemiesByDistance(user *common.Actor) []*common.Actor {
	currentMap := g.GetWorldState().GetMap()
	var nearbyEnemies []*common.Actor
	for _, visPos := range user.GetFOV().Visibles {
		if !currentMap.IsActorAt(visPos) {
			continue
		}
		actor := currentMap.ActorAt(visPos)
		if (actor.IsHostileTo(user) || g.shiftIsBeingPressedThisTick) && actor.IsAlive() && user != actor {
			nearbyEnemies = append(nearbyEnemies, actor)
		}
	}
	slices.SortStableFunc(nearbyEnemies, func(i, j *common.Actor) int {
		distI := currentMap.MoveDistance(user.MapPosition(), i.MapPosition())
		distJ := currentMap.MoveDistance(user.MapPosition(), j.MapPosition())
		return cmp.Compare(distI, distJ)
	})
	return nearbyEnemies
}