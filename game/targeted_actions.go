package game

import (
	"math/rand"
	"rpg/actions"
	"rpg/common"
	"rpg/geometry"
)

const (
	ActionNameFireball common.TargetedActionName = "Fireball"
	ActionNameFirewall common.TargetedActionName = "Firewall"
	ActionNameBlink    common.TargetedActionName = "Blink"
	ActionNameMindPush    common.TargetedActionName = "Mind Push"
	ActionNameImmobilize       common.TargetedActionName = "Immobilize"
	ActionNameDebilitatingShot common.TargetedActionName = "Debilitating Shot"
	ActionNameFireRay		  common.TargetedActionName = "Fire Ray"
	ActionNameFireBresen		  common.TargetedActionName = "Fire Bresen"
)

func (g *GameInstance) GetRandomTargetedAction(randomSource *rand.Rand, qLevel int) *common.TargetedAction {
	actionNames := []common.TargetedActionName{
		ActionNameFireball,
		ActionNameFirewall,
		ActionNameBlink,
		ActionNameMindPush,
		ActionNameImmobilize,
		ActionNameDebilitatingShot,
		ActionNameFireRay,
		ActionNameFireBresen,
	}
	return g.NewTargetedActionFromName(actionNames[randomSource.Intn(len(actionNames))])
}
func (g *GameInstance) NewTargetedActionFromName(name common.TargetedActionName) *common.TargetedAction {
	switch name {
	case ActionNameFireball:
		return g.getFireballAction(name)
	case ActionNameFirewall:
		return g.getFirewallAction(name)
	case ActionNameBlink:
		return g.getBlinkAction(name)
	case ActionNameMindPush:
		return g.getForcePushAction(name)
	case ActionNameImmobilize: return g.getForceApplyStatusEffect(name, common.StatusImmobile, 3)
	case ActionNameDebilitatingShot: return g.getDebilitatingShot(name)
	case ActionNameFireRay: return g.getFireRay(name)
	case ActionNameFireBresen: return g.getFireBresen(name)
	}
	return nil
}

func (g *GameInstance) getFireballAction(name common.TargetedActionName) *common.TargetedAction {
	hitPattern := DijkstraAoE(g.GetWorldState().GetMap(), 3)
	targetedAction := common.NewTargetedAction(name, hitPattern, g.throwFireball, NewRangedAttackTargetingRules)
	targetedAction.SetIcon(common.IconEffectFireSmall)
	return targetedAction
}
func (g *GameInstance) throwFireball(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	g.CancelTargeting()
	g.actionBroker.CancelAllAnimations()

	g.actionBroker.Execute(actions.NewFireballAction(actions.NewActorCause("Fireball", g.state.GetPlayer()), g.state.GetPlayer(), 10, target, hitPositions))
}

func (g *GameInstance) projectFire(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	g.CancelTargeting()
	g.actionBroker.CancelAllAnimations()

	for _, hitPos := range hitPositions {
		g.actionBroker.Execute(actions.NewSpawnObjectAction(actions.NewActorCause("Fire", g.state.GetPlayer()), g.spawnFireAt, hitPos))
	}
}

func (g *GameInstance) forcePush(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	g.CancelTargeting()
	g.actionBroker.CancelAllAnimations()

	currentMap := g.state.GetMap()
	currentMap.ApplyToActorsAt(hitPositions, func(hitActor *common.Actor) {
		hitPos := hitActor.MapPosition()
		pushDirection := hitPos.Sub(attacker.MapPosition())
		pushDistance := 3
		g.actionBroker.Execute(actions.NewKnockbackAction(actions.NewActorCause("Push", g.state.GetPlayer()), hitActor, pushDirection, pushDistance))
	})
}
func (g *GameInstance) getFireRay(name common.TargetedActionName) *common.TargetedAction {
	hitPattern := RayAoE(g.GetWorldState().GetMap(), 7)
	targetedAction := common.NewTargetedAction(name, hitPattern, g.projectFire, NewDirectTargetingRules)
	targetedAction.SetIcon(common.IconEffectFireSmall)
	return targetedAction
}

func (g *GameInstance) getFireBresen(name common.TargetedActionName) *common.TargetedAction {
	hitPattern := BresenhamAoE(g.GetWorldState().GetMap(), 7)
	targetedAction := common.NewTargetedAction(name, hitPattern, g.projectFire, NewDirectTargetingRules)
	targetedAction.SetIcon(common.IconEffectFireSmall)
	return targetedAction
}
func (g *GameInstance) getFirewallAction(name common.TargetedActionName) *common.TargetedAction {
	targetedAction := common.NewTargetedAction(name, WallAoE(g.GetWorldState().GetMap(), 5), g.projectFire, NewDirectTargetingRules)
	targetedAction.SetIcon(common.IconEffectFireSmall)
	return targetedAction
}
func (g *GameInstance) getForcePushAction(name common.TargetedActionName) *common.TargetedAction {
	forcePush := common.NewTargetedAction(name, OneTargetTile, g.forcePush, NewDirectTargetingRules)
	forcePush.SetIcon(223)
	return forcePush
}
func (g *GameInstance) getBlinkAction(name common.TargetedActionName) *common.TargetedAction {
	blinkAction := common.NewTargetedAction(name, OneTargetTile, g.blink, NewDirectTargetingRules)
	blinkAction.SetIcon(223)
	return blinkAction
}
func (g *GameInstance) getForceApplyStatusEffect(name common.TargetedActionName, effect common.StatusEffect, turnCount int) *common.TargetedAction {
	applyStatus := common.NewTargetedAction(name, OneTargetTile, g.applyStatusEffectToSelection(effect, turnCount), NewDirectTargetingRules)
	applyStatus.SetIcon(223)
	return applyStatus
}
func (g *GameInstance) getDebilitatingShot(name common.TargetedActionName) *common.TargetedAction {
	applyStatus := common.NewTargetedAction(name, OneTargetTile, g.playerAimRangedAttack, NewDirectTargetingRules)
	applyStatus.SetIcon(223)
	return applyStatus
}
func (g *GameInstance) applyStatusEffectToSelection(effect common.StatusEffect, turnCount int) common.SelectionHandler {
	return func(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
		currentMap := g.state.GetMap()
		currentMap.ApplyToActorsAt(hitPositions, func(actor *common.Actor) {
			g.actionBroker.Execute(actions.NewApplyStatusAction(actions.NewActorCause("ApplyStatus", g.state.GetPlayer()), actor, effect, turnCount))
		})
	}
}

func (g *GameInstance) blink(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	g.CancelTargeting()
	g.actionBroker.CancelAllAnimations()
	g.cancelAutoMove()

	player := g.state.GetPlayer() == attacker
	action := actions.NewMoveAction(actions.NewActorCause("Blink", g.state.GetPlayer()), attacker, target, player)
	action.DisableAnimation()
	g.actionBroker.Execute(action)
}