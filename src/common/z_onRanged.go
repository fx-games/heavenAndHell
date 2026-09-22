package common

import (
	"fmt"
)

type OnRangedHook struct {
	ItemHook
	hookDescription string
}

func (d OnRangedHook) ToString() string {
	return fmt.Sprintf("On Ranged: %s", d.hookDescription)
}

func (d OnRangedHook) Name() string {
	return "OnRangedHook"
}

func NewOnRangedHook(description string, after func(itemContext *ItemContext, action Action, context ActionContext) []Action) *OnRangedHook {
	return &OnRangedHook{
		ItemHook: ItemHook{
			HookedAction: ActionNameRangedAttack,
			AfterHook:    after,
		},
		hookDescription: description,
	}
}

func NewMainOnRangedHook() *OnRangedHook {
	return &OnRangedHook{
		ItemHook: ItemHook{
			HookedAction: ActionNameRangedAttack,
			AfterHook: func(context *ItemContext, action Action, actionContext ActionContext) []Action {
				rangedAction := action.(*RangedAttackAction)
				actor := rangedAction.GetAttacker()
				handleStealResource(actor, rangedAction)
				return nil
			},
		},
	}
}
