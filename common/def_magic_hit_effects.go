package common

import (
	"fmt"
	"math/rand"
	"rpg/defs"
	"rpg/geometry"
	"rpg/stats"
	"rpg/util"
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
	TargetedActionNameFireball     TargetedActionName = "fireball"
	ActionNameFirewall             TargetedActionName = "firewall"
	ActionNameBlink                TargetedActionName = "blink"
	ActionNameMindPush             TargetedActionName = "mind_push"
	ActionNameImmobilize           TargetedActionName = "immobilize_target"
	ActionNameDebilitatingShot     TargetedActionName = "debilitating_shot"
	ActionNameFireRay              TargetedActionName = "fire_ray"
	TargetedActionNameStealItem    TargetedActionName = "steal_item"
	ActionNameStealItemAndTeleport TargetedActionName = "steal_item_vanish"
	ActionNameFireBresen           TargetedActionName = "fire_bresen"
)

func (g *Factory) GetRandomTargetedAction(randomSource *rand.Rand, dungeonLevel int) *TargetedAction {
	var allowed []defs.TargetedActionDefinition
	for _, targetedDef := range g.definitions.TargetedDefinitions {
		if targetedDef.DungeonLevel <= dungeonLevel {
			allowed = append(allowed, targetedDef)
		}
	}
	randomDefinition := allowed[randomSource.Intn(len(allowed))]
	tAction := g.NewTargetedActionFromDefinition(randomDefinition)
	tAction.SetDungeonLevel(dungeonLevel)
	return tAction
}

func (g *Factory) confuseOnHitProc(args util.Arguments) *HitEffect {
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			currentMap := g.state.GetMap()
			if !currentMap.IsActorAt(target) {
				return
			}
			hitActor := currentMap.ActorAt(target)
			g.actionBroker.Execute(NewApplyStatusAction(NewActorCause("Dart Throw", attacker), hitActor, StatusConfused, 5))
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Confuse target"),
		},
	}
}

func (g *Factory) throwFireball(args util.Arguments) *HitEffect {
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			action := NewDamageSpellAction(NewActorCause("Fireball", attacker), attacker, 10, target, hitPositions)
			g.actionBroker.Execute(action)
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Throw a fireball"),
		},
	}
}

func (g *Factory) projectFire(args util.Arguments) *HitEffect {
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			for _, hitPos := range hitPositions {
				g.actionBroker.Execute(NewSpawnObjectAction(NewActorCause("Fire", attacker), g.spawnFireAt, hitPos))
			}
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Create fire at target location"),
		},
	}
}

func (g *Factory) spawnFireAt(location geometry.Point) {
	fire := NewFire(3)
	procFireAtLocation := func(location geometry.Point, startAnimating func() bool) {
		if g.state.GetMap().IsActorAt(location) {
			actor := g.state.GetMap().ActorAt(location)
			action := NewChangeResourceAction(AbstractCause("Fire"), actor, stats.Health, -3)
			action.SetAnimationStartPredicate(startAnimating)
			g.actionBroker.Execute(action)
		}
	}
	onTurn := func() {
		if !fire.IsAlive() {
			g.state.RemoveObject(fire)
			return
		}
		fire.DealDamage(nil)
	}
	fire.SetOnTurn(onTurn)
	fire.SetDealDamage(procFireAtLocation)
	g.addToTimeKeeper(fire)

	g.state.SpawnObject(location, fire)
	procFireAtLocation(location, AnimateNow())
}

func (g *Factory) fireZone(args util.Arguments) *HitEffect {
	damageInterval := stats.NewIntervalFromString(args.Get(0))
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			if !attacker.GetInventory().HasLightSource() {
				return
			}
			damagePerTile := damageInterval.LinearRandom()
			damageSpellAction := NewDamageSpellAction(NewActorCause("Flask of Oil", attacker), attacker, damagePerTile, target, hitPositions)
			g.actionBroker.Execute(damageSpellAction)
		},
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Create a small fiery explosion,")),
			util.LabelTextFromString(fmt.Sprintf("dealing %s damage to all targets", damageInterval.ToString())),
		},
	}
}

func (g *Factory) stealItem(args util.Arguments) *HitEffect {
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			currentMap := g.state.GetMap()
			currentMap.ApplyToActorsAt(hitPositions, func(hitActor *Actor) {
				g.actionBroker.Execute(NewStealItemAction(NewActorCause("Steal", attacker), attacker, hitActor))
			})
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Steal item"),
		},
	}
}

func (g *Factory) stealItemAndTeleport(args util.Arguments) *HitEffect {
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			currentMap := g.state.GetMap()
			currentMap.ApplyToActorsAt(hitPositions, func(hitActor *Actor) {
				g.actionBroker.Execute(NewStealItemAction(NewActorCause("Steal", attacker), attacker, hitActor))
			})
			teleportAction := g.NewUntargetedActionFromName("teleport")
			teleportAction.Invoke(attacker)
			attacker.SetNeutral()
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Steal item and teleport"),
		},
	}
}

func (g *Factory) mindPush(args util.Arguments) *HitEffect {
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			currentMap := g.state.GetMap()
			currentMap.ApplyToActorsAt(hitPositions, func(hitActor *Actor) {
				hitPos := hitActor.MapPosition()
				pushDirection := hitPos.Sub(attacker.MapPosition())
				pushDistance := 3
				g.actionBroker.Execute(NewKnockbackAction(NewActorCause("Push", attacker), hitActor, pushDirection, pushDistance))
			})
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Push target away"),
		},
	}
}

/*
	 func (g *Factory) getDebilitatingShot(name common.TargetedActionName) *common.TargetedAction {
		applyStatus := common.NewTargetedAction(name, OneTargetTile, g.rangedAttack, NewDirectTargetingRules)
		applyStatus.SetIcon(223)
		return applyStatus
	}
*/
func (g *Factory) applyStatusEffectToSelection(args util.Arguments) *HitEffect {
	//effect common.StatusEffect, turnCount int
	effectName := args.Get(0)
	turnCount := args.GetInt(1)
	if !IsValidStatusEffect(effectName) {
		g.logError(fmt.Sprintf("Unknown status effect: %s", effectName))
		return nil
	}
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			currentMap := g.state.GetMap()
			currentMap.ApplyToActorsAt(hitPositions, func(actor *Actor) {
				g.actionBroker.Execute(NewApplyStatusAction(NewActorCause("ApplyStatus", attacker), actor, StatusEffect(effectName), turnCount))
			})
		},
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Apply %s to target(s)", effectName)),
		},
	}
}

func (g *Factory) blink(args util.Arguments) *HitEffect {
	return &HitEffect{
		Activate: func(attacker *Actor, target geometry.Point, hitPositions []geometry.Point) {
			isPlayer := g.state.GetPlayer() == attacker
			action := NewMoveAction(NewActorCause("Blink", attacker), attacker, target, isPlayer)
			action.DisableAnimation()
			g.actionBroker.Execute(action)
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Teleport to target location"),
		},
	}
}
