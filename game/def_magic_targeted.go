package game

import (
	"math/rand"
	"rpg/actions"
	"rpg/common"
	"rpg/geometry"
)

/*
Spell Effects of Moria
 - Sleep Monster
 - Detect Treasure
 - Detect Invisible
 - Detect Monsters
 - Light Area
 - Map Area
 - Aggravate Monster
 - Magic Bolt
 - Fireball
 - Recharge item
 - Drain life
 - Speed Monster
 - Confuse Monster
 - Stone to mud
 - Polymorph other
 - Build Wall
 - Clone Monster
 - Teleport Self / Other
 - (Mass) Genocide
 - Cure Status Effect
 - Earthquake
 - Destruction
 - Enchant Item
 - Remove curse
 - Warding glyph
 - Slow poison
 - Bless

From Brogue
 - Discord (Turn monsters against each other)

*/


const (
	ActionNameFireball common.TargetedActionName = "Fireball"
	ActionNameFirewall common.TargetedActionName = "Firewall"
	ActionNameBlink    common.TargetedActionName = "Blink"
	ActionNameMindPush    common.TargetedActionName = "Mind Push"
	ActionNameImmobilize       common.TargetedActionName = "Immobilize"
	ActionNameDebilitatingShot common.TargetedActionName = "Debilitating Shot"
	ActionNameFireRay		  common.TargetedActionName = "Fire Ray"
	ActionNameStealItem		  common.TargetedActionName = "Steal Item"
	ActionNameStealItemAndTeleport		  common.TargetedActionName = "Steal Item & Vanish"
	ActionNameFireBresen		  common.TargetedActionName = "Fire Bresen"
)

func (g *GameInstance) GetRandomTargetedAction(randomSource *rand.Rand, qLevel int) *common.TargetedAction {
	actionNames := []common.TargetedActionName{
		ActionNameFireball,
		ActionNameFirewall,
		ActionNameBlink,
		ActionNameMindPush,
		ActionNameImmobilize,
		//ActionNameDebilitatingShot,
		ActionNameFireRay,
		ActionNameFireBresen,
		ActionNameStealItem,
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
	//case ActionNameDebilitatingShot: return g.getDebilitatingShot(name)
	case ActionNameFireRay: return g.getFireRay(name)
	case ActionNameFireBresen: return g.getFireBresen(name)
	case ActionNameStealItem: return g.getStealItemAction(name)
	case ActionNameStealItemAndTeleport: return g.getStealItemAndTeleportAction(name)
	}
	return nil
}

func (g *GameInstance) getFireballAction(name common.TargetedActionName) *common.TargetedAction {
	hitPattern := DijkstraAoE(g.GetWorldState().GetMap(), 3)
	targetedAction := common.NewTargetedAction(name, hitPattern, g.throwFireball, NewRangedAttackTargetingRules)
	targetedAction.SetMonetaryValue(3500)
	targetedAction.SetIcon(common.IconEffectFireSmall)
	return targetedAction
}
func (g *GameInstance) throwFireball(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	action := actions.NewDamageSpellAction(actions.NewActorCause("Fireball", attacker), attacker, 10, target, hitPositions)
	g.actionBroker.Execute(action)
}

func (g *GameInstance) projectFire(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	for _, hitPos := range hitPositions {
		g.actionBroker.Execute(actions.NewSpawnObjectAction(actions.NewActorCause("Fire", attacker), g.spawnFireAt, hitPos))
	}
}

func (g *GameInstance) stealItem(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	currentMap := g.state.GetMap()
	currentMap.ApplyToActorsAt(hitPositions, func(hitActor *common.Actor) {
		g.actionBroker.Execute(actions.NewStealItemAction(actions.NewActorCause("Steal", attacker), attacker, hitActor))
	})
}

func (g *GameInstance) stealItemAndTeleport(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	currentMap := g.state.GetMap()
	currentMap.ApplyToActorsAt(hitPositions, func(hitActor *common.Actor) {
		g.actionBroker.Execute(actions.NewStealItemAction(actions.NewActorCause("Steal", attacker), attacker, hitActor))
	})
	teleportAway := g.getRandomTeleportAction()
	teleportAway.Invoke(attacker)
	attacker.SetNeutral()
}

func (g *GameInstance) forcePush(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	currentMap := g.state.GetMap()
	currentMap.ApplyToActorsAt(hitPositions, func(hitActor *common.Actor) {
		hitPos := hitActor.MapPosition()
		pushDirection := hitPos.Sub(attacker.MapPosition())
		pushDistance := 3
		g.actionBroker.Execute(actions.NewKnockbackAction(actions.NewActorCause("Push", attacker), hitActor, pushDirection, pushDistance))
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
	targetedAction.SetMonetaryValue(2500)
	targetedAction.SetIcon(common.IconEffectFireSmall)
	return targetedAction
}
func (g *GameInstance) getForcePushAction(name common.TargetedActionName) *common.TargetedAction {
	forcePush := common.NewTargetedAction(name, OneTargetTile, g.forcePush, NewDirectTargetingRules)
	forcePush.SetMonetaryValue(1500)
	forcePush.SetIcon(223)
	return forcePush
}

func (g *GameInstance) getStealItemAction(name common.TargetedActionName) *common.TargetedAction {
	blinkAction := common.NewTargetedAction(name, OneTargetTile, g.stealItem, NewMeleeTargetingRules)
	blinkAction.SetMonetaryValue(3000)
	blinkAction.SetIcon(223)
	return blinkAction
}

func (g *GameInstance) getStealItemAndTeleportAction(name common.TargetedActionName) *common.TargetedAction {
	targetedAction := common.NewTargetedAction(name, OneTargetTile, g.stealItemAndTeleport, NewMeleeTargetingRules)
	targetedAction.SetMonetaryValue(2800)
	targetedAction.SetIcon(223)
	return targetedAction
}
func (g *GameInstance) getBlinkAction(name common.TargetedActionName) *common.TargetedAction {
	blinkAction := common.NewTargetedAction(name, OneTargetTile, g.blink, NewDirectTargetingRules)
	blinkAction.SetMonetaryValue(2800)
	blinkAction.SetIcon(223)
	return blinkAction
}
func (g *GameInstance) getForceApplyStatusEffect(name common.TargetedActionName, effect common.StatusEffect, turnCount int) *common.TargetedAction {
	applyStatus := common.NewTargetedAction(name, OneTargetTile, g.applyStatusEffectToSelection(effect, turnCount), NewDirectTargetingRules)
	applyStatus.SetIcon(223)
	return applyStatus
}
/*
func (g *GameInstance) getDebilitatingShot(name common.TargetedActionName) *common.TargetedAction {
	applyStatus := common.NewTargetedAction(name, OneTargetTile, g.rangedAttack, NewDirectTargetingRules)
	applyStatus.SetIcon(223)
	return applyStatus
}

 */
func (g *GameInstance) applyStatusEffectToSelection(effect common.StatusEffect, turnCount int) common.SelectionHandler {
	return func(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
		currentMap := g.state.GetMap()
		currentMap.ApplyToActorsAt(hitPositions, func(actor *common.Actor) {
			g.actionBroker.Execute(actions.NewApplyStatusAction(actions.NewActorCause("ApplyStatus", attacker), actor, effect, turnCount))
		})
	}
}

func (g *GameInstance) blink(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	isPlayer := g.state.GetPlayer() == attacker
	action := actions.NewMoveAction(actions.NewActorCause("Blink", attacker), attacker, target, isPlayer)
	action.DisableAnimation()
	g.actionBroker.Execute(action)
}