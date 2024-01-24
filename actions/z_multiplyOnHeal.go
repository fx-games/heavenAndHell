package actions

import "fmt"

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

func (d *MultiplyOnHealHook) Before(action Action, state WorldState) (Action, WorldState) {
	healAction := action.(*ChangeRessourceAction)
	if healAction.Cause().Action() == d.forSource {
		healAction.SetChangeFactor(healAction.GetChangeFactor() * d.multiplier)
		d.executionCount++
		println(fmt.Sprintf("(%s) Multiplier %f applied", healAction.Name(), d.multiplier))
	}
	return healAction, state
}
