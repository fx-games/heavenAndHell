package actions

import "rpg/common"

type ItemContext struct {
	wearer *common.Actor
	item   common.Item
}

func (c ItemContext) GetActor() *common.Actor {
	return c.wearer
}

func (c ItemContext) GetItem() common.Item {
	return c.item
}

type ItemHook struct {
	BaseActionHook
	afterHook   func(itemContext *ItemContext, action Action, engine Engine) []Action
	beforeHook  func(itemContext *ItemContext, action Action, engine Engine) []Action
	itemContext *ItemContext
}

func (i *ItemHook) IsBefore() bool {
	return i.beforeHook != nil
}

func (i *ItemHook) IsAfter() bool {
	return i.afterHook != nil
}
func (i *ItemHook) SetItemContext(context *ItemContext) {
	i.itemContext = context
}
func (i *ItemHook) After(action Action, engine Engine) []Action {
	return i.afterHook(i.itemContext, action, engine)
}

func (i *ItemHook) Before(action Action, engine Engine) []Action {
	return i.beforeHook(i.itemContext, action, engine)
}
