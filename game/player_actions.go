package game

import (
	"cmp"
	"fmt"
	"math"
	"math/rand"
	"rpg/audio"
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
	"rpg/util"
	"slices"
)

type Actions struct {
	player       *common.Actor
	actionBroker *common.Broker
	state        *common.State
	animator     common.Animator
	ui           func() common.UserInterface
	config       *common.Configuration
	audio        *audio.Player
	definitions  defs.DataContainer

	getTimeKeeper  func() *common.TimeKeeper
	logError       func(message string)
	autoExplore    bool
	playerPath     []geometry.Point
	isResting      bool
	queueAfterTurn func(call func())
}

func (a *Actions) BeginActionTargeting(element *common.TargetedAction) {
	a.beginActionTargeting(element, nil)
}

func (a *Actions) EndTurn(autTaken int) {
	a.actionBroker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("Wait"), autTaken))
}

func (a *Actions) DropItem(item *common.Item) {
	dropItemAction := common.NewDropAction(common.AbstractCause("DropItem"), a.player, item)
	a.actionBroker.Execute(dropItemAction)
}

func (a *Actions) ActivateUntargetedAction(action *common.UntargetedAction, source common.EffectSource) {
	a.player.SetLastCastUntargeted(action)
	a.activateUntargetedAction(a.player, action, source)
}
func (a *Actions) beginActionTargeting(action *common.TargetedAction, chargedItem *common.Item) {
	state := a.state
	if !state.GetPlayer().IsAlive() {
		return
	}
	if chargedItem == nil {
		a.player.SetLastCastTargeted(action)
	}
	act := a
	act.CancelAutoMove()

	rangedAttackRules := action.GetTargetingRules(state.GetPlayer(), state.GetMap())
	onHit := &common.HitEffect{
		Activate: func(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
			if chargedItem != nil {
				chargedItem.UseCharge()
				if !chargedItem.HasChargesLeft() && chargedItem.SelfDestructs() {
					state.GetPlayer().GetInventory().Remove(chargedItem)
				}
			}
			act.CancelAutoMove()
			action.OnSelected(attacker, target, hitPositions)
		},
		Description: action.GetDescription(),
	}
	a.ui().BeginTargeting(rangedAttackRules, onHit)
}

func (a *Actions) PickUpItem() {
	state := a.state
	player := state.GetPlayer()
	if !player.IsAlive() || !state.GetMap().IsItemAt(player.MapPosition()) {
		return
	}
	if !player.CanPickup(state.GetMap().ItemAt(player.MapPosition())) {
		state.Factory().PlayD1HeroSound(14 + rand.Intn(3)) // AutoGetItem: "I can't carry any more"
		return
	}
	pickUpAction := common.NewPickUpAction(common.AbstractCause("PickUpAction"), player)
	a.actionBroker.Execute(pickUpAction)
	a.actionBroker.Execute(common.NewEndPlayerTurnAction(pickUpAction.Cause(), pickUpAction.GetDelayInAut()))
}

type PlayerActionsContext struct {
	Player         *common.Actor
	ActionBroker   *common.Broker
	State          *common.State
	UI             func() common.UserInterface
	GetTimeKeeper  func() *common.TimeKeeper
	Msg            func(message string)
	Animator       common.Animator
	Configuration  *common.Configuration
	QueueAfterTurn func(call func())
}

func NewActions(context PlayerActionsContext) *Actions {
	return &Actions{
		player:         context.Player,
		actionBroker:   context.ActionBroker,
		state:          context.State,
		ui:             context.UI,
		animator:       context.Animator,
		getTimeKeeper:  context.GetTimeKeeper,
		logError:       context.Msg,
		config:         context.Configuration,
		queueAfterTurn: context.QueueAfterTurn,
	}
}
func (a *Actions) EquipItem(item *common.Item) {
	if d := item.D1(); d != nil && !item.MeetsRequirements() {
		a.logError(fmt.Sprintf("You can't equip %s: it needs %d Str, %d Mag and %d Dex.", item.Name(), d.MinStr, d.MinMag, d.MinDex))
		a.state.Factory().PlayD1HeroSound(13)
		return
	}
	a.actionBroker.Execute(common.NewEquipItemAction(common.AbstractCause("equip"), a.player, item))
}

func (a *Actions) UnequipItem(item *common.Item) {
	a.actionBroker.Execute(common.NewUnequipItemAction(common.AbstractCause("unequip"), a.player, item))
}

func (a *Actions) ActivateItemActionForPlayer(itemWithAction *common.Item) {
	msg := a.logError
	user := a.player // an AI actor needs to provide a target pos for targeted common.. handle this elsewhere..
	if !user.CanUseItems() || !itemWithAction.HasAction() {
		return
	}

	if !itemWithAction.HasChargesLeft() {
		msg(fmt.Sprintf("%s is out of charges", itemWithAction.Name()))
		return
	}

	if itemWithAction.HasTargetedAction() {
		a.beginActionTargeting(itemWithAction.GetTargetedAction(), itemWithAction)
	} else if itemWithAction.HasUntargetedAction() {

		untargetedAction := itemWithAction.GetUntargetedAction()
		if !untargetedAction.CanActivate(user) {
			return // e.g. a book that needs more magic than the reader has: not consumed
		}

		itemWithAction.UseCharge()
		if !itemWithAction.HasChargesLeft() && itemWithAction.SelfDestructs() {
			user.GetInventory().Remove(itemWithAction)
		}

		a.activateUntargetedAction(user, untargetedAction, common.EffectSource(itemWithAction.GetInternalName()))
		if itemWithAction.IsPotion() { // potion effects don't end the turn themselves, like spells do
			a.actionBroker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("Drink"), 10))
		}
		/* Actions should end the turn themselves
		autTaken := untargetedAction.GetTimeNeededInAut()
		if autTaken > 0 {
			a.actionBroker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("ActiveItemUse"), autTaken))
		}
		*/
	}
}

func (a *Actions) activateUntargetedAction(actor *common.Actor, action *common.UntargetedAction, source common.EffectSource) {
	action.Reset()
	if action.IsDelayed() {
		// add to timekeeper
		a.logError(fmt.Sprintf("%s is delayed for %d turns", action.Name(), action.GetDelayInTurns()))
		action.SetCurrentUserAndSource(actor, source)
		timeKeeper := a.getTimeKeeper()
		timeKeeper.AddTimed(action)
	} else if action.CanActivate(actor) {
		action.Invoke(actor, source)
	}
}
func (a *Actions) playerThrow(thrownItem *common.Item) func(player *common.Actor, targetPos geometry.Point) {
	return func(player *common.Actor, targetPos geometry.Point) {
		throwAction := common.NewThrowAction(common.NewActorCause("Throw", player), player, thrownItem, targetPos)
		a.actionBroker.Execute(throwAction)
		a.actionBroker.Execute(common.NewEndPlayerTurnAction(throwAction.Cause(), throwAction.GetDelayInAut()))
	}
}
func (a *Actions) GetContextActionsForItem(item *common.Item) []util.MenuItem {
	// TODO:-..
	var menuForActions []util.MenuItem
	if item.HasUntargetedAction() {
		menuForActions = append(menuForActions, util.MenuItem{
			MainText: item.GetUntargetedAction().Name(),
			Action: func() {
				a.ActivateItemActionForPlayer(item)
			},
		})
	}
	if item.HasTargetedAction() {
		menuForActions = append(menuForActions, util.MenuItem{
			MainText: item.GetTargetedAction().Name(),
			Action: func() {
				a.ActivateItemActionForPlayer(item)
			},
		})
	}
	return menuForActions
}

func (a *Actions) ManualMovePlayer(direction geometry.Point) {
	state := a.state
	ui := a.ui()
	player := state.GetPlayer()
	if !player.IsAlive() {
		return
	}
	ui.CancelTargeting()
	a.CancelAutoMove()
	//g.actionBroker.CancelAllAnimations()

	if player.HasStatusEffect(common.StatusConfused) {
		if rand.Intn(3) != 0 { // one in three chance to succeed with movement
			// pick a different random direction..
			possibleDirectionsRemaining := state.GetMap().GetFilteredDirectionsForMovement(func(dir geometry.Point) bool {
				if dir == direction {
					return false
				}
				return true
			})
			direction = possibleDirectionsRemaining[rand.Intn(len(possibleDirectionsRemaining))]
		}
	}

	playerAction := common.PlayerMoveToAttack(player, direction, a.ui().IsPlayerForcingAggression(), a.getActionContext())
	if playerAction == nil {
		return
	}

	attackType, isAttack := common.AttackTypeOf(playerAction)
	if isAttack {
		a.executeAttack(playerAction, attackType)
		return
	}
	a.actionBroker.Execute(playerAction)
	a.actionBroker.Execute(common.NewEndPlayerTurnAction(playerAction.Cause(), playerAction.GetDelayInAut()))
}

func (a *Actions) getActionContext() common.ActionContext {
	return common.ActionContext{
		GameState:     a.state,
		UserInterface: a.ui,
		AudioPlayer:   a.audio,
		Data:          a.definitions,
		ActionBroker:  a.actionBroker,
	}
}

func (a *Actions) nextStepOnPlayerPath() {
	state := a.state
	msg := a.logError
	if !state.GetPlayer().IsAlive() {
		a.stopAutoMove()
		return
	}
	if a.IsPlayerAutoMoving() {
		if state.GetVisibility().GetVisibleEnemyCount(func(actor *common.Actor) bool {
			return actor.IsHostile() && actor.IsHostileTo(state.GetPlayer())
		}) > 0 {
			a.stopAutoMove()
			msg("You can see an enemy")
			return
		}

		// pop first element
		nextStep := a.playerPath[0]

		if state.GetMap().IsActorAt(nextStep) {
			//actorAt := state.GetMap().ActorAt(nextStep)
			a.stopAutoMove()
			if !a.autoExplore { // never explore into another actor, only deliberately
				direction := nextStep.Sub(state.GetPlayer().MapPosition())
				a.ManualMovePlayer(direction)
			}
			return
		}

		if state.GetMap().IsObjectAt(nextStep) {
			if a.state.IsClosedDoorAt(nextStep) {
				openDoorAction := common.NewOpenDoorAction(common.AbstractCause("Player Movement"), state.GetPlayer(), nextStep)
				a.actionBroker.QueueForExecution(openDoorAction)
				a.actionBroker.QueueForExecution(common.NewEndPlayerTurnAction(openDoorAction.Cause(), openDoorAction.GetDelayInAut()))
				return
			} else if !a.autoExplore || a.config.AutoExploreOpenContainers {
				objectAt := state.GetMap().ObjectAt(nextStep)
				if !objectAt.IsWalkable(state.GetPlayer()) {
					a.stopAutoMove()
					msg(fmt.Sprintf("You bump into %s", objectAt.Name()))
					objectAt.ActorBumpedInto(state.GetPlayer(), common.AnimateNow())
					if o, ok := objectAt.(*common.D1Object); ok && o.UsesTurn(state.GetPlayer()) {
						a.actionBroker.QueueForExecution(common.NewEndPlayerTurnAction(common.AbstractCause("Operate"), 10))
					}
					return
				}
			}
		}

		if !state.GetMap().IsCurrentlyPassable(nextStep) {
			a.stopAutoMove()
			msg("You can't move there")
			return
		}

		a.playerPath = a.playerPath[1:]
		autoMoveAction := common.NewMoveAction(common.AbstractCause("Player Movement"), state.GetPlayer(), nextStep, true)
		autoMoveAction.SetAnimationDelay(0.08)
		a.actionBroker.QueueForExecution(autoMoveAction)
		a.actionBroker.QueueForExecution(common.NewEndPlayerTurnAction(autoMoveAction.Cause(), autoMoveAction.GetDelayInAut()))
	} else if a.autoExplore {
		if target, hasTarget := a.getNextAutoExploreTarget(); hasTarget {
			a.continuePlayerAutoMovement(target)
		} else {
			msg("Nothing left to explore")
			a.stopAutoMove()
		}
	}
}
func (a *Actions) continuePlayerAutoMovement(targetPos geometry.Point) {
	currentMap := a.state.GetMap()
	var isWalkable func(point geometry.Point) bool
	if a.autoExplore {
		isWalkable = a.allowedForAutoExplore(targetPos)
	} else {
		isWalkable = a.allowedForAutoMove(targetPos)
	}
	pathToTarget := currentMap.GetJPSPath(a.state.GetPlayer().MapPosition(), targetPos, isWalkable)
	if len(pathToTarget) <= 1 {
		return
	}
	a.playerPath = pathToTarget[1:]
	a.nextStepOnPlayerPath()
}
func (a *Actions) allowedForAutoExplore(targetPos geometry.Point) func(point geometry.Point) bool {
	return func(point geometry.Point) bool {
		currentMap := a.state.GetMap()
		isWalkable := currentMap.CurrentlyPassableAndSafeForActor(a.state.GetPlayer())
		if isWalkable(point) && !currentMap.IsStairsAt(point) {
			return true
		}
		// the explore target itself may be an unopened container to bump (Open Containers option)
		return a.config.AutoExploreOpenContainers && point == targetPos && currentMap.IsObjectAt(point)
	}
}
func (a *Actions) allowedForAutoMove(targetPos geometry.Point) func(point geometry.Point) bool {
	return func(point geometry.Point) bool {
		currentMap := a.state.GetMap()
		isWalkable := currentMap.CurrentlyPassableAndSafeForActor(a.state.GetPlayer())
		return isWalkable(point) || point == targetPos
	}
}

func (a *Actions) StartPlayerAutoMovement(targetPos geometry.Point) {
	player := a.state.GetPlayer()
	if !player.IsAlive() {
		return
	}
	currentMap := a.state.GetMap()

	// single step, like a manual move..
	if currentMap.MoveDistance(player.MapPosition(), targetPos) == 1 {
		// this seems to be an intended action..
		direction := targetPos.Sub(player.MapPosition())
		a.ManualMovePlayer(direction)
		return
	}

	if len(a.state.GetVisibility().GetVisibleEnemies()) > 0 {
		// player is trying to move while enemies are visible..
		a.logError("You can't run while enemies are visible")
		return
	}

	if !player.CanAutoMove() {
		a.logError("You can't move in a coordinated fashion right now")
		return
	}

	playerPosition := player.MapPosition()

	// check for item directly beneath us.. if so, pick it up
	if a.config.AutoPickup {
		if currentMap.IsItemAt(playerPosition) {
			itemAt := currentMap.ItemAt(playerPosition)
			if !itemAt.IsDroppedByPlayer() {
				pickUpAction := common.NewPickUpAction(common.AbstractCause("PickUp"), player)
				a.actionBroker.Execute(pickUpAction)
				a.actionBroker.Execute(common.NewEndPlayerTurnAction(pickUpAction.Cause(), pickUpAction.GetDelayInAut()))
			}
		}
	}

	var isWalkable func(point geometry.Point) bool
	if a.autoExplore {
		isWalkable = a.allowedForAutoExplore(targetPos)
	} else {
		isWalkable = a.allowedForAutoMove(targetPos)
	}
	pathToTarget := currentMap.GetJPSPath(playerPosition, targetPos, isWalkable)
	if len(pathToTarget) <= 1 {
		a.stopAutoMove()
		return
	}
	a.animator.CancelAllAnimations()
	a.actionBroker.ClearQueuedActions(func(action common.Action) bool {
		if moveAction, ok := action.(*common.MoveAction); ok {
			return moveAction.GetActor() == player
		}
		return false
	})
	a.playerPath = pathToTarget[1:]
	a.nextStepOnPlayerPath()
}

func (a *Actions) getNextAutoExploreTarget() (geometry.Point, bool) {
	state := a.state
	player := state.GetPlayer()
	playerPos := player.MapPosition()

	if a.config.AutoPickup {
		nearestItem := state.GetMap().GetNearestExploredItem(playerPos, func(item *common.Item) bool {
			return player.CanPickup(item) && !item.IsDroppedByPlayer() && (!item.IsHidden() || state.GetFlags().IsSet(string(common.FlagHasTrueSight)))
		})
		if nearestItem != playerPos {
			return nearestItem, true
		}
	}

	if a.config.AutoExploreOpenContainers {
		nearestContainer := state.GetMap().GetNearestExploredObject(playerPos, func(object common.Object) bool {
			container, ok := object.(*common.D1Object)
			return ok && container.IsUnlootedContainer()
		})
		if nearestContainer != playerPos {
			return nearestContainer, true
		}
	}

	unexplored := state.GetMap().GetNearestUnexploredWalkablePosition(playerPos)
	if unexplored != playerPos {
		return unexplored, true
	}
	return geometry.Point{}, false
}

func (a *Actions) CancelAutoMove() {
	a.stopAutoMove()
	a.animator.CancelAllAnimations()

}
func (a *Actions) stopAutoMove() {
	state := a.state
	a.actionBroker.ClearQueuedActions(func(action common.Action) bool {
		if moveAction, ok := action.(*common.MoveAction); ok {
			return moveAction.GetActor() == state.GetPlayer()
		}
		return false
	})

	a.autoExplore = false
	a.playerPath = nil
}

func (a *Actions) StartAutoExplore() {
	if !a.state.GetPlayer().CanAutoMove() {
		a.logError("You can't explore right now")
		return
	}
	target, hasTarget := a.getNextAutoExploreTarget()
	if a.autoExplore {
		a.stopAutoMove()
		a.logError("You stop exploring")
		return
	}
	if hasTarget {
		a.logError("You start exploring")
		a.autoExplore = true
		a.StartPlayerAutoMovement(target)
	} else {
		a.logError("Nothing to explore")
	}
}

func (a *Actions) goOneLevelUp() {
	state := a.state
	if !state.GetPlayer().IsAlive() {
		return
	}
	currentMap := state.GetMap()
	mapName := currentMap.GetName()
	if mapName == string(defs.LevelNameTown) {
		return
	}

	ourPos := state.GetPlayer().MapPosition()
	nearestStairsUp := currentMap.GetNearestTile(ourPos, func(tile gridmap.Tile) bool {
		return tile.IsStairsUp()
	})

	if nearestStairsUp == ourPos {
		a.logError("No known path")
		return
	}
	a.CancelAutoMove()
	a.StartPlayerAutoMovement(nearestStairsUp)
}

func (a *Actions) goOneLevelDown() {
	state := a.state
	if !state.GetPlayer().IsAlive() {
		return
	}
	currentMap := state.GetMap()
	//mapName := currentMap.Action()

	ourPos := state.GetPlayer().MapPosition()
	nearestStairsDown := currentMap.GetNearestTile(ourPos, func(tile gridmap.Tile) bool {
		return tile.IsStairsDown()
	})
	if nearestStairsDown == ourPos {
		a.logError("No known path")
		return
	}
	a.CancelAutoMove()
	a.StartPlayerAutoMovement(nearestStairsDown)
}

func (a *Actions) QuickRangedAttack() {
	state := a.state
	if !state.GetPlayer().IsAlive() {
		return
	}
	if !state.GetPlayer().GetEquipment().HasRangedWeaponEquipped() {
		a.logError("You don't have a ranged weapon equipped")
		return
	}
	rangedAttackRuleProvider := common.NewRangedAttackTargetingRules(util.Arguments{})
	rangedAttackRules := rangedAttackRuleProvider.GetTargetRules(state.GetPlayer(), common.OneTargetTile(state.GetMap, util.Arguments{}), state.GetMap())
	target := rangedAttackRules.GetPreSelectedTarget()
	if target == state.GetPlayer().MapPosition() {
		a.logError("You can't attack yourself like that")
		return
	}
	a.playerRangedAttack(state.GetPlayer(), target)
	return
}

// CastLastSpellAtNearest recasts the last spell picked from the spell menu or skill bar at the nearest hostile actor.
func (a *Actions) CastLastSpellAtNearest() {
	state := a.state
	player := state.GetPlayer()
	if !player.IsAlive() {
		return
	}
	targeted, untargeted := player.LastCastSpell()
	switch {
	case targeted != nil:
		target := targeted.GetTargetingRules(player, state.GetMap()).GetPreSelectedTarget()
		targeted.OnSelected(player, target, targeted.GetHitPositions(player.MapPosition(), target))
	case untargeted != nil:
		a.activateUntargetedAction(player, untargeted, common.EffectSource(untargeted.GetInternalName()))
	default:
		a.logError("No spell to recast")
	}
}

func (a *Actions) AimedRangedAttack() {
	state := a.state
	if !state.GetPlayer().IsAlive() {
		return
	}
	if !state.GetPlayer().GetEquipment().HasRangedWeaponEquipped() {
		a.logError("You don't have a ranged weapon equipped")
		return
	}
	a.defaultRangedTargeting(a.playerRangedAttack) // execute ranged attack action and end turn)
	return
}
func (a *Actions) defaultRangedTargeting(onTargetSelected func(player *common.Actor, targetPos geometry.Point)) {
	state := a.state
	rangedAttackRuleProvider := common.NewRangedAttackTargetingRules(util.Arguments{})
	rangedAttackRules := rangedAttackRuleProvider.GetTargetRules(state.GetPlayer(), common.OneTargetTile(state.GetMap, util.Arguments{}), state.GetMap())
	a.beginRangedTargeting(rangedAttackRules, onTargetSelected)
}
func (a *Actions) beginRangedTargeting(targetRules common.TargetRules, onSelected func(attacker *common.Actor, target geometry.Point)) {
	state := a.state
	if !state.GetPlayer().IsAlive() {
		return
	}
	onHit := &common.HitEffect{
		Activate: func(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
			origin := state.GetPlayer().MapPosition()
			if origin == target {
				a.logError("You can't attack yourself like that")
				return
			}
			onSelected(attacker, target)
		},
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Ranged attack")),
		},
	}
	a.ui().BeginTargeting(targetRules, onHit)
}

func (a *Actions) AimedThrow() {
	state := a.state
	player := state.GetPlayer()
	if !player.IsAlive() {
		return
	}

	equipment := player.GetEquipment()
	inventory := player.GetInventory()
	if !equipment.HasThrowableQuivered() {
		userInterface := a.ui()
		throwFilter := func(element *common.Item) bool {
			return element.IsThrowable()
		}
		if len(inventory.StackedItemsWithFilter(throwFilter)) == 0 {
			a.logError("You have nothing to throw.")
			return
		}
		onSelected := func(itemKey rune) {
			userInterface.PopAll()
			thrownItem := inventory.GetItemByLetter(itemKey)
			throwComponent := thrownItem.GetThrowStats()
			targetRulesProvider := common.NewRangedAttackTargetingRules(util.Arguments{"throw"})
			rangedAttackRules := targetRulesProvider.GetTargetRules(player, throwComponent.HitZone, state.GetMap())
			a.beginRangedTargeting(rangedAttackRules, a.playerThrow(thrownItem))
		}
		userInterface.OpenListInventoryForSelection(throwFilter, onSelected)
	} else {
		thrownItem := equipment.GetNextQuiveredThrowable()
		throwComponent := thrownItem.GetThrowStats()
		targetRulesProvider := common.NewRangedAttackTargetingRules(util.Arguments{"throw"})
		rangedAttackRules := targetRulesProvider.GetTargetRules(player, throwComponent.HitZone, state.GetMap())

		a.beginRangedTargeting(rangedAttackRules, a.playerThrow(thrownItem))
	}
}

func (a *Actions) AutoAttack() {
	a.stopAutoMove()
	state := a.state
	gui := a.ui()
	if !state.GetPlayer().IsAlive() {
		return
	}
	var visibleEnemies []*common.Actor

	if gui.IsPlayerForcingAggression() {
		for actor, _ := range state.GetVisibility().GetVisibleActors() {
			visibleEnemies = append(visibleEnemies, actor)
		}
	} else {
		visibleEnemies = state.GetVisibility().GetVisibleEnemies()
	}

	if len(visibleEnemies) == 0 {
		return
	}

	player := state.GetPlayer()
	hasRanged := player.GetEquipment().HasRangedWeaponEquipped()
	hasMelee := player.GetEquipment().HasMeleeWeaponEquipped()
	slices.SortStableFunc(visibleEnemies, func(i, j *common.Actor) int {
		distI := i.MapPosition().ManhattanDistanceTo(player.MapPosition())
		distJ := j.MapPosition().ManhattanDistanceTo(player.MapPosition())
		return cmp.Compare(distI, distJ)
	})
	currentMap := state.GetMap()
	for _, nearestEnemy := range visibleEnemies {
		dist := currentMap.MoveDistance(player.MapPosition(), nearestEnemy.MapPosition())
		if dist <= 1 {
			if hasMelee || !hasRanged {
				a.playerMeleeAttack(player, nearestEnemy)
				return
			} else if hasRanged {
				a.playerRangedAttack(player, nearestEnemy.MapPosition())
				return
			}
		}

		if hasRanged {
			a.playerRangedAttack(player, nearestEnemy.MapPosition())
			return
		}

		// do melee
		freeAttackPositions := currentMap.GetFilteredNeighborsForMovement(nearestEnemy.MapPosition(), func(point geometry.Point) bool {
			return currentMap.IsCurrentlyPassable(point)
		})

		if len(freeAttackPositions) == 0 {
			continue
		}
		distMap := make([]util.Tuple[geometry.Point, int], len(freeAttackPositions))
		for _, freeAttackPosition := range freeAttackPositions {
			distanceTo := currentMap.MoveDistance(player.MapPosition(), freeAttackPosition)
			distMap = append(distMap, util.NewTuple(freeAttackPosition, distanceTo))
		}
		slices.SortStableFunc(distMap, func(i, j util.Tuple[geometry.Point, int]) int {
			return cmp.Compare(i.GetItem2(), j.GetItem2())
		})

		for _, distTuple := range distMap {
			targetPos := distTuple.GetItem1()
			walkPath := currentMap.GetJPSPath(player.MapPosition(), targetPos, currentMap.CurrentlyPassableAndSafeForActor(player))
			if len(walkPath) <= 1 {
				continue
			}
			// remove first pos
			walkPath = walkPath[1:]
			firstStep := walkPath[0]

			a.playerMoveOneStep(player, firstStep)
			return
		}
	}

}

func (a *Actions) playerMoveOneStep(player *common.Actor, targetPos geometry.Point) {
	moveAction := common.NewMoveAction(common.NewActorCause("Move", player), player, targetPos, true)
	a.actionBroker.Execute(moveAction)
	a.actionBroker.Execute(common.NewEndPlayerTurnAction(moveAction.Cause(), moveAction.GetDelayInAut()))
}

// executeAttack runs action and ends the turn, unless it would destroy an already-broken equipped weapon, in
// which case the player is asked to confirm first (Gameplay menu > Warning: Weapon breaking).
func (a *Actions) executeAttack(action common.Action, attackType common.AttackType) {
	proceed := func() {
		a.actionBroker.Execute(action)
		a.actionBroker.Execute(common.NewEndPlayerTurnAction(action.Cause(), action.GetDelayInAut()))
	}
	if brokenWeapon := common.WeaponAboutToBreak(a.player, attackType); brokenWeapon != nil && a.config.WeaponBreakWarning {
		a.ui().Confirm(fmt.Sprintf("Your %s will break. Attack anyway?", brokenWeapon.Name()), proceed)
		return
	}
	proceed()
}

func (a *Actions) playerMeleeAttack(player *common.Actor, defender *common.Actor) {
	meleeAction := common.NewMeleeAttackAction(common.NewActorCause("Melee attack", player), player, defender)
	a.executeAttack(meleeAction, common.MeleeAttack)
}
func (a *Actions) playerRangedAttack(player *common.Actor, targetPos geometry.Point) {
	rangedAttackAction := common.NewRangedAttackAction(common.NewActorCause("Ranged attack", player), player, targetPos)
	/* TODO
	hitEffects := player.GetRangedHitEffects()
	if len(hitEffects) > 0 {
		rangedAttackAction.SetAdditionalOnHitActions(func(attacker *common.Actor, target geometry.Point, hitPositions ) []common.Action {
			for _, effect := range hitEffects {
				effect.Activate(attacker, target, hitPositions)
			}
		})
	}

	*/
	a.executeAttack(rangedAttackAction, common.RangedAttack)
}

func (a *Actions) RestUntilHealed() {
	state := a.state
	if len(state.GetVisibility().GetVisibleEnemies()) > 0 {
		a.logError("You can't rest with enemies nearby")
		return
	}
	/*
		hpRegenRate := state.GetPlayer().GetStats().GetAttributeAsFloat(stats.HealthRegeneration)
		if hpRegenRate <= 0 {
			a.isResting = false
			a.logError("You won't heal through resting")
			return
		}
	*/
	healthResource := state.GetPlayer().GetStats().GetResource(stats.Health)
	if healthResource.IsAtMax() {
		a.isResting = false
		if a.isResting {
			a.logError("Your spirits have been restored")
		} else {
			a.logError("You are already at full health")
		}
		return
	}

	playerInventory := state.GetPlayer().GetInventory()
	if !playerInventory.HasFood() {
		a.isResting = false
		a.logError("You have no food to eat")
		return
	}

	playerInventory.UseFood()

	a.isResting = true
	userInterface := a.ui()
	userInterface.FadeToBlack()
	a.queueAfterTurn(func() {
		a.isResting = false
		userInterface.FadeFromBlack()
		a.actionBroker.QueueForExecution(common.NewChangeResourceAction(common.AbstractCause("Resting"), state.GetPlayer(), stats.Health, float64(healthResource.GetMax())))
	})
	a.actionBroker.QueueForExecution(common.NewEndPlayerTurnAction(common.AbstractCause("Resting"), 10))
}
func (a *Actions) SetQueueAfterTurn(afterTurn func(call func())) {
	a.queueAfterTurn = afterTurn
}
// ShowObjectInfoAt opens the description popup for a shrine at mapPos, analogous to ShowActorInfoAt. No-op for
// anything that isn't a previewable shrine.
func (a *Actions) ShowObjectInfoAt(mapPos geometry.Point) {
	obj, ok := a.state.GetMap().ObjectAt(mapPos).(*common.D1Object)
	if !ok {
		return
	}
	lines, ok := common.D1ShrineInfoText(obj)
	if !ok {
		return
	}
	a.ui().OpenTextWindow(util.ToLabelText(lines))
}
func (a *Actions) ShowActorInfoAt(mapPos geometry.Point) {
	state := a.state
	player := state.GetPlayer()
	actor := state.GetMap().ActorAt(mapPos)
	infoText := util.ToLabelText(actor.GetInfoText())
	if actor.D1Monster() != nil {
		infoText = append(infoText, util.ToLabelText(common.D1CombatForecast(state, player, actor))...)
		a.ui().OpenTextWindow(infoText)
		return
	}
	if actor != player && actor.IsHostile() {
		otherHasRanged := actor.GetEquipment().HasRangedWeaponEquipped()
		playerHasRanged := player.GetEquipment().HasRangedWeaponEquipped()

		otherMeleeDamage := actor.GetDamageDice(common.MeleeAttack)

		expectedMeleeDamage := otherMeleeDamage.ExpectedValue()
		maxMeleeDamage := otherMeleeDamage.Max()

		expectedMeleeDamageReduced := common.CalculateMeleeDamage(expectedMeleeDamage, 1.0, actor, player)
		maxMeleeDamageReduced := common.CalculateMeleeDamage(maxMeleeDamage, 1.0, actor, player)

		if maxMeleeDamageReduced == 0 {
			infoText = append(infoText, util.LabelTextFromString(""),
				util.LabelTextFromStringWithColor("This creature will not be able to penetrate", defs.Red),
				util.LabelTextFromStringWithColor("your armor with a regular attack.", defs.Red),
			)
		} else if expectedMeleeDamageReduced == 0 {
			infoText = append(infoText, util.LabelTextFromString(""),
				util.LabelTextFromStringWithColor("It will be hard for this creature", defs.Red),
				util.LabelTextFromStringWithColor("to penetrate your armor with a regular attack.", defs.Red),
			)
			infoText = append(infoText, util.LabelTextFromString(""), util.LabelTextFromStringWithColor(fmt.Sprintf("Enemy could at worst hit for %d damage per melee attack,", maxMeleeDamageReduced), defs.Red))
			infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("that's %d%% of your current health,", maxMeleeDamageReduced*100/player.GetStats().GetResource(stats.Health).Value()), defs.Red))
			hitsForMeleeKill := int(math.Ceil(float64(player.GetStats().GetResource(stats.Health).Value()) / float64(maxMeleeDamageReduced)))
			infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("and at worst could defeat you in %d hits.", hitsForMeleeKill), defs.Red))
		} else {
			infoText = append(infoText, util.LabelTextFromString(""), util.LabelTextFromStringWithColor(fmt.Sprintf("Enemy typically hits for %d damage per melee attack,", expectedMeleeDamageReduced), defs.Red))
			infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("that's %d%% of your current health,", expectedMeleeDamageReduced*100/player.GetStats().GetResource(stats.Health).Value()), defs.Red))
			hitsForMeleeKill := int(math.Ceil(float64(player.GetStats().GetResource(stats.Health).Value()) / float64(maxMeleeDamageReduced)))
			infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("and at worst could defeat you in %d hits.", hitsForMeleeKill), defs.Red))
		}

		if otherHasRanged {
			otherRangedDamage := actor.GetDamageDice(common.RangedAttack)

			expectedRangedDamage := otherRangedDamage.ExpectedValue()
			maxRangedDamage := otherRangedDamage.Max()

			expectedRangedDamageReduced := common.CalculateRangedDamage(expectedRangedDamage, 1.0, actor, player)
			maxRangedDamageReduced := common.CalculateRangedDamage(maxRangedDamage, 1.0, actor, player)

			if maxRangedDamageReduced == 0 {
				infoText = append(infoText, util.LabelTextFromString(""),
					util.LabelTextFromStringWithColor("This creature will not be able to penetrate", defs.Red),
					util.LabelTextFromStringWithColor("your armor with a ranged attack.", defs.Red),
				)
			} else if expectedRangedDamageReduced == 0 {
				infoText = append(infoText, util.LabelTextFromString(""),
					util.LabelTextFromStringWithColor("It will be hard for this creature", defs.Red),
					util.LabelTextFromStringWithColor("to penetrate your armor with a ranged attack.", defs.Red),
				)
				infoText = append(infoText, util.LabelTextFromString(""), util.LabelTextFromStringWithColor(fmt.Sprintf("Enemy could at worst hit for %d damage per ranged attack,", maxRangedDamageReduced), defs.Red))
				infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("that's %d%% of your current health,", maxRangedDamageReduced*100/player.GetStats().GetResource(stats.Health).Value()), defs.Red))
				hitsForRangedKill := int(math.Ceil(float64(player.GetStats().GetResource(stats.Health).Value()) / float64(maxRangedDamageReduced)))
				infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("and at worst could defeat you in %d hits.", hitsForRangedKill), defs.Red))
			} else {
				infoText = append(infoText, util.LabelTextFromString(""), util.LabelTextFromStringWithColor(fmt.Sprintf("Enemy typically hits for %d damage per ranged attack,", expectedRangedDamageReduced), defs.Red))
				infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("that's %d%% of your current health,", expectedRangedDamageReduced*100/player.GetStats().GetResource(stats.Health).Value()), defs.Red))
				hitsForRangedKill := int(math.Ceil(float64(player.GetStats().GetResource(stats.Health).Value()) / float64(maxRangedDamageReduced)))
				infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("and at worst could defeat you in %d hits.", hitsForRangedKill), defs.Red))
			}
		}

		playerMeleeDamage := player.GetDamageDice(common.MeleeAttack)

		playerExpectedMeleeDamage := playerMeleeDamage.ExpectedValue()
		playerMaxMeleeDamage := playerMeleeDamage.Max()

		playerExpectedMeleeDamageReduced := common.CalculateMeleeDamage(playerExpectedMeleeDamage, 1.0, player, actor)
		playerMaxMeleeDamageReduced := common.CalculateMeleeDamage(playerMaxMeleeDamage, 1.0, player, actor)

		if playerMaxMeleeDamageReduced == 0 {
			infoText = append(infoText, util.LabelTextFromString(""),
				util.LabelTextFromStringWithColor("You will not be able to penetrate", defs.DarkGreen),
				util.LabelTextFromStringWithColor("the enemy's armor with a regular attack.", defs.DarkGreen),
			)
		} else if playerExpectedMeleeDamageReduced == 0 {
			infoText = append(infoText, util.LabelTextFromString(""),
				util.LabelTextFromStringWithColor("It will be hard for you", defs.DarkGreen),
				util.LabelTextFromStringWithColor("to penetrate the enemy's armor with a regular attack.", defs.DarkGreen),
			)
			infoText = append(infoText, util.LabelTextFromString(""), util.LabelTextFromStringWithColor(fmt.Sprintf("You could at best hit for %d damage per melee attack,", playerMaxMeleeDamageReduced), defs.DarkGreen))
			infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("that's %d%% of the enemy's current health,", playerMaxMeleeDamageReduced*100/actor.GetStats().GetResource(stats.Health).Value()), defs.DarkGreen))
			hitsForMeleeKill := int(math.Ceil(float64(actor.GetStats().GetResource(stats.Health).Value()) / float64(playerMaxMeleeDamageReduced)))
			infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("and at best could defeat the enemy in %d hits.", hitsForMeleeKill), defs.DarkGreen))
		} else {
			infoText = append(infoText, util.LabelTextFromString(""), util.LabelTextFromStringWithColor(fmt.Sprintf("You typically hit for %d damage per melee attack,", playerExpectedMeleeDamageReduced), defs.DarkGreen))
			infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("that's %d%% of the enemy's current health,", playerExpectedMeleeDamageReduced*100/actor.GetStats().GetResource(stats.Health).Value()), defs.DarkGreen))
			hitsForMeleeKill := int(math.Ceil(float64(actor.GetStats().GetResource(stats.Health).Value()) / float64(playerMaxMeleeDamageReduced)))
			infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("and at best could defeat the enemy in %d hits.", hitsForMeleeKill), defs.DarkGreen))
		}

		if playerHasRanged {

			playerHitChance := common.CalculateRangedWeaponHitChance(player, actor)

			infoText = append(infoText, util.LabelTextFromString(""), util.LabelTextFromStringWithColor(fmt.Sprintf("You have a %d%% chance to hit with a ranged attack.", int(playerHitChance*100)), defs.DarkGreen))

			playerRangedDamage := player.GetDamageDice(common.RangedAttack)

			playerExpectedRangedDamage := playerRangedDamage.ExpectedValue()
			playerMaxRangedDamage := playerRangedDamage.Max()

			playerExpectedRangedDamageReduced := common.CalculateRangedDamage(playerExpectedRangedDamage, 1.0, player, actor)
			playerMaxRangedDamageReduced := common.CalculateRangedDamage(playerMaxRangedDamage, 1.0, player, actor)

			if playerMaxRangedDamageReduced == 0 {
				infoText = append(infoText, util.LabelTextFromString(""),
					util.LabelTextFromStringWithColor("You will not be able to penetrate", defs.DarkGreen),
					util.LabelTextFromStringWithColor("the enemy's armor with a ranged attack.", defs.DarkGreen),
				)
			} else if playerExpectedRangedDamageReduced == 0 {
				infoText = append(infoText, util.LabelTextFromString(""),
					util.LabelTextFromStringWithColor("It will be hard for you", defs.DarkGreen),
					util.LabelTextFromStringWithColor("to penetrate the enemy's armor with a ranged attack.", defs.DarkGreen),
				)
				infoText = append(infoText, util.LabelTextFromString(""), util.LabelTextFromStringWithColor(fmt.Sprintf("You could at best hit for %d damage per ranged attack,", playerMaxRangedDamageReduced), defs.DarkGreen))
				infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("that's %d%% of the enemy's current health,", playerMaxRangedDamageReduced*100/actor.GetStats().GetResource(stats.Health).Value()), defs.DarkGreen))
				hitsForRangedKill := int(math.Ceil(float64(actor.GetStats().GetResource(stats.Health).Value()) / float64(playerMaxRangedDamageReduced)))
				infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("and at best could defeat the enemy in %d hits.", hitsForRangedKill), defs.DarkGreen))
			} else {
				infoText = append(infoText, util.LabelTextFromString(""), util.LabelTextFromStringWithColor(fmt.Sprintf("You typically hit for %d damage per ranged attack,", playerExpectedRangedDamageReduced), defs.DarkGreen))
				infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("that's %d%% of the enemy's current health,", playerExpectedRangedDamageReduced*100/actor.GetStats().GetResource(stats.Health).Value()), defs.DarkGreen))
				hitsForRangedKill := int(math.Ceil(float64(actor.GetStats().GetResource(stats.Health).Value()) / float64(playerMaxRangedDamageReduced)))
				infoText = append(infoText, util.LabelTextFromStringWithColor(fmt.Sprintf("and at best could defeat the enemy in %d hits.", hitsForRangedKill), defs.DarkGreen))
			}
		}
	}

	a.ui().OpenTextWindow(infoText)
}

func (a *Actions) updateFoV() {
	state := a.state
	player := state.GetPlayer()
	a.actionBroker.Execute(common.NewUpdateFoVAction(common.AbstractCause("FoV Update"), player))
}

func (a *Actions) IsPlayerAutoMoving() bool {
	return len(a.playerPath) > 0
}
