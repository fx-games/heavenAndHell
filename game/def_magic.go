package game

import (
    "rpg/actions"
    "rpg/common"
    "rpg/geometry"
)

// Design System for Magic effects
// We have

// UntargetedActions
// - Cannot be targeted
// - Can be delayed
// - Will be executed in the context of the actor that uses it

// TargetedActions
// - Can be targeted
// - Cannot be delayed (currently)
// - Must define a HitZone for targeting
// - Will receive the attacker, the target position, and the hit locations

// StatusEffects
// - Can be applied to an actor
// - Can be temporary or permanent
// - Can be parameterized (e.g. poison damage)
// - Will have a direct impact on game mechanics

func (g *GameInstance) confuseOnHitProc(attacker *common.Actor, hitLocation geometry.Point) {
    currentMap := g.state.GetMap()
    if !currentMap.IsActorAt(hitLocation) {
        return
    }
    hitActor := currentMap.ActorAt(hitLocation)
    g.actionBroker.Execute(actions.NewApplyStatusAction(actions.NewActorCause("Dart Throw", attacker), hitActor, common.StatusConfused, 5))
}

