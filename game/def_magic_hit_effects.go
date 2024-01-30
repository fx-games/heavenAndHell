package game

import (
	"fmt"
	"io"
	"math/rand"
	"rpg/actions"
	"rpg/common"
	"rpg/geometry"
	"rpg/recfile"
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

type TargetedActionDefinition struct {
	Name string
	InternalName string
	Icon int32
	DungeonLevel int

	HitZone string
	Targeting   string
	OnHitEffect string
}
const (
	ActionNameFireball             common.TargetedActionName = "fireball"
	ActionNameFirewall             common.TargetedActionName = "firewall"
	ActionNameBlink                common.TargetedActionName = "blink"
	ActionNameMindPush             common.TargetedActionName = "mind_push"
	ActionNameImmobilize           common.TargetedActionName = "immobilize_target"
	ActionNameDebilitatingShot     common.TargetedActionName = "debilitating_shot"
	ActionNameFireRay              common.TargetedActionName = "fire_ray"
	ActionNameStealItem            common.TargetedActionName = "steal_item"
	ActionNameStealItemAndTeleport common.TargetedActionName = "steal_item_vanish"
	ActionNameFireBresen           common.TargetedActionName = "fire_bresen"
)
func (g *GameInstance) GetRandomTargetedAction(randomSource *rand.Rand, dungeonLevel int) *common.TargetedAction {
	var allowed []TargetedActionDefinition
	for _, targetedDef := range g.targetedDefinitions {
		if targetedDef.DungeonLevel <= dungeonLevel {
			allowed = append(allowed, targetedDef)
		}
	}
	randomDefinition := allowed[randomSource.Intn(len(allowed))]
	tAction := g.NewTargetedActionFromDefinition(randomDefinition)
	tAction.SetDungeonLevel(dungeonLevel)
	return tAction
}
func (g *GameInstance) NewTargetedActionFromName(name string) *common.TargetedAction {
	if def, exists := g.targetedDefinitions[name]; exists {
		return g.NewTargetedActionFromDefinition(def)
	}
	g.ErrLog(fmt.Sprintf("Unknown targeted action: %s", name))
	return nil
}
func ReadTargetedDefinitions(file io.Reader) map[string]TargetedActionDefinition {
	records := recfile.Read(file)
	targetedDefs := make(map[string]TargetedActionDefinition, len(records))
	for _, record := range records {
		def := NewTargetedFromRecord(record)
		targetedDefs[def.InternalName] = def
	}
	return targetedDefs
}

func NewTargetedFromRecord(record recfile.Record) TargetedActionDefinition {
	def := TargetedActionDefinition{}
	for _, field := range record {
		switch field.Name {
		case "name":
			def.Name = field.Value
		case "internal_name":
			def.InternalName = field.Value
		case "icon":
			def.Icon = field.AsInt32()
		case "dungeon_level":
			def.DungeonLevel = field.AsInt()
		case "hit_zone":
			def.HitZone = field.Value
		case "targeting":
			def.Targeting = field.Value
		case "effect":
			def.OnHitEffect = field.Value
		}
	}
	return def
}

func (g *GameInstance) NewTargetedActionFromDefinition(definition TargetedActionDefinition) *common.TargetedAction {
	targetedAction := common.NewTargetedAction(definition.Name, g.NewHitZoneFromString(definition.HitZone), g.NewOnHitEffectFromString(definition.OnHitEffect), g.NewTargetingRulesFromString(definition.Targeting))
	targetedAction.SetInternalName(definition.InternalName)
	targetedAction.SetDungeonLevel(definition.DungeonLevel)
	targetedAction.SetIcon(definition.Icon)
	return targetedAction
}
func (g *GameInstance) GetAllHitEffects() map[string]func(args util.Arguments)common.HitEffect {
	return map[string]func(args util.Arguments)common.HitEffect{
		"apply": func(args util.Arguments) common.HitEffect {
			effectName := args.Get(0)
			turnCount := args.GetInt(1)
			if !common.IsValidStatusEffect(effectName) {
				g.ErrLog(fmt.Sprintf("Unknown status effect: %s", effectName))
				return nil
			}
			return g.applyStatusEffectToSelection(common.StatusEffect(effectName), turnCount)
		},
		"confuse": func(args util.Arguments) common.HitEffect {
			return g.confuseOnHitProc
		},
		"fireball": func(args util.Arguments) common.HitEffect {
			return g.throwFireball
		},
		"project_fire": func(args util.Arguments) common.HitEffect {
			return g.projectFire
		},
		"fire_zone": func(args util.Arguments) common.HitEffect {
			return g.fireZone
		},
		"steal_item": func(args util.Arguments) common.HitEffect {
			return g.stealItem
		},
		"steal_item_teleport": func(args util.Arguments) common.HitEffect {
			return g.stealItemAndTeleport
		},
		"mind_push": func(args util.Arguments) common.HitEffect {
			return g.mindPush
		},
		"blink": func(args util.Arguments) common.HitEffect {
			return g.blink
		},
	}
}
func (g *GameInstance) NewOnHitEffectFromString(effect string) common.HitEffect {
	if effect == "" {
		return nil
	}
	allHitEffects := g.GetAllHitEffects()
	if util.LooksLikeAFunction(effect) {
		parametrizedEffect, args := util.GetNameAndArgs(effect)
		if resolver, ok := allHitEffects[parametrizedEffect]; ok {
			return resolver(args)
		}
	}
	if _, ok := allHitEffects[effect]; ok {
		return allHitEffects[effect](util.Arguments{})
	}
	if effect != "" {
		g.ErrLog(fmt.Sprintf("Unknown on hit effect: %s", effect))
	}
	return nil
}

func (g *GameInstance) confuseOnHitProc(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	currentMap := g.state.GetMap()
	if !currentMap.IsActorAt(target) {
		return
	}
	hitActor := currentMap.ActorAt(target)
	g.actionBroker.Execute(actions.NewApplyStatusAction(actions.NewActorCause("Dart Throw", attacker), hitActor, common.StatusConfused, 5))
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

func (g *GameInstance) fireZone(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	if !attacker.GetInventory().HasLightSource() {
		return
	}
	damagePerTile := 6
	damageSpellAction := actions.NewDamageSpellAction(actions.NewActorCause("Flask of Oil", attacker), attacker, damagePerTile, target, hitPositions)
	g.actionBroker.Execute(damageSpellAction)
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
	g.getRandomTeleportAction(attacker)
	attacker.SetNeutral()
}

func (g *GameInstance) mindPush(attacker *common.Actor, target geometry.Point, hitPositions []geometry.Point) {
	currentMap := g.state.GetMap()
	currentMap.ApplyToActorsAt(hitPositions, func(hitActor *common.Actor) {
		hitPos := hitActor.MapPosition()
		pushDirection := hitPos.Sub(attacker.MapPosition())
		pushDistance := 3
		g.actionBroker.Execute(actions.NewKnockbackAction(actions.NewActorCause("Push", attacker), hitActor, pushDirection, pushDistance))
	})
}


/*
	func (g *GameInstance) getDebilitatingShot(name common.TargetedActionName) *common.TargetedAction {
		applyStatus := common.NewTargetedAction(name, OneTargetTile, g.rangedAttack, NewDirectTargetingRules)
		applyStatus.SetIcon(223)
		return applyStatus
	}
*/
func (g *GameInstance) applyStatusEffectToSelection(effect common.StatusEffect, turnCount int) common.HitEffect {
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
