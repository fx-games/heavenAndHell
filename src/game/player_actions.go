package game

import (
	"cmp"
	"fmt"
	"rpg/src/audio"
	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"rpg/src/stats"
	"rpg/src/util"
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

	getTimeKeeper   func() *common.TimeKeeper
	logError        func(message string)
	autoExplore     bool
	playerPath      []geometry.Point
	isResting       bool
	queueAfterTurn  func(call func())
	lastSpellTarget *common.Actor // recast prefers it while it stays a valid target
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
	if rangedAttackRules.GetPreSelectedTarget() == a.player.MapPosition() && a.hostileInSight() {
		a.logError("No clear path to any target")
	}
	onHit := &common.HitEffect{
		Activate: func(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
			if chargedItem != nil {
				chargedItem.UseCharge()
				if !chargedItem.HasChargesLeft() && chargedItem.SelfDestructs() {
					state.GetPlayer().GetInventory().Remove(chargedItem)
				}
			}
			act.CancelAutoMove()
			a.rememberSpellTarget(target)
			action.OnSelected(attacker, target, hitPositions)
		},
		Description: action.GetDescription(),
	}
	a.ui().BeginTargeting(rangedAttackRules, onHit)
}

func (a *Actions) PickUpItem() {
	state := a.state
	player := state.GetPlayer()
	item, ok := state.GetMap().TryGetItemAt(player.MapPosition())
	if !player.IsAlive() || !ok {
		return
	}
	if !player.CanPickup(item) {
		state.Factory().PlayHeroSound(14 + common.Random().Intn(3)) // AutoGetItem: "I can't carry any more"
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
	if d := item.Roll(); d != nil && !item.MeetsRequirements() {
		a.logError(fmt.Sprintf("You can't equip %s: it needs %d Str, %d Mag and %d Dex.", item.Name(), d.MinStr, d.MinMag, d.MinDex))
		a.state.Factory().PlayHeroSound(13)
		return
	}
	common.PlayUICue("item_equip")
	a.actionBroker.Execute(common.NewEquipItemAction(common.AbstractCause("equip"), a.player, item))
	a.actionBroker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("equip"), 10)) // not D1's (instant): a belt swap isn't free
}

func (a *Actions) UnequipItem(item *common.Item) {
	common.PlayUICue("item_unequip")
	a.actionBroker.Execute(common.NewUnequipItemAction(common.AbstractCause("unequip"), a.player, item))
	a.actionBroker.Execute(common.NewEndPlayerTurnAction(common.AbstractCause("unequip"), 10))
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
	if d := itemWithAction.Roll(); d != nil && d.Type == rules.ITypeStaff {
		user.SetLastCastItem(itemWithAction)
	}

	isStaff := itemWithAction.Roll() != nil && itemWithAction.Roll().Type == rules.ITypeStaff // staves cast, not UseInvItem
	if itemWithAction.HasTargetedAction() {
		if !isStaff {
			a.actionBroker.Emit(common.ItemUsed{Item: itemWithAction, User: user})
		}
		a.beginActionTargeting(itemWithAction.GetTargetedAction(), itemWithAction)
	} else if itemWithAction.HasUntargetedAction() {

		untargetedAction := itemWithAction.GetUntargetedAction()
		if !untargetedAction.CanActivate(user) {
			return // e.g. a book that needs more magic than the reader has: not consumed
		}
		if !isStaff {
			a.actionBroker.Emit(common.ItemUsed{Item: itemWithAction, User: user})
		}

		itemWithAction.UseCharge()
		if !itemWithAction.HasChargesLeft() && itemWithAction.SelfDestructs() {
			user.GetInventory().Remove(itemWithAction)
		}

		a.activateUntargetedAction(user, untargetedAction, common.EffectSource(itemWithAction.GetInternalName()))
		a.state.ItemUsed(itemWithAction)
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
	forced := a.ui().IsPlayerForcingAggression()
	action.Reset()
	if action.IsDelayed() {
		// add to timekeeper
		a.logError(fmt.Sprintf("%s is delayed for %d turns", action.Name(), action.GetDelayInTurns()))
		action.SetCurrentUserAndSource(actor, source, forced)
		timeKeeper := a.getTimeKeeper()
		timeKeeper.AddTimed(action)
	} else if action.CanActivate(actor) {
		action.Invoke(actor, source, forced)
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
		if common.Random().Intn(3) != 0 { // one in three chance to succeed with movement
			// pick a different random direction..
			possibleDirectionsRemaining := state.GetMap().GetFilteredDirectionsForMovement(func(dir geometry.Point) bool {
				return dir != direction
			})
			direction = possibleDirectionsRemaining[common.Random().Intn(len(possibleDirectionsRemaining))]
		}
	}

	playerAction := common.PlayerMoveToAttack(player, direction, a.ui().IsPlayerForcingAggression(), a.getActionContext())
	if playerAction == nil {
		return
	}

	a.executeAttack(playerAction)
}

func (a *Actions) getActionContext() common.ActionContext {
	return common.ActionContext{
		GameState:    a.state,
		Show:         func(r common.UIRequest) { show(a.ui(), r) },
		AudioPlayer:  a.audio,
		Data:         a.definitions,
		ActionBroker: a.actionBroker,
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

		if blocker, ok := state.GetMap().TryGetActorAt(nextStep); ok {
			if !a.playerPerceives(blocker) {
				msg("Something is in the way")
			}
			a.stopAutoMove()
			if !a.autoExplore { // never explore into another actor, only deliberately
				direction := nextStep.Sub(state.GetPlayer().MapPosition())
				a.ManualMovePlayer(direction)
			}
			return
		}

		if objectAt, ok := state.GetMap().TryGetObjectAt(nextStep); ok {
			if a.state.IsClosedDoorAt(nextStep) {
				openDoorAction := common.NewOpenDoorAction(common.AbstractCause("Player Movement"), state.GetPlayer(), nextStep)
				a.actionBroker.QueueForExecution(openDoorAction)
				a.actionBroker.QueueForExecution(common.NewEndPlayerTurnAction(openDoorAction.Cause(), openDoorAction.GetDelayInAut()))
				return
			} else if !a.autoExplore || a.config.AutoExploreOpenContainers {
				if !objectAt.IsWalkable(state.GetPlayer()) {
					a.stopAutoMove()
					msg(fmt.Sprintf("You bump into %s", objectAt.Name()))
					objectAt.ActorBumpedInto(state.GetPlayer())
					if o, ok := objectAt.(*common.Prop); ok && o.UsesTurn(state.GetPlayer()) {
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
		autoMoveAction.Step = true
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
		a.logError("No way on from here")
		a.stopAutoMove()
		return
	}
	a.playerPath = pathToTarget[1:]
	a.nextStepOnPlayerPath()
}
func (a *Actions) allowedForAutoExplore(targetPos geometry.Point) func(point geometry.Point) bool {
	return func(point geometry.Point) bool {
		currentMap, player := a.state.GetMap(), a.state.GetPlayer()
		isWalkable := currentMap.CurrentlyPassableAndSafeForActor(player)
		if isWalkable(point) && !currentMap.IsStairsAt(point) {
			return true
		}
		// an actor the player can't see is no wall to plan around: in the dark the path ran into one's cell, found
		// no way and exploring stopped without a word. The step itself bumps into it (nextStepOnPlayerPath).
		if blocker, ok := currentMap.TryGetActorAt(point); ok && !a.playerPerceives(blocker) && !currentMap.IsStairsAt(point) {
			return currentMap.IsWalkableFor(point, blocker) && !currentMap.IsObviousHazardAt(point)
		}
		// the explore target itself may be an unopened container to bump (Open Containers option)
		return a.config.AutoExploreOpenContainers && point == targetPos && currentMap.IsObjectAt(point)
	}
}
func (a *Actions) playerPerceives(actor *common.Actor) bool {
	return actor.IsVisible() && a.state.GetPlayer().CanPerceive(actor.MapPosition())
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

	// single step, like a manual move - but not for auto explore: a manual move cancels it, and in the dark the
	// next unexplored cell is usually the neighbor, so exploring stopped after every step
	if !a.autoExplore && currentMap.MoveDistance(player.MapPosition(), targetPos) == 1 {
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
		if itemAt, ok := currentMap.TryGetItemAt(playerPosition); ok {
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
		if a.autoExplore {
			a.logError("No way on from here")
		}
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
			container, ok := object.(*common.Prop)
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
	if state.IsPlayerInTown() {
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
}

// hostileInSight: a visible enemy the player could aim at, path or no path.
func (a *Actions) hostileInSight() bool {
	return a.state.GetMap().GetNearestActor(a.player.MapPosition(), func(e *common.Actor) bool {
		return e.IsVisible() && e.IsHostileTo(a.player) && a.player.CanSee(e.MapPosition()) && !e.IsVendor()
	}) != nil
}

func (a *Actions) rememberSpellTarget(target geometry.Point) {
	a.lastSpellTarget = nil
	if m := a.state.GetMap(); m.Contains(target) && m.IsActorAt(target) {
		a.lastSpellTarget = m.GetActor(target)
	}
}

// CastLastSpellAtNearest recasts the last spell picked from the spell menu or skill bar, or cast from a staff, at its last target if still valid, else the nearest hostile actor.
func (a *Actions) CastLastSpellAtNearest() {
	state := a.state
	player := state.GetPlayer()
	if !player.IsAlive() {
		return
	}
	targeted, untargeted, item := player.LastCastSpell()
	var source common.EffectSource
	if item != nil {
		if !player.GetInventory().Has(item) && !player.GetEquipment().IsEquipped(item) {
			a.logError("No spell to recast")
			return
		}
		if !item.HasChargesLeft() {
			a.logError(fmt.Sprintf("%s is out of charges", item.Name()))
			return
		}
		targeted, untargeted = item.GetTargetedAction(), item.GetUntargetedAction()
		source = common.EffectSource(item.GetInternalName())
	}
	var target geometry.Point
	if targeted != nil {
		rules := targeted.GetTargetingRules(player, state.GetMap())
		target = rules.GetPreSelectedTarget()
		if t := a.lastSpellTarget; t != nil && t.IsAlive() && slices.Contains(rules.Targets(), t.MapPosition()) {
			target = t.MapPosition()
		}
		if target == player.MapPosition() {
			if a.hostileInSight() {
				a.logError("No clear path to any target")
			} else {
				a.logError("No target in sight")
			}
			return
		}
	}
	if item != nil {
		item.UseCharge() // staves never self-destruct, so no removal like ActivateItemActionForPlayer
	}
	switch {
	case targeted != nil:
		a.rememberSpellTarget(target)
		targeted.OnSelected(player, target, targeted.GetHitPositions(player.MapPosition(), target))
	case untargeted != nil:
		if source == "" {
			source = common.EffectSource(untargeted.GetInternalName())
		}
		a.activateUntargetedAction(player, untargeted, source)
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
			util.LabelTextFromString("Ranged attack"),
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
		for actor := range state.GetVisibility().GetVisibleActors() {
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
			return cmp.Compare(i.Item2, j.Item2)
		})

		for _, distTuple := range distMap {
			targetPos := distTuple.Item1
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
	moveAction.Step = true
	a.actionBroker.Execute(moveAction)
	a.actionBroker.Execute(common.NewEndPlayerTurnAction(moveAction.Cause(), moveAction.GetDelayInAut()))
}

// executeAttack runs action and ends the turn.
func (a *Actions) executeAttack(action common.Action) {
	a.actionBroker.Execute(action)
	a.actionBroker.Execute(common.NewEndPlayerTurnAction(action.Cause(), action.GetDelayInAut()))
}

func (a *Actions) playerMeleeAttack(player *common.Actor, defender *common.Actor) {
	meleeAction := common.NewMeleeAttackAction(common.NewActorCause("Melee attack", player), player, defender)
	a.executeAttack(meleeAction)
}

// CloseDoorAt shuts the open door at mapPos when the player clicks it from next to it, as D1 does.
func (a *Actions) CloseDoorAt(mapPos geometry.Point) bool {
	player := a.state.GetPlayer()
	if !player.IsAlive() || !common.CanCloseDoorAt(a.state.GetMap(), player, mapPos) {
		return false
	}
	closeDoor := common.NewCloseDoorAction(common.AbstractCause("Close door"), player, mapPos)
	a.actionBroker.QueueForExecution(closeDoor)
	a.actionBroker.QueueForExecution(common.NewEndPlayerTurnAction(closeDoor.Cause(), closeDoor.GetDelayInAut()))
	return true
}

// RangedAttackAt fires the player's equipped ranged weapon at mapPos, if there is a clear shot at something worth
// shooting: a hostile actor, or an intact barrel. What a left-click on a distant enemy or barrel does; false (do
// nothing) for anything else, so the caller falls back to its normal click handling (move/auto-explore).
func (a *Actions) RangedAttackAt(mapPos geometry.Point) bool {
	state := a.state
	player := state.GetPlayer()
	currentMap := state.GetMap()
	if !player.IsAlive() || !player.GetEquipment().HasRangedWeaponEquipped() {
		return false
	}
	if !player.CanSee(mapPos) {
		return false
	}
	if currentMap.MoveDistance(player.MapPosition(), mapPos) <= 1 {
		return false // adjacent: let the normal click-to-move-into-melee handling take it
	}
	target, isActor := currentMap.TryGetActorAt(mapPos)
	switch {
	case isActor:
		if !target.IsVisible() || !target.IsHostileTo(player) || target.IsVendor() {
			return false
		}
	case common.IsBarrel(currentMap, mapPos):
	default:
		return false
	}
	a.stopAutoMove()
	a.playerRangedAttack(player, mapPos)
	return true
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
	a.executeAttack(rangedAttackAction)
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
	o, _ := a.state.GetMap().TryGetObjectAt(mapPos)
	obj, ok := o.(*common.Prop)
	if !ok {
		return
	}
	lines, ok := common.ShrineInfoText(obj)
	if !ok {
		return
	}
	a.ui().OpenTextWindow(util.ToLabelText(lines))
}

// ShowActorInfoAt opens the description window for the actor at mapPos.
func (a *Actions) ShowActorInfoAt(mapPos geometry.Point) {
	actor, _ := a.state.GetMap().TryGetActorAt(mapPos)
	title, sections := common.ActorInfoSections(a.state, actor)
	a.ui().OpenInfoWindow(title, sections)
}

func (a *Actions) updateFoV() {
	state := a.state
	player := state.GetPlayer()
	a.actionBroker.Execute(common.NewUpdateFoVAction(common.AbstractCause("FoV Update"), player))
}

func (a *Actions) IsPlayerAutoMoving() bool {
	return len(a.playerPath) > 0
}
