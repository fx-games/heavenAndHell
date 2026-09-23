package common

import "rpg/src/util"

type OnRangedHook struct {
	FuncHook
}

func (d OnRangedHook) Name() string {
	return "OnRangedHook"
}

func NewMainOnRangedHook() *OnRangedHook {
	return &OnRangedHook{
		FuncHook: FuncHook{
			HookedAction: ActionNameRangedAttack,
			AfterHook: func(action Action, actionContext ActionContext) []Action {
				rangedAction := action.(*RangedAttackAction)
				actor := rangedAction.GetAttacker()
				handleStealResource(actor, rangedAction)
				if actor.hero == nil || actor.HeroSheet().FireballShot <= 0 {
					return nil
				}
				// fireball_shot: the shot bursts where it lands, hitting what stands around it
				m := actionContext.GameState.GetMap
				tr := NewRangedAttackTargetingRules(util.Arguments{}).GetTargetRules(actor, DijkstraAoE(m, util.Arguments{"3"}), m()).(*RangedTargetingRules)
				target := rangedAction.GetTargetPos()
				return OneAction(NewDamageSpellAction(NewActorCause(ActionNameRangedAttack, actor), actor, actor.HeroSheet().FireballShot, target, tr.GetHitpositions(target)))
			},
		},
	}
}
