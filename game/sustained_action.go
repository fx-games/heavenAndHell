package game

import (
	"image/color"
	"rpg/actions"
	"rpg/common"
	"rpg/stats"
	"rpg/util"
)

type SustainedAction struct {
	*common.BaseTimedBeing

	isActive   bool
	activate   func(user *common.Actor)
	deactivate func(user *common.Actor)
	tryPayCost func(user *common.Actor) bool

	name string
	user *common.Actor
}

func (a *SustainedAction) String() string {
	return a.name
}

func (a *SustainedAction) Counter() int {
	return -1
}
func (a *SustainedAction) GetTooltipLines() []util.LabelText {
	return []util.LabelText{
		util.LabelText{Text: a.name, TextColor: color.White},
	}
}

func (a *SustainedAction) InventoryIcon() int32 {
	return common.IconEffectFireSmall
}

func (a *SustainedAction) TintColor() color.Color {
	return color.White
}

func (a *SustainedAction) IsAlive() bool {
	return a.isActive
}

func (a *SustainedAction) Turn() {
	if a.isActive {
		if !a.tryPayCost(a.user) {
			a.Deactivate()
		}
	}
}

func (a *SustainedAction) Name() string {
	return a.name
}
func (a *SustainedAction) SetCurrentUser(user *common.Actor) {
	a.user = user
}
func (a *SustainedAction) Deactivate() {
	a.deactivate(a.user)
	a.isActive = false
}
func (a *SustainedAction) TryActivate(actor *common.Actor) bool {
	a.SetCurrentUser(actor)
	if a.tryPayCost(actor) {
		a.isActive = true
		a.activate(actor)
		return true
	}
	return false
}

type SustainedActionName string

func (g *GameInstance) getInnerFlame() *SustainedAction {
	hook := func(context *actions.ItemContext, action actions.Action, engine actions.Engine) []actions.Action {
		attackAction := action.(*actions.MeleeAttackAction)
		attackAction.SimulateAttack(g)

		attacker := attackAction.GetAttacker()
		defender := attackAction.GetTargetedDefender()
		if defender == nil {
			return nil
		}
		forceVector := defender.MapPosition().Sub(attacker.MapPosition())
		displacePos := g.getDisplacePositionForActor(defender, forceVector)

		var chainedActions []actions.Action
		currentMap := g.state.GetMap()

		if displacePos == attacker.MapPosition() {
			// swap positions
			currentMap.SwapPositions(attacker, defender)
			//swapAction := actions.NewSwapPositionsAction(actions.NewActorCause("Inner Flame", attacker), attacker, defender, attacker == g.state.GetPlayer())
			//chainedActions = append(chainedActions, swapAction)
		} else {
			// PROBLEM: activating a new movement action will also trigger the endturn hooks, so we will spend two turns..
			// PROBLEM 2: we don't update the draw positions correctly
			// PROBLEM 3: The animation for this case is all over the place
			oldDefenderPos := defender.MapPosition()
			currentMap.MoveActor(defender, displacePos)
			currentMap.MoveActor(attacker, oldDefenderPos)
			/*
			   moveDefenderAction := actions.NewMoveAction(actions.NewActorCause("Inner Flame", attacker), defender, displacePos, defender == g.state.GetPlayer())
			   chainedActions = append(chainedActions, moveDefenderAction)

			   moveAttackerAction := actions.NewMoveAction(actions.NewActorCause("Inner Flame", attacker), attacker, oldDefenderPos, attacker == g.state.GetPlayer())
			   chainedActions = append(chainedActions, moveAttackerAction)
			*/
		}
		attackAction.SetTargetPos(defender.MapPosition())

		return chainedActions
	}

	s := &SustainedAction{
		name:           "Inner Flame",
		BaseTimedBeing: common.NewBaseTimedBeing(),
		tryPayCost: func(user *common.Actor) bool {
			if user.GetStats().GetRessource(stats.Mana).Value() < 1 {
				return false
			}
			g.actionBroker.Execute(actions.NewChangeRessourceAction(actions.NewActorCause("Inner Flame", user), user, stats.Mana, -1))
			return true
		},
	}
	meleeHook := actions.NewOnBeforeMeleeHook("Inner flame", hook)

	s.activate = func(user *common.Actor) {
		g.timekeeper.AddTimed(s)
		meleeHook.SetMatchAttacker(user)
		g.actionBroker.RegisterHook(meleeHook)
	}

	s.deactivate = func(user *common.Actor) {
		g.actionBroker.RemoveBeforeHook(meleeHook)
	}

	return s
}
