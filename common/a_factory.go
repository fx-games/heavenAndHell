package common

import (
	"fmt"
	"math/rand"
	"rpg/d1"
	"rpg/defs"
	"rpg/stats"
	"rpg/util"
)

// Will turn definitions into actual game objects
type Factory struct {
	definitions  defs.DataContainer
	actionBroker ActionBroker
	randomSource *rand.Rand
	state        *State
	gui          func() UserInterface

	giveAndEquipItemToActor func(actor *Actor, item *Item)
	getTimeKeeper           func() *TimeKeeper
	logError                func(msg string)
	queueAfterTurn          func(func())
	d1                      *d1.Generator
}

type FactoryContext struct {
	Definitions  defs.DataContainer
	ActionBroker ActionBroker
	RandomSource *rand.Rand
	State        *State

	GetTimeKeeper func() *TimeKeeper

	GiveAndEquipItemToActor func(actor *Actor, item *Item)
	Msg                     func(msg string)
	D1                      *d1.Generator
}

func NewFactory(context FactoryContext) *Factory {
	return &Factory{
		definitions:             context.Definitions,
		randomSource:            context.RandomSource,
		state:                   context.State,
		giveAndEquipItemToActor: context.GiveAndEquipItemToActor,
		actionBroker:            context.ActionBroker,
		getTimeKeeper:           context.GetTimeKeeper,
		logError:                context.Msg,
		d1:                      context.D1,
	}
}

func (g *Factory) D1() *d1.Generator { return g.d1 }
func (g *Factory) NewRandomEnemy(dungeonLevel int) (*Actor, []defs.EnemyDefinition) {
	// get defs
	enemyDefinitions := g.definitions.EnemyDefinitions

	// filter for dungeon level
	var filteredEnemies []defs.EnemyDefinition
	for _, enemyDef := range enemyDefinitions {
		if !enemyDef.CanAppearAt(g.randomSource, dungeonLevel) {
			continue
		}
		filteredEnemies = append(filteredEnemies, enemyDef)
	}

	if len(filteredEnemies) == 0 {
		g.logError(fmt.Sprintf("no enemies found for dungeon level %d", dungeonLevel))
		return nil, nil
	}
	// pick random enemy definition
	enemyDef := filteredEnemies[g.randomSource.Intn(len(filteredEnemies))]

	// wants to spawn as a group?
	var groupDefs []defs.EnemyDefinition
	if len(enemyDef.GroupSpawn) > 0 {
		for _, otherDef := range enemyDef.GroupSpawn {
			otherDefName := otherDef
			spawnCount := 1
			if util.LooksLikeAFunction(otherDef) {
				oDefName, args := util.GetNameAndArgs(otherDef)
				otherDefName = oDefName
				spawnCount = stats.NewIntervalFromString(args.Get(0)).LinearRandom()
			}
			for i := 0; i < spawnCount; i++ {
				if otherEnemyDef, exists := enemyDefinitions[otherDefName]; exists {
					groupDefs = append(groupDefs, otherEnemyDef)
				}
			}
		}
	}
	// create enemy
	return g.NewEnemyFromDefinition(enemyDef, dungeonLevel), groupDefs
}


func (g *Factory) NewUniqueEnemyFromName(name string) *Actor {
	// new way
	uniqueDefs := g.definitions.UniqueEnemyDefinitions

	if enemyDef, ok := uniqueDefs[name]; ok {
		return g.NewEnemyFromDefinition(enemyDef, 1)
	}

	return nil
}

func (g *Factory) NewEnemyFromDefinition(enemyDef defs.EnemyDefinition, dungeonLevel int) *Actor {
	actor := NewActor(enemyDef.Icon)
	actor.SetAsciiIcon(enemyDef.AsciiIcon)
	actor.SetUnicodeIcon(enemyDef.UnicodeIcon)
	actor.SetCP437Icon(enemyDef.CP437Icon)
	actor.SetColor(enemyDef.Color)
	//index := len(emptyMap.Actors())
	actor.SetName(enemyDef.Name)
	actor.SetInternalName(enemyDef.InternalName)
	actor.SetDeathStringFormats(enemyDef.DeathStringFormats)
	actor.SetHostile()

	if enemyDef.Tags.Has(defs.TagUnique) {
		actor.SetUnique()
	}
	if enemyDef.Tags.Has(defs.TagNoCorpse) {
		actor.SetNoCorpse()
	}
	if enemyDef.Tags.Has(defs.TagFlying) {
		actor.AddStatusEffect(StatusFlying, -1)
	}
	for trigger, lines := range enemyDef.Speech {
		actor.SetSpeechForTrigger(trigger, lines)
	}
	actor.SetDamageDice(enemyDef.DamageDice)
	actor.SetBaseSpeed(enemyDef.BaseSpeed)

	actorStats := actor.GetStats()

	actorStats.SetAttribute(stats.MaxHealth, enemyDef.ResourceHealth.Roll())
	actorStats.GetResource(stats.Health).SetToMax()

	for statName, statValue := range enemyDef.Stats {
		actorStats.SetAttribute(stats.StatFromString(statName), statValue)
	}



	var targetedSkills []*TargetedAction
	for _, skillName := range enemyDef.TargetedSkills {
		skillAction := g.NewTargetedActionFromName(skillName)
		if skillAction != nil {
			targetedSkills = append(targetedSkills, skillAction)
		}
	}
	actor.SetTargetedSkills(targetedSkills)

	var untargetedSkills []*UntargetedAction
	for _, skillName := range enemyDef.UntargetedSkills {
		skillAction := g.NewUntargetedActionFromName(skillName)
		if skillAction != nil {
			untargetedSkills = append(untargetedSkills, skillAction)
		}
	}
	actor.SetUntargetedSkills(untargetedSkills)
	actor.SetAggroRange(enemyDef.AggroRange)

	return actor
}







func GetFlickerPatternFromName(pattern string) []float64 {
	switch pattern {
	case "fire":
		return defs.GetFireFlickerPattern()
	case "smooth":
		return defs.GetSmoothFlickerPattern()
	}
	return nil
}


func (g *Factory) NewTargetedActionFromName(name string) *TargetedAction {
	var args util.Arguments

	if util.LooksLikeAFunction(name) {
		name, args = util.GetNameAndArgs(name)
	}

	if def, exists := g.definitions.TargetedDefinitions[name]; exists {
		return g.NewTargetedActionFromDefinition(def, args)
	}
	g.logError(fmt.Sprintf("Unknown targeted action: %s", name))
	return nil
}

func (g *Factory) NewTargetedActionFromDefinition(definition defs.TargetedActionDefinition, args util.Arguments) *TargetedAction {
	targetedAction := NewTargetedAction(
		definition.Name,
		HitZoneFromString(g.state.GetMap, definition.HitZone),
		g.NewOnHitEffectFromNameAndArgs(definition.OnHitEffect, args),
		NewTargetingRulesFromString(definition.Targeting),
	)

	targetedAction.SetInternalName(definition.InternalName)
	targetedAction.SetDungeonLevel(definition.DungeonLevel)
	targetedAction.SetIcon(definition.Icon)
	return targetedAction
}

func (g *Factory) GetAllHitEffects() map[string]func(args util.Arguments) *HitEffect {
	return map[string]func(args util.Arguments) *HitEffect{
		"confuse":             g.confuseTarget,
		"berserk":             g.berserkTarget,
		"poison":              g.poisonTarget,
		"immobilize":          g.immobilizeTarget,
		"fireball":            g.throwFireball,
		"project_fire":        g.projectFire,
		"summon_monster":      g.summonRandomMonster,
		"jumpstart":           g.jumpStart,
		"fire_zone":           g.fireZone,
		"steal_item":          g.stealItem,
		"steal_item_teleport": g.stealItemAndTeleport,
		"mind_push":           g.mindPush,
		"blink":               g.blink,
		"earthshaker":         g.earthShaker,
	}
}

func (g *Factory) NewOnHitEffectFromString(effectName string) *HitEffect {
	if effectName == "" {
		return nil
	}
	var args util.Arguments
	if util.LooksLikeAFunction(effectName) {
		effectName, args = util.GetNameAndArgs(effectName)
	}
	return g.NewOnHitEffectFromNameAndArgs(effectName, args)
}

func (g *Factory) NewOnHitEffectFromNameAndArgs(effectName string, args util.Arguments) *HitEffect {
	allHitEffects := g.GetAllHitEffects()
	if _, ok := allHitEffects[effectName]; ok {
		return allHitEffects[effectName](args)
	}
	if effectName != "" {
		g.logError(fmt.Sprintf("Unknown on hit effect: %s", effectName))
	}
	return nil
}




func (g *Factory) NewInfiniteTargetedWand(targetedAction *TargetedAction) *Item {
	return g.NewTargetedWand(targetedAction, -1, false)
}

func (g *Factory) NewFiniteTargetedWand(targetedAction *TargetedAction) *Item {
	return g.NewTargetedWand(targetedAction, 3, false)
}
func (g *Factory) NewEtherealTargetedWand(targetedAction *TargetedAction) *Item {
	return g.NewTargetedWand(targetedAction, 5, true)
}
func (g *Factory) NewInfiniteUntargetedWand(untargetedAction *UntargetedAction) *Item {
	return g.NewUntargetedWand(untargetedAction, -1, false)
}

func (g *Factory) NewFiniteUntargetedWand(untargetedAction *UntargetedAction) *Item {
	return g.NewUntargetedWand(untargetedAction, 3, false)
}

func (g *Factory) NewEtherealUntargetedWand(untargetedAction *UntargetedAction) *Item {
	return g.NewUntargetedWand(untargetedAction, 5, true)
}

func (g *Factory) NewTargetedWand(targetedAction *TargetedAction, charges int, destroyAfterUse bool) *Item {
	itemName := fmt.Sprintf("a wand of %s", targetedAction.Name())
	baseItem := NewItem(g.state.GetPlayer, itemName, defs.IconItemWand)
	baseItem.SetCategory(defs.ItemCategoryWand)
	baseItem.SetQualityLevel(1)
	baseItem.SetCharges(charges)
	baseItem.SetDestroyAfterUse(destroyAfterUse)
	baseItem.SetTargetedAction(targetedAction)

	return baseItem
}
func (g *Factory) NewUntargetedWand(untargetedAction *UntargetedAction, charges int, destroyAfterUse bool) *Item {
	itemName := fmt.Sprintf("a wand of %s", untargetedAction.Name())
	baseItem := NewItem(g.state.GetPlayer, itemName, defs.IconItemWand)
	baseItem.SetCategory(defs.ItemCategoryWand)
	baseItem.SetQualityLevel(1)
	baseItem.SetCharges(charges)
	baseItem.SetDestroyAfterUse(destroyAfterUse)
	baseItem.SetUntargetedAction(untargetedAction)

	return baseItem
}
func (g *Factory) GetRandomUntargetedAction(dungeonLevel int, actionUsage defs.ActionUsage) *UntargetedAction {
	possibleActions := make([]*UntargetedAction, 0)
	for _, def := range g.definitions.UntargetedDefinitions {
		if def.DungeonLevel <= dungeonLevel && def.AllowedUsage.Allows(actionUsage) {
			possibleActions = append(possibleActions, g.NewUntargetedActionFromDefinition(def, def.DefaultArgs))
		}
	}

	if len(possibleActions) > 0 {
		return possibleActions[rand.Intn(len(possibleActions))]
	}
	g.logError(fmt.Sprintf("no untargeted action found for dungeon level %d", dungeonLevel))

	return nil
}

func (g *Factory) NewUntargetedActionFromDefinition(def defs.UntargetedActionDefinition, args util.Arguments) *UntargetedAction {
	action := NewUntargetedAction(def.Name, def.Icon)
	action.SetInternalName(def.InternalName)
	action.SetDelay(def.ActivationDelay)
	effectFromName := g.UntargetedEffectFromName(def.InternalName, args)
	if effectFromName == nil {
		g.logError(fmt.Sprintf("no untargeted effect found for name %s", def.InternalName))
		return nil
	}
	action.SetEffect(effectFromName)
	if def.ActivationCondition != "" {
		action.SetCondition(g.NewConditionFromString(def.ActivationCondition))
	}

	return action
}
func (g *Factory) UntargetedEffectFromName(name string, args util.Arguments) *UntargetedEffect {
	allEffects := g.GetAllUntargetedEffects()
	if effect, exists := allEffects[name]; exists {
		untargetedEffect := effect(args)
		return &untargetedEffect
	}
	g.logError(fmt.Sprintf("no untargeted effect found for name %s", name))
	return nil
}

func (g *Factory) GetAllUntargetedEffects() map[string]func(args util.Arguments) UntargetedEffect {
	return map[string]func(args util.Arguments) UntargetedEffect{
		string(ActionNameHealHealthAndMana): g.getHealHealthAndMana,
		string(ActionNameHealHealth):        g.getHealHealthBy,
		string(ActionNameBerserk):           g.getBerserkSelfAction,
		string(ActionNameApplyStatusSelf):   g.getApplyStatusAction,
		string(ActionNameHealMana):          g.getHealManaBy,
		string(ActionNameTeleport):          g.getRandomTeleportAction,
		string(ActionNameBuffSpeed):         g.getSpeedBuff,
		string(ActionNameBuffDamage):        g.getDamageBuff,
		string(ActionNameRecall):            g.getPlayerRecallAction,
		string(ActionNameBless):             g.getBlessing,
		string(ActionNameStoneSkin):         g.getStoneSkinBuff,
		string(ActionNameEnchantWeapon):     g.getEnchantWeapon,
		string(ActionNameEnchantArmor):      g.getEnchantArmor,
		string(ActionNameAcquirement):       g.getAcquirement,
		string(ActionNameMultiShot):         g.getMultiShot,
		string(ActionNameTelefrag):          g.getRandomTelefragAction,
		string(ActionNameReplicate):         g.getReplicate,
		string(ActionNameEarthquake):        g.getEarthquake,
	}
}


func (g *Factory) NewPotion(untargetedAction *UntargetedAction) *Item {
	name := fmt.Sprintf("a potion of %s", untargetedAction.Name())
	baseItem := NewItem(g.state.GetPlayer, name, defs.IconItemPotionRed)
	baseItem.SetCategory(defs.ItemCategoryPotion)
	baseItem.SetQualityLevel(1)
	baseItem.SetCharges(1)
	baseItem.SetDestroyAfterUse(true)
	baseItem.SetUntargetedAction(untargetedAction)
	return baseItem
}


func (g *Factory) NewHiddenNote(itemName string, messagText []util.LabelText, onRead func()) *Item {
	action := NewUntargetedAction("read", defs.IconItemScroll)
	//action.SetDescription(fmt.Sprintf("TODO: Description for %s", def.Name))

	effect := g.getReadMessage(messagText, onRead)
	action.SetEffect(&effect)
	action.SetNoTimeNeeded()

	scroll := g.NewScroll(action)
	scroll.SetName(itemName)
	scroll.SetUnlimitedCharges()
	scroll.ShowNameOnlyInTooltip()
	scroll.Hide()
	return scroll
}
func (g *Factory) NewScroll(untargetedAction *UntargetedAction) *Item {
	itemName := fmt.Sprintf("a scroll of %s", untargetedAction.Name())
	baseItem := NewItem(g.state.GetPlayer, itemName, defs.IconItemScroll)
	baseItem.SetCategory(defs.ItemCategoryScroll)
	baseItem.SetQualityLevel(1)
	baseItem.SetCharges(1)
	baseItem.SetDestroyAfterUse(true)
	baseItem.SetUntargetedAction(untargetedAction)
	return baseItem
}

func (g *Factory) NewSign(inscription []string) *BumpObject {
	return NewBumpObject("a sign", defs.IconSign, func(actor *Actor, startAnimating func() bool) {
		g.gui().OpenTextWindow(util.ToLabelText(inscription))
	})
}

func (g *Factory) SetQueueAfterTurn(afterTurn func(call func())) {
	g.queueAfterTurn = afterTurn
}

func (g *Factory) SetUserInterface(ui func() UserInterface) {
	g.gui = ui
}

func (g *Factory) NewPoisonedEffect(turnsLeft int, damagePerRound int) *ParametrizedStatusEffect {
	return NewParametrizedStatusEffect(StatusDamageOverTime, func(a *Actor) {
		g.actionBroker.Execute(NewChangeResourceAction(AbstractCause("poison"), a, stats.Health, float64(-damagePerRound)))
	}, turnsLeft)
}
