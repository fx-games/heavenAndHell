package common

import (
	"rpg/src/geometry"
)

type MoveAction struct {
	BaseAction
	actor             *Actor
	targetPos         geometry.Point
	originPos         geometry.Point
	isPlayer          bool
	animationDelay    float64
	animationDisabled bool
	Step              bool // a walk of the hero's own: D1 plays a footstep (PM_DoWalk's PS_WALK1)
}

func NewMoveAction(source Cause, actor *Actor, targetPos geometry.Point, isPlayer bool) *MoveAction {
	return &MoveAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameMove,
		},
		actor:          actor,
		targetPos:      targetPos,
		isPlayer:       isPlayer,
		animationDelay: 0.2,
	}
}

func (d *MoveAction) SetAnimationDelay(delay float64) {
	d.animationDelay = delay
}

func (d *MoveAction) GetDelayInAut() int {
	return d.actor.MoveCost()
}

// PROBLEM: Since all enemy actions are triggered in one tick,
// but the re-positioning of the actor takes place over multiple ticks,
// all the following actors will have a false idea of the world state.
// SOLUTION: The actor should be moved immediately, but the animation should be delayed.

func (d *MoveAction) Execute(context ActionContext) []Action {
	state := context.GameState
	gameMap := state.GetMap() // read state
	actor := d.actor
	d.originPos = actor.MapPosition()
	gameMap.MoveActor(actor, d.targetPos) // write state
	if d.Step {
		playSfxAt(context, "PS_WALK1", d.targetPos)
	}

	context.ActionBroker.Emit(Moved{Actor: actor, From: d.originPos, To: d.targetPos, Delay: d.animationDelay,
		ByPlayer: d.isPlayer, Quiet: d.animationDisabled})
	return nil
}

func (d *MoveAction) GetActor() *Actor {
	return d.actor
}

func (d *MoveAction) GetOldPos() geometry.Point {
	return d.originPos
}

func (d *MoveAction) GetNewPos() geometry.Point {
	return d.targetPos
}

func (d *MoveAction) DisableAnimation() {
	d.animationDisabled = true
}

func PlayerMoveToAttack(actor *Actor, relativeDirection geometry.Point, forceAttack bool, context ActionContext) Action {
	state := context.GameState
	gameMap := state.GetMap()
	targetPos := actor.MapPosition().Add(relativeDirection)
	if objectAtPos, ok := gameMap.TryGetObjectAt(targetPos); ok {
		if state.IsClosedDoorAt(targetPos) {
			return NewOpenDoorAction(NewActorCause("Open door", actor), actor, targetPos)
		} else {
			objectAtPos.ActorBumpedInto(actor)
			if o, ok := objectAtPos.(*Prop); ok && o.UsesTurn(actor) {
				state.factory.ActionBroker.Execute(NewEndPlayerTurnAction(NewActorCause("Operate", actor), 10))
				return nil
			}
		}
	}

	actorAtPos, isActorAt := gameMap.TryGetActorAt(targetPos)
	isTargetFree := !isActorAt && gameMap.IsWalkable(targetPos)
	if isActorAt {
		if actorAtPos.IsVendor() && !forceAttack {
			return NewTalkToAction(NewActorCause("Talk to", actor), actorAtPos)
		} else if mon := actorAtPos.monster; mon != nil && mon.TalkMsg != "" && actor.hero != nil && !forceAttack {
			state.factory.TalkToMonster(actorAtPos) // TalktoMonster: a monster with something to say is talked to, not hit
			return nil
		} else if actorAtPos.monster != nil && actorAtPos.monster.Golem && !forceAttack {
			return NewSwapPositionsAction(NewActorCause("Swap", actor), actor, actorAtPos, true)
		} else if actor.hero != nil && actor.GetEquipment().GetWeapon(RangedAttack) != nil { // D1 bows shoot point-blank
			return NewRangedAttackAction(NewActorCause("Ranged attack", actor), actor, targetPos)
		} else {
			return NewMeleeAttackAction(NewActorCause("Melee attack", actor), actor, actorAtPos)
		}
	} else if isTargetFree {
		return NewMoveAction(NewActorCause("Move", actor), actor, targetPos, state.GetPlayer() == actor)
	}

	state.GetMap().SetExplored(targetPos)
	return nil
}
