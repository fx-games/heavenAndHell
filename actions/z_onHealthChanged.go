package actions

import (
	"rpg/common"
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
			hookedAction: ActionNameChangeRessource,
		},
	}
}

func (d OnHealthChangedHook) After(action Action, engine Engine) []Action {
	changeRessource := action.(*ChangeRessourceAction)
	actorFromAction := changeRessource.GetActor()

	if changeRessource.GetRessourceName() == stats.Health && changeRessource.GetChangeAmount() < 0 {
		actor := actorFromAction
		state := engine.GetWorldState()
		if actor.IsVendor() {
			for _, otherActor := range state.GetMap().Actors() {
				if otherActor.IsVendor() {
					otherActor.SetHostile()
					otherActor.SetState(common.ActorStateAttacking)
					otherActor.SetVendorType(common.VendorTypeNone)
				}
			}
		}
	}

	if changeRessource.GetRessourceName() == stats.Health && !actorFromAction.IsAlive() {
		return OneAction(NewDieAction(action.Cause(), actorFromAction))
	}

	return nil
}
