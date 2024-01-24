package actions

import "fmt"

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

func NewMainOnRangedHook() *OnRangedHook {
	return &OnRangedHook{
		ItemHook: ItemHook{
			BaseActionHook: BaseActionHook{
				hookedAction: ActionNameRangedAttack,
			},
			afterHook: func(context *ItemContext, action Action, engine Engine) []Action {
				rangedAction := action.(*RangedAttackAction)
				actor := rangedAction.GetAttacker()
				handleStealResource(actor, rangedAction)
				return nil
			},
		},
	}
}

func NewOnRangedHook(hookDescription string, afterHook func(*ItemContext, Action, Engine) []Action) *OnRangedHook {
	return &OnRangedHook{
		ItemHook: ItemHook{
			BaseActionHook: BaseActionHook{
				hookedAction: ActionNameRangedAttack,
			},
			afterHook: afterHook,
		},
		hookDescription: hookDescription,
	}
}
