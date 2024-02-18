package common

type ApplyStatusParametrizedAction struct {
	BaseAction
	actor     *Actor
	effect    *ParametrizedStatusEffect
}

func NewApplyStatusParametrizedAction(source Cause, actor *Actor, effect *ParametrizedStatusEffect) *ApplyStatusParametrizedAction {
	return &ApplyStatusParametrizedAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameApplyStatusEffect,
		},
		actor:     actor,
		effect:    effect,
	}
}
func (d *ApplyStatusParametrizedAction) Execute(context ActionContext) []Action {
	d.actor.AddParametrizedStatusEffect(d.effect)
	return nil
}

func (d *ApplyStatusParametrizedAction) GetAnimations() []Animation {
	return nil
}

func (d *ApplyStatusParametrizedAction) GetActor() *Actor {
	return d.actor
}
