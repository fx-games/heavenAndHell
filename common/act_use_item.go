package common

type UseItemAction struct {
	BaseAction
	actor      *Actor
	item       *Item
	actionFunc func(action ActionContext) []Action
}

func NewUseItemAction(source Cause, actor *Actor, item *Item, actionFunc func(action ActionContext) []Action) *UseItemAction {
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
func (d *UseItemAction) Execute(context ActionContext) []Action {
	return d.actionFunc(context)
}

func (d *UseItemAction) GetAnimations() []Animation {
	return nil
}
