package common

type ItemContext struct {
	Wearer *Actor
	Item   *Item
}

func (c ItemContext) GetActor() *Actor {
	return c.Wearer
}

func (c ItemContext) GetItem() *Item {
	return c.Item
}

type ItemHook struct {
	HookedAction ActionName
	AfterHook    func(itemContext *ItemContext, action Action, context ActionContext) []Action
	BeforeHook   func(itemContext *ItemContext, action Action, context ActionContext) []Action
	itemContext  *ItemContext
}

func (i *ItemHook) ActionName() ActionName {
	return i.HookedAction
}

func (i *ItemHook) IsBefore() bool {
	return i.BeforeHook != nil
}

func (i *ItemHook) IsAfter() bool {
	return i.AfterHook != nil
}
func (i *ItemHook) SetItemContext(context *ItemContext) {
	i.itemContext = context
}
func (i *ItemHook) After(action Action, context ActionContext) []Action {
	return i.AfterHook(i.itemContext, action, context)
}

func (i *ItemHook) Before(action Action, context ActionContext) []Action {
	return i.BeforeHook(i.itemContext, action, context)
}
