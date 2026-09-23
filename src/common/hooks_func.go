package common

// FuncHook is an ActionHook made of plain functions: the engine's own hooks on melee, ranged and drop actions.
type FuncHook struct {
	HookedAction ActionName
	AfterHook    func(action Action, context ActionContext) []Action
	BeforeHook   func(action Action, context ActionContext) []Action
}

func (i *FuncHook) ActionName() ActionName { return i.HookedAction }
func (i *FuncHook) IsBefore() bool         { return i.BeforeHook != nil }
func (i *FuncHook) IsAfter() bool          { return i.AfterHook != nil }
func (i *FuncHook) After(action Action, context ActionContext) []Action {
	return i.AfterHook(action, context)
}
func (i *FuncHook) Before(action Action, context ActionContext) []Action {
	return i.BeforeHook(action, context)
}
