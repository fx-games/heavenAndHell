package actions

import (
    "rpg/common"
)

type ApplyStatusAction struct {
    BaseAction
    actor     *common.Actor
    effect    common.StatusEffect
    turnCount int
}

func NewApplyStatusAction(source Cause, actor *common.Actor, effect common.StatusEffect, turnCount int) *ApplyStatusAction {
    return &ApplyStatusAction{
        BaseAction: BaseAction{
            cause: source,
            name:  ActionNameApplyStatusEffect,
        },
        actor: actor,
        effect: effect,
        turnCount: turnCount,
    }
}
func (d *ApplyStatusAction) Execute(engine Engine) []Action {
    d.actor.AddStatusEffect(d.effect, d.turnCount)
    return nil
}

func (d *ApplyStatusAction) GetAnimations() []common.Animation {
    return nil
}

func (d *ApplyStatusAction) GetActor() *common.Actor {
    return d.actor
}

