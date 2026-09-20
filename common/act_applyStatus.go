package common

type ApplyStatusAction struct {
	BaseAction
	actor     *Actor
	effect    StatusEffect
	turnCount int
}

func NewApplyStatusAction(source Cause, actor *Actor, effect StatusEffect, turnCount int) *ApplyStatusAction {
	return &ApplyStatusAction{
		BaseAction: BaseAction{
			cause: source,
			name:  ActionNameApplyStatusEffect,
		},
		actor:     actor,
		effect:    effect,
		turnCount: turnCount,
	}
}
func (d *ApplyStatusAction) Execute(context ActionContext) []Action {
	d.actor.AddStatusEffect(d.effect, d.turnCount)
	return nil
}

func (d *ApplyStatusAction) GetAnimations() []Animation {
	return nil
}

func (d *ApplyStatusAction) GetActor() *Actor {
	return d.actor
}
