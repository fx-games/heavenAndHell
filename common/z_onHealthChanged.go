package common

import (
	"rpg/stats"
)

type OnHealthChangedHook struct {
	BaseActionHook
}

func (d OnHealthChangedHook) IsBefore() bool {
	return false
}

func (d OnHealthChangedHook) IsAfter() bool {
	return true
}

func NewOnHealthChangedHook() *OnHealthChangedHook {
	return &OnHealthChangedHook{
		BaseActionHook: BaseActionHook{
			hookedAction: ActionNameChangeResource,
		},
	}
}

func (d OnHealthChangedHook) After(action Action, context ActionContext) []Action {
	changeResource := action.(*ChangeResourceAction)
	actorFromAction := changeResource.GetActor()

	if changeResource.GetResourceName() == stats.Health && changeResource.GetChangeAmount() < 0 {

		actorFromAction.SetHostile()
		actorFromAction.SetState(ActorStateAttacking)

		state := context.GameState
		if actorFromAction.IsVendor() {
			for _, otherActor := range state.GetMap().Actors() {
				if otherActor.IsVendor() {
					otherActor.SetHostile()
					otherActor.SetState(ActorStateAttacking)
					otherActor.SetVendorType(VendorTypeNone)
				}
			}
		}
	}

	if changeResource.GetResourceName() == stats.Health && !actorFromAction.IsAlive() {
		return OneAction(NewDieAction(action.Cause(), actorFromAction))
	}

	return nil
}
