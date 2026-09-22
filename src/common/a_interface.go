package common

import (
	"fmt"
	"regexp"
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
	attackerName := attacker.Name() + "'s"
	if !attacker.IsVisible() {
		attackerName = "something's"
	}
	if attacker == state.GetPlayer() {
		attackerName = "your"
	}

	defenderName := defender.Name()
	if !defender.IsVisible() {
		defenderName = "Something"
	}
	if defender == state.GetPlayer() {
		defenderName = "You"
	}

	msg(fmt.Sprintf("%s dodged %s attack", defenderName, attackerName))
}

func logMissed(context ActionContext, attacker, defender *Actor) {
	state := context.GameState
	msg := context.Msg
	attackerName := attacker.Name()
	if !attacker.IsVisible() {
		attackerName = "Something"
	}
	defenderName := defender.Name()
	if !defender.IsVisible() {
		defenderName = "something"
	}
	verb := "misses"

	if attacker == state.GetPlayer() {
		attackerName = "You"
		verb = "miss"
	}
	if defender == state.GetPlayer() {
		defenderName = "you"
	}
	msg(fmt.Sprintf("%s %s %s", attackerName, verb, defenderName))
}
func logAttack(context ActionContext, attacker, defender *Actor, damageAmount int) {
	state := context.GameState
	msg := context.Msg
	attackerName := attacker.Name()
	if !attacker.IsVisible() {
		attackerName = "Something"
	}
	defenderName := defender.Name()
	if !defender.IsVisible() {
		defenderName = "something"
	}
	verb := "hits"

	if attacker == state.GetPlayer() {
		attackerName = "You"
		verb = "hit"
	}
	if defender == state.GetPlayer() {
		defenderName = "you"
	}
	if damageAmount > 0 {
		msg(fmt.Sprintf("%s %s %s (%d)", attackerName, verb, defenderName, damageAmount))
	} else {
		msg(fmt.Sprintf("%s %s %s, but no damage is done", attackerName, verb, defenderName))
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

func RemoveColorCodes(colorCodedText string) string {
	// color codes look like this [:red] [:blue] [:green] [:yellow] [:white] [:24,54,222]
	regexPattern := `\[(:[\w,]+)\]`
	rp, _ := regexp.Compile(regexPattern)

	return rp.ReplaceAllString(colorCodedText, "")
}
