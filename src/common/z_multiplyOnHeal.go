package common

type MultiplyOnHealHook struct {
	BaseActionHook
	forSource  ActionName
	multiplier float64

	executionCount int
}

func NewMultiplyOnHealHook(forSource ActionName, factor float64) *MultiplyOnHealHook {
	return &MultiplyOnHealHook{
		BaseActionHook: BaseActionHook{
			hookedAction: "ActionNameChangeStat",
		},
		multiplier: factor,
		forSource:  forSource,
	}
}

func (d *MultiplyOnHealHook) Before(action Action, context ActionContext) (Action, ActionContext) {
	healAction := action.(*ChangeResourceAction)
	if healAction.Cause().Action() == d.forSource {
		healAction.SetChangeFactor(healAction.GetChangeFactor() * d.multiplier)
		d.executionCount++
	}
	return healAction, context
}
