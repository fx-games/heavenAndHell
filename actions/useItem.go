package actions

import (
	"rpg/common"
)

type UseItemAction struct {
	BaseAction
	actor      *common.Actor
	item       common.Item
	actionFunc func(Engine) []Action
}

func NewUseItemAction(source Cause, actor *common.Actor, item common.Item, actionFunc func(Engine) []Action) *UseItemAction {
	return &UseItemAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameUseItem,
		},
		actor:      actor,
		item:       item,
		actionFunc: actionFunc,
	}
}
func (d *UseItemAction) Execute(engine Engine) []Action {
	return d.actionFunc(engine)
}

func (d *UseItemAction) GetAnimations() []common.Animation {
	return nil
}
