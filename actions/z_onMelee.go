package actions

import (
	"fmt"
	"rpg/common"
	"rpg/stats"
)

type OnMeleeHook struct {
	ItemHook
	matchAttacker     *common.Actor
	effectDescription string
}

func (d *OnMeleeHook) ToString() string {
	return fmt.Sprintf("On Melee Hit: %s", d.effectDescription)
}

func (d *OnMeleeHook) SetMatchAttacker(actor *common.Actor) {
	d.matchAttacker = actor
}
func (d *OnMeleeHook) Name() string {
	return "OnMeleeHook"
}

func (d *OnMeleeHook) Before(action Action, engine Engine) []Action {
	meleeAction := action.(*MeleeAttackAction)
	if d.matchAttacker != nil && d.matchAttacker != meleeAction.GetAttacker() {
		return nil
	}
	return d.ItemHook.Before(action, engine)
}
func NewOnAfterMeleeHook(effectDescription string, afterHook func(*ItemContext, Action, Engine) []Action) *OnMeleeHook {
	return &OnMeleeHook{
		ItemHook: ItemHook{
			BaseActionHook: BaseActionHook{
				hookedAction: ActionNameMeleeAttack,
			},
			afterHook: afterHook,
		},
		effectDescription: effectDescription,
	}
}
func NewOnBeforeMeleeHook(effectDescription string, beforeHook func(*ItemContext, Action, Engine) []Action) *OnMeleeHook {
	return &OnMeleeHook{
		ItemHook: ItemHook{
			BaseActionHook: BaseActionHook{
				hookedAction: ActionNameMeleeAttack,
			},
			beforeHook: beforeHook,
		},
		effectDescription: effectDescription,
	}
}

func NewMainOnMeleeHook() *OnMeleeHook {
	return &OnMeleeHook{
		ItemHook: ItemHook{
			BaseActionHook: BaseActionHook{
				hookedAction: ActionNameMeleeAttack,
			},
			afterHook: func(context *ItemContext, action Action, engine Engine) []Action {
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

func handleStealResource(actor *common.Actor, damageAction DamagingAction) {
	lifeStealFactor := actor.GetStats().GetAttributeAsFloat(stats.LifeStealFactor)
	if lifeStealFactor > 0.0 {
		lifeStealAmount := float64(damageAction.GetTotalDamageDealt()) * lifeStealFactor
		actor.GetStats().GetRessource(stats.Health).IncreaseByFloat(lifeStealAmount)
	}

	manaStealFactor := actor.GetStats().GetAttributeAsFloat(stats.ManaStealFactor)
	if manaStealFactor > 0.0 {
		manaStealAmount := float64(damageAction.GetTotalDamageDealt()) * manaStealFactor
		actor.GetStats().GetRessource(stats.Mana).IncreaseByFloat(manaStealAmount)
	}
}
