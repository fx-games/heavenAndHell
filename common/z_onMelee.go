package common

import (
	"fmt"

	"rpg/stats"
)

type OnMeleeHook struct {
	ItemHook
	matchAttacker     *Actor
	effectDescription string
}

func (d *OnMeleeHook) ToString() string {
	return fmt.Sprintf("On Melee Hit: %s", d.effectDescription)
}

func (d *OnMeleeHook) SetMatchAttacker(actor *Actor) {
	d.matchAttacker = actor
}
func (d *OnMeleeHook) Name() string {
	return "OnMeleeHook"
}

func (d *OnMeleeHook) Before(action Action, context ActionContext) []Action {
	meleeAction := action.(*MeleeAttackAction)
	if d.matchAttacker != nil && d.matchAttacker != meleeAction.GetAttacker() {
		return nil
	}
	return d.ItemHook.Before(action, context)
}
func NewOnAfterMeleeHook(effectDescription string, afterHook func(*ItemContext, Action, ActionContext) []Action) *OnMeleeHook {
	return &OnMeleeHook{
		ItemHook: ItemHook{
			HookedAction: ActionNameMeleeAttack,
			AfterHook:    afterHook,
		},
		effectDescription: effectDescription,
	}
}
func NewOnBeforeMeleeHook(effectDescription string, beforeHook func(*ItemContext, Action, ActionContext) []Action) *OnMeleeHook {
	return &OnMeleeHook{
		ItemHook: ItemHook{
			HookedAction: ActionNameMeleeAttack,
			BeforeHook:   beforeHook,
		},
		effectDescription: effectDescription,
	}
}

func NewMainOnMeleeHook() *OnMeleeHook {
	return &OnMeleeHook{
		ItemHook: ItemHook{
			HookedAction: ActionNameMeleeAttack,
			AfterHook: func(itemContext *ItemContext, action Action, actionContext ActionContext) []Action {
				meleeAction := action.(*MeleeAttackAction)
				actor := meleeAction.GetAttacker()
				handleStealResource(actor, meleeAction)
				return nil
			},
		},
	}
}

type DamagingAction interface {
	GetTotalDamageDealt() int
}

func handleStealResource(actor *Actor, damageAction DamagingAction) {
	lifeStealFactor := actor.GetStats().GetAttributeAsFloat(stats.LifeStealFactor)
	if lifeStealFactor > 0.0 {
		lifeStealAmount := float64(damageAction.GetTotalDamageDealt()) * lifeStealFactor
		actor.GetStats().GetResource(stats.Health).IncreaseByFloat(lifeStealAmount)
	}

	manaStealFactor := actor.GetStats().GetAttributeAsFloat(stats.ManaStealFactor)
	if manaStealFactor > 0.0 {
		manaStealAmount := float64(damageAction.GetTotalDamageDealt()) * manaStealFactor
		actor.GetStats().GetResource(stats.Mana).IncreaseByFloat(manaStealAmount)
	}
}
