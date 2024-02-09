package common

import (
	"fmt"
)

type BaseAction struct {
	cause Cause
	name  ActionName
}

func (d *BaseAction) Cause() Cause {
	return d.cause
}

func (d *BaseAction) Name() ActionName {
	return d.name
}
func (d *BaseAction) GetAnimations() []Animation {
	return nil
}

func (d *BaseAction) GetDelayInAut() int {
	return 10
}

type BaseActionHook struct {
	hookedAction ActionName
}

func (d *BaseActionHook) ActionName() ActionName {
	return d.hookedAction
}

func (d *BaseActionHook) Before(action Action, context ActionContext) []Action {
	return OneAction(action)
}

func (d *BaseActionHook) After(action Action, context ActionContext) []Action {
	return nil
}

func logDodged(context ActionContext, attacker, defender *Actor) {
	state := context.GameState
	msg := context.Msg
	if attacker == state.GetPlayer() {
		msg(fmt.Sprintf("Your attack misses %s", defender.Name()))
	} else if defender == state.GetPlayer() {
		msg(fmt.Sprintf("%s misses you", attacker.Name()))
	} else {
		msg(fmt.Sprintf("%s misses %s", attacker.Name(), defender.Name()))
	}
}
func logAttack(context ActionContext, attacker, defender *Actor, damageAmount int) {
	state := context.GameState
	msg := context.Msg
	if attacker == state.GetPlayer() {
		if damageAmount > 0 {
			msg(fmt.Sprintf("Your attack hits %s", defender.Name()))
		} else {
			msg(fmt.Sprintf("Your attack hits %s but does no damage", defender.Name()))
		}
	} else if defender == state.GetPlayer() {
		if damageAmount > 0 {
			msg(fmt.Sprintf("%s hits you", attacker.Name()))
		} else {
			msg(fmt.Sprintf("%s hits you but does no damage", attacker.Name()))
		}
	} else {
		if damageAmount > 0 {
			msg(fmt.Sprintf("%s hits %s", attacker.Name(), defender.Name()))
		} else {
			msg(fmt.Sprintf("%s hits %s but does no damage", attacker.Name(), defender.Name()))
		}
	}
}

func OneAnimation(animation Animation) []Animation {
	return []Animation{animation}
}

type BaseAnimation struct {
	startThisTick func() bool
	hasStarted    bool
}

func (b *BaseAnimation) shouldRunNow() bool {
	if !b.hasStarted && b.startThisTick != nil && !b.startThisTick() {
		return false
	}
	b.hasStarted = true
	return true
}
func (b *BaseAnimation) StartWhenTrue(startThisTick func() bool) {
	b.startThisTick = startThisTick
}
