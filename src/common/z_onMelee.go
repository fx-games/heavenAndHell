package common

import "rpg/src/stats"

type OnMeleeHook struct {
	FuncHook
}

func (d *OnMeleeHook) Name() string {
	return "OnMeleeHook"
}

func NewMainOnMeleeHook() *OnMeleeHook {
	return &OnMeleeHook{
		FuncHook: FuncHook{
			HookedAction: ActionNameMeleeAttack,
			AfterHook: func(action Action, actionContext ActionContext) []Action {
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
