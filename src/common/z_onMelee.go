package common

import (
	"fmt"

	"rpg/src/stats"
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

func NewOnAfterMeleeHook(description string, after func(itemContext *ItemContext, action Action, context ActionContext) []Action) *OnMeleeHook {
	return &OnMeleeHook{
		ItemHook: ItemHook{
			HookedAction: ActionNameMeleeAttack,
			AfterHook:    after,
		},
		effectDescription: description,
	}
}

func NewOnBeforeMeleeHook(description string, before func(itemContext *ItemContext, action Action, context ActionContext) []Action) *OnMeleeHook {
	return &OnMeleeHook{
		ItemHook: ItemHook{
			HookedAction: ActionNameMeleeAttack,
			BeforeHook:   before,
		},
		effectDescription: description,
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
