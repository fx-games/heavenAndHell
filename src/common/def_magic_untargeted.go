package common

import (
	"cmp"
	"fmt"
	"image/color"
	"math"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/stats"
	"rpg/src/util"
	"slices"
)

const (
	ActionNameHealHealthAndMana UntargetedActionName = "heal_health_mana_full"
	ActionNameHealHealth        UntargetedActionName = "heal_health"
	ActionNameBerserk           UntargetedActionName = "apply_berserk_self"
	ActionNameApplyStatusSelf   UntargetedActionName = "apply_status_self"
	ActionNameHealMana          UntargetedActionName = "heal_mana"
	ActionNameTeleport          UntargetedActionName = "random_near_teleport"
	ActionNameBuffDamage        UntargetedActionName = "buff_damage"
	ActionNameBuffSpeed         UntargetedActionName = "buff_speed"
	ActionNameRecall            UntargetedActionName = "recall"
	ActionNameBless             UntargetedActionName = "buff_armor"
	ActionNameStoneSkin         UntargetedActionName = "stoneskin"
	ActionNameEnchantWeapon     UntargetedActionName = "enchant_weapon"
	ActionNameEnchantArmor      UntargetedActionName = "enchant_armor"
	ActionNameAcquirement       UntargetedActionName = "acquirement"
	ActionNameMultiShot         UntargetedActionName = "multi_shot"
	ActionNameTelefrag          UntargetedActionName = "telefrag"
	ActionNameReadMessage       UntargetedActionName = "read_message"
	ActionNameReplicate         UntargetedActionName = "replicate_with"
	ActionNameEarthquake        UntargetedActionName = "earthquake"
	ActionNameRageSelf          UntargetedActionName = "apply_rage_self"
	ActionNameReflectSelf       UntargetedActionName = "apply_reflect_self"
)

func (g *Factory) NewUntargetedActionFromName(name string) *UntargetedAction {
	var args util.Arguments

	if util.LooksLikeAFunction(name) {
		name, args = util.GetNameAndArgs(name)
	}
	// The mod's effects/untargeted.rec gives the action its name, icon and default arguments; without a record
	// it still resolves straight from the effect registry, nameless.
	def, hasDef := g.Generator.D.UntargetedDefs[name]
	if hasDef && args.IsEmpty() && !def.DefaultArgs.IsEmpty() {
		args = def.DefaultArgs
	}
	effect := g.UntargetedEffectFromName(name, args)
	if effect == nil {
		return nil
	}
	action := NewUntargetedAction(name, 0)
	if hasDef {
		action = NewUntargetedAction(def.Name, def.Icon)
		action.SetDelay(def.ActivationDelay)
		if def.ActivationCondition != "" {
			action.SetCondition(g.NewConditionFromString(def.ActivationCondition))
		}
	}
	action.SetInternalName(name)
	action.SetEffect(effect)
	return action
}

// NewTargetedActionFromName builds a skill the mod's effects/targeted.rec declares; nil for a name it does not.
func (g *Factory) NewTargetedActionFromName(name string) *TargetedAction {
	var args util.Arguments
	if util.LooksLikeAFunction(name) {
		name, args = util.GetNameAndArgs(name)
	}
	def, ok := g.Generator.D.TargetedDefs[name]
	if !ok {
		g.Msg(fmt.Sprintf("no targeted action found for name %s", name))
		return nil
	}
	return g.NewTargetedActionFromDefinition(def, args)
}

func (g *Factory) UntargetedEffectFromName(name string, args util.Arguments) *UntargetedEffect {
	allEffects := g.GetAllUntargetedEffects()
	if effect, exists := allEffects[name]; exists {
		untargetedEffect := effect(args)
		return &untargetedEffect
	}
	g.Msg(fmt.Sprintf("no untargeted effect found for name %s", name))
	return nil
}

func (g *Factory) GetAllUntargetedEffects() map[string]func(args util.Arguments) UntargetedEffect {
	return map[string]func(args util.Arguments) UntargetedEffect{
		string(ActionNameHealHealthAndMana): g.getHealHealthAndMana,
		string(ActionNameHealHealth):        g.getHealHealthBy,
		string(ActionNameBerserk):           g.getBerserkSelfAction,
		string(ActionNameApplyStatusSelf):   g.getApplyStatusAction,
		string(ActionNameHealMana):          g.getHealManaBy,
		"true_sight":                        g.getTrueSight,
		string(ActionNameReadMessage):       g.getReadMessage,
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
		string(ActionNameRageSelf):          g.getRageSelfAction,
		string(ActionNameReflectSelf):       g.getReflectSelfAction,
		"shield_allies":                     g.shieldAllies,
	}
}

func (g *Factory) getReplicate(args util.Arguments) UntargetedEffect {
	effect := func(user *Actor, source EffectSource, forced bool) {
		currentMap := g.State.GetMap()
		origin := user.MapPosition()
		// find position for spawning
		freePositions := currentMap.GetFreeCellsForDistribution(origin, 1, func(p geometry.Point) bool {
			return currentMap.IsCurrentlyPassable(p)
		})

		if len(freePositions) == 0 { // no space on map
			return
		}

		clonePosition := freePositions[0]

		// spawn clone
		clonedActor := user.Clone()
		clonedActor.SetDrawPosition(clonePosition.ToPointF())
		currentMap.AddActor(clonedActor, clonePosition)

		keeper := g.GetTimeKeeper()

		keeper.AddTimed(clonedActor)
		g.Msg(fmt.Sprintf("A new %s is born!", clonedActor.Name()))
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString("Replicate"),
		},
	}
}

func (g *Factory) getSpeedBuff(args util.Arguments) UntargetedEffect {
	amount := util.ParseDice(args.Get(0)).Roll(rng)
	turnCount := util.ParseDice(args.Get(1)).Roll(rng)
	effect := func(user *Actor, source EffectSource, forced bool) {
		modifier := &stats.Modifier{
			Attribute:      stats.BaseSpeed,
			Description:    "of haste",
			Value:          float64(amount),
			Mode:           stats.ModifyAdditive,
			IsPercentBased: false,
			StackID:        string(source),
		}
		timekeeper := g.GetTimeKeeper()

		RemoveTimedEffectsBySource(timekeeper, source)

		timedModifier := NewTimedModifier(string(ActionNameBuffSpeed), user, modifier, turnCount)
		timedModifier.Apply()
		timekeeper.AddTimed(timedModifier)

		g.Msg("You feel the world around you slow down!")
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("+%d speed for %d turns", amount, turnCount)),
		},
	}
}

func RemoveTimedEffectsBySource(timekeeper *TimeKeeper, source EffectSource) {
	if source == "" {
		return
	}
	timeModsWithSameSource := timekeeper.ClearTrackedBeings(func(being TimedBeing) bool {
		if mod, ok := being.(*TimedModifier); ok {
			return mod.modifier.StackID == string(source)
		}
		if untargeted, ok := being.(*UntargetedAction); ok {
			return untargeted.effectSource == source
		}
		return false
	})
	for _, timedMod := range timeModsWithSameSource {
		if mod, ok := timedMod.(*TimedModifier); ok {
			mod.Remove()
		}
	}
}

func (g *Factory) getDamageBuff(args util.Arguments) UntargetedEffect {
	intervalRange := util.NewIntervalFromString(args.Get(0))
	turnCount := args.GetInt(1)
	effect := func(user *Actor, source EffectSource, forced bool) {
		modifier := defs.AddToBaseAttributeRandomly(g.RandomSource, stats.DamageBonus, intervalRange.AsTuple())
		modifier.StackID = string(source)
		timedModifier := NewTimedModifier(string(ActionNameBuffDamage), user, modifier, turnCount)
		timeKeeper := g.GetTimeKeeper()

		RemoveTimedEffectsBySource(timeKeeper, source)
		timedModifier.Apply()
		timeKeeper.AddTimed(timedModifier)
		g.Msg("You feel stronger!")
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("+%s damage for %d turns", intervalRange.ToString(), turnCount)),
		},
	}
}
func (g *Factory) getEnchantWeapon(args util.Arguments) UntargetedEffect {
	maxEnchantLevel := 3
	effect := func(user *Actor, source EffectSource, forced bool) {
		isWeapon := func(element *Item) bool { return element.IsWeapon() }
		selected := func(itemKey rune) {
			g.Show(CloseMenus{})
			inventory := user.GetInventory()
			selectedItem := inventory.GetItemByLetter(itemKey)
			weaponStats := selectedItem.GetWeaponStats()
			if weaponStats.Plus >= maxEnchantLevel {
				g.Msg(fmt.Sprintf("Your [:darkText]%s[:white] cannot be enchanted any further.", selectedItem.Name()))
				return
			}
			weaponStats.Plus += 1
			g.Msg(fmt.Sprintf("Your [:darkText]%s[:white] glows [:blue]blue[:white] for a moment.", selectedItem.Name()))
		}

		if user == g.State.GetPlayer() {
			g.Show(ChooseInventoryItem{isWeapon, selected})
		} else if user.GetEquipment().HasWeaponEquipped() {
			// select wielded weapon
			wieldedWeapon := user.GetEquipment().GetBySlot(defs.SlotNameMainHand)
			weaponStats := wieldedWeapon.GetWeaponStats()
			if weaponStats.Plus >= maxEnchantLevel {
				return
			}
			weaponStats.Plus += 1
		}
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("+1 to a weapon's enchantment, up to +%d", maxEnchantLevel)),
		},
	}
}
func (g *Factory) getEnchantArmor(args util.Arguments) UntargetedEffect {
	maxEnchantLevel := 3
	effect := func(user *Actor, source EffectSource, forced bool) {
		isArmor := func(element *Item) bool { return element.IsArmor() }
		selected := func(itemKey rune) {
			g.Show(CloseMenus{})
			inventory := user.GetInventory()
			selectedItem := inventory.GetItemByLetter(itemKey)
			weaponStats := selectedItem.GetArmorStats()
			if weaponStats.Plus >= maxEnchantLevel {
				g.Msg(fmt.Sprintf("Your [:darkText]%s[:white] cannot be enchanted any further.", selectedItem.Name()))
				return
			}
			weaponStats.Plus += 1
			g.Msg(fmt.Sprintf("Your [:darkText]%s[:white] glows [:blue]blue[:white] for a moment.", selectedItem.Name()))
		}

		if user == g.State.GetPlayer() {
			g.Show(ChooseInventoryItem{isArmor, selected})
		} else if user.GetEquipment().HasArmorEquipped() {
			// select worn armor
			wornArmor := user.GetEquipment().GetBySlot(defs.SlotNameArmorTorso)
			armorStats := wornArmor.GetWeaponStats()
			if armorStats.Plus >= maxEnchantLevel {
				return
			}
			armorStats.Plus += 1
		}
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("+1 to an armor's enchantment, up to +%d", maxEnchantLevel)),
		},
	}
}
func (g *Factory) getAcquirement(args util.Arguments) UntargetedEffect {
	level := 12
	if !args.IsEmpty() {
		level = util.ParseDice(args.Get(0)).Roll(rng)
	}
	effect := func(user *Actor, source EffectSource, forced bool) {
		// ponytail: main's GenerateMagicItem rolled from defs.ItemDefinition, which is content and is
		// gone. d1's FloorItem with onlygood is the equivalent "drop one good item" roll.
		it := g.Generator.FloorItem(level, true, false)
		if it == nil {
			return
		}
		g.State.AddItemToMapWithDistribution(g.NewRolledItem(it), user.MapPosition())
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Drops one good item of item level %d", level)),
		},
	}
}
func (g *Factory) getHealHealthAndMana(args util.Arguments) UntargetedEffect {
	effect := func(user *Actor, source EffectSource, forced bool) {
		healHealthAction := NewChangeResourceAction(NewActorCause("Full Heal", user), user, stats.Health, float64(user.GetStats().GetAttribute(stats.MaxHealth)))
		healManaAction := NewChangeResourceAction(NewActorCause("Full Heal", user), user, stats.Mana, float64(user.GetStats().GetAttribute(stats.MaxMana)))
		g.ActionBroker.Execute(healHealthAction)
		g.ActionBroker.Execute(healManaAction)
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString("Completely heal health and mana"),
		},
	}
}
func (g *Factory) logIfPlayer(user *Actor, message string) {
	if user == g.State.GetPlayer() {
		g.Msg(message)
	}
}
func (g *Factory) getHealHealthBy(args util.Arguments) UntargetedEffect {
	interval := util.NewIntervalFromString(args.Get(0))
	effect := func(user *Actor, source EffectSource, forced bool) {
		healHealthAction := NewChangeResourceAction(NewActorCause("Heal", user), user, stats.Health, float64(interval.Roll(rng)))
		g.ActionBroker.Execute(healHealthAction)
		g.logIfPlayer(user, "You feel better")
		g.tryEndTurn(user, 100/user.GetBaseSpeed())
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Heals %s life", interval.ToString())),
		},
	}
}

func (g *Factory) getHealManaBy(args util.Arguments) UntargetedEffect {
	interval := util.NewIntervalFromString(args.Get(0))
	effect := func(user *Actor, source EffectSource, forced bool) {
		healHealthAction := NewChangeResourceAction(NewActorCause("Mana Surge", user), user, stats.Mana, float64(interval.Roll(rng)))
		g.ActionBroker.Execute(healHealthAction)
		g.Msg("You feel a surge of mana")
		g.tryEndTurn(user, 100/user.GetBaseSpeed())
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Restores %s mana", interval.ToString())),
		},
	}
}
func (g *Factory) getMultiShot(args util.Arguments) UntargetedEffect {
	shotCount := args.GetInt(0)
	effect := func(user *Actor, source EffectSource, forced bool) {
		if !user.GetEquipment().HasRangedWeaponEquipped() {
			g.Msg("No ranged weapon equipped")
			return
		}
		nearbyEnemies := g.getVisibleEnemiesByDistance(user, forced)
		if len(nearbyEnemies) == 0 {
			g.Msg("No enemies in sight")
			return
		}
		hitCount := min(shotCount, len(nearbyEnemies))
		for i := 0; i < hitCount; i++ {
			target := nearbyEnemies[i]
			rangedAttackAction := NewRangedAttackAction(NewActorCause("Multi Shot", user), user, target.MapPosition())
			g.ActionBroker.Execute(rangedAttackAction)
		}
		g.tryEndTurn(user, 100/user.GetAttackSpeed(RangedAttack))
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Shoot at %d enemies at the same time", shotCount)),
		},
	}
}

func (g *Factory) getEarthquake(args util.Arguments) UntargetedEffect {
	earthQuakeRadius := args.GetIntOrDefault(0, 4)
	earthQuakeDamageRange := args.GetIntOrDefault(1, 10)
	effect := func(user *Actor, source EffectSource, forced bool) {
		currentMap := g.State.GetMap()
		generator := g.RandomSource
		allPOIs := g.State.GetAllPOIs()

		defWallIcon := defs.IconDungeonRockyWall
		// ponytail: main's IconDungeonFloorAlternative1 / IconDungeonOfDoom*Modern are content
		// icons that engine-split does not define; fall back to the rocky/red-earth theme.
		defFloorIcon := defs.IconDungeonRedEarthFloor

		floorNumber := g.State.CurrentLevel().Depth

		switch floorNumber {
		case 1, 2:
			defWallIcon = defs.IconCathedralWall
			defFloorIcon = defs.IconCathedralFloor
		case 3, 4:
			defWallIcon = defs.IconCatacombsWall
			defFloorIcon = defs.IconCatacombsFloor
		case 5, 6, 7, 8:
			defWallIcon = defs.IconDungeonRockyWall
			defFloorIcon = defs.IconDungeonRedEarthFloor
		}
		wallTile := gridmap.Tile{
			DefinedIcon:        defWallIcon,
			DefinedDescription: "a wall",
			IsWalkable:         false,
			IsTransparent:      false,
		}

		floorTile := gridmap.Tile{
			DefinedIcon:        defFloorIcon,
			DefinedDescription: "floor tile",
			IsWalkable:         true,
			IsTransparent:      false,
		}

		areaOfEffect := currentMap.GetDijkstraMap(user.MapPosition(), earthQuakeRadius, func(point geometry.Point) bool {
			return !currentMap.IsStairsAt(point) && !currentMap.IsBorderWall(point)
		})

		delete(areaOfEffect, user.MapPosition())

		shuffleAreaOfEffect := func() {
			for pos := range areaOfEffect {
				if generator.Intn(2) == 0 {
					continue
				}
				// flip terrain
				if currentMap.IsWalkable(pos) {
					currentMap.SetTile(pos, wallTile)
				} else {
					currentMap.SetTile(pos, floorTile)
				}
			}
		}

		// this is the real algorithm we are using
		// shuffle stuff around
		shuffleAreaOfEffect()
		// update the reachability map
		g.State.updatePlayerDijkstra()

		// do this until the level is connected
		for !g.State.IsReachableByPlayer(allPOIs) {
			shuffleAreaOfEffect()
			g.State.updatePlayerDijkstra()
		}

		getNearestFreePositionInAoE := func(origin geometry.Point) geometry.Point {
			minDistance := math.MaxInt64
			nearestFree := origin
			for pos := range areaOfEffect {
				if !currentMap.CanPlaceActorHere(pos) {
					continue
				}
				distance := geometry.DistanceManhattan(origin, pos)
				if distance < minDistance {
					minDistance = distance
					nearestFree = pos
				}
			}
			return nearestFree
		}
		isSomethingStuckHere := func(pos geometry.Point) bool {
			if currentMap.IsTileWalkable(pos) {
				return false
			}
			if currentMap.IsObjectAt(pos) || currentMap.IsItemAt(pos) || currentMap.IsActorAt(pos) || currentMap.IsDownedActorAt(pos) {
				return true
			}
			return false
		}
		var affectedActorActions []Action
		for pos := range areaOfEffect {
			if !isSomethingStuckHere(pos) {
				continue
			}
			if currentMap.IsItemAt(pos) { // destroy items on affected tiles
				currentMap.RemoveItemAt(pos)
			}

			if currentMap.IsObjectAt(pos) { // destroy objects on affected tiles
				currentMap.RemoveObjectAt(pos)
			}

			if downedActor, ok := currentMap.TryGetDownedActorAt(pos); ok { // destroy corpses on affected tiles
				currentMap.RemoveDownedActor(downedActor)
			}

			if hitActor, ok := currentMap.TryGetActorAt(pos); ok {
				// move around the actors to the nearest free position and damage them in the process
				newPos := getNearestFreePositionInAoE(hitActor.MapPosition())
				moveAction := NewMoveAction(NewActorCause("Earthquake", user), hitActor, newPos, false)
				affectedActorActions = append(affectedActorActions, moveAction)

				damage := generator.Intn(earthQuakeDamageRange) + 1
				damageAction := NewChangeResourceAction(NewActorCause("Earthquake", user), hitActor, stats.Health, float64(-damage))
				affectedActorActions = append(affectedActorActions, damageAction)

				updateFoV := NewUpdateFoVAction(NewActorCause("Earthquake", hitActor), hitActor)
				affectedActorActions = append(affectedActorActions, updateFoV)
			}
		}
		g.ActionBroker.QueueForExecution(NewUpdateFoVAction(NewActorCause("Earthquake", user), user))
		g.ActionBroker.QueueForExecutionAtTheSameTime(affectedActorActions)

		actionCost := 10 // default cost for any action is 10 aut
		g.tryEndTurn(user, actionCost)
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Reshapes walls within %d steps; creatures caught take 1-%d damage", earthQuakeRadius, earthQuakeDamageRange)),
		},
	}
}

func (g *Factory) tryEndTurn(user *Actor, autUsed int) {
	if user == g.State.GetPlayer() {
		g.ActionBroker.QueueForExecution(NewEndPlayerTurnAction(NewActorCause("Action Used", user), autUsed))
	}
}

func (g *Factory) getBlessing(args util.Arguments) UntargetedEffect {
	turnCount := args.GetInt(0)
	effect := func(user *Actor, source EffectSource, forced bool) {

		// the status is the whole effect: HeroSheet reads it
		g.ActionBroker.Execute(NewApplyStatusAction(NewActorCause("ApplyStatusEffect", user), user, StatusBlessed, turnCount))
		g.logIfPlayer(user, "You feel protected")
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("+5 Armor Class, or doubles it from 5 up, for %d turns", turnCount)),
		},
	}
}
func (g *Factory) getStoneSkinBuff(args util.Arguments) UntargetedEffect {
	turnCount := util.ParseDice(args.Get(0)).Roll(rng)
	effect := func(user *Actor, source EffectSource, forced bool) {
		// the status is the whole effect: HeroSheet and MoveCost read it, so nothing else can outlast it
		g.ActionBroker.Execute(NewApplyStatusAction(NewActorCause("ApplyStatusEffect", user), user, StatusStoneSkin, turnCount))
		g.logIfPlayer(user, "Your skin hardens but movement becomes difficult.")
		g.tryEndTurn(user, 100/user.GetBaseSpeed())
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Armor class raised to %d, steps take twice as long, for %d turns", StoneSkinAC, turnCount)),
		},
	}
}

func (g *Factory) getBerserkSelfAction(args util.Arguments) UntargetedEffect {
	if args.IsEmpty() {
		g.Msg("Berserk effect requires arguments")
	}
	intervalRange := util.NewIntervalFromString(args.Get(0))
	turnCount := args.GetInt(1)
	effect := func(user *Actor, source EffectSource, forced bool) {
		applyStatusAction := NewApplyStatusAction(NewActorCause("ApplyStatusEffect", user), user, StatusBerserk, turnCount)
		g.ActionBroker.Execute(applyStatusAction)

		timeKeeper := g.GetTimeKeeper()

		RemoveTimedEffectsBySource(timeKeeper, source)
		modifier := defs.AddToBaseAttributeRandomly(g.RandomSource, stats.DamageBonus, intervalRange.AsTuple())
		modifier.StackID = string(source)
		timedModifier := NewTimedModifier(string(ActionNameBuffDamage), user, modifier, turnCount)
		timedModifier.Apply()

		timeKeeper.AddTimed(timedModifier)

		g.Msg("Blood for the blood god!")
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Berserk: +%s damage for %d turns", intervalRange.ToString(), turnCount)),
		},
	}
}

func (g *Factory) getApplyStatusAction(args util.Arguments) UntargetedEffect {
	statusEffect := args.Get(0)
	turnCount := args.GetInt(1)
	message := args.Get(2)
	effect := func(user *Actor, source EffectSource, forced bool) {
		applyStatusAction := NewApplyStatusAction(NewActorCause("ApplyStatusEffect", user), user, StatusEffect(statusEffect), turnCount)
		g.ActionBroker.Execute(applyStatusAction)
		if message != "" {
			g.Msg(message)
		}
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Makes you %s for %d turns", statusEffect, turnCount)),
		},
	}
}

func (g *Factory) getRandomTelefragAction(args util.Arguments) UntargetedEffect {
	effect := func(user *Actor, source EffectSource, forced bool) {
		currentMap := g.State.GetMap()
		var visEnemies []*Actor
		for _, visPos := range user.GetFOV().Visibles {
			if actorAt, ok := currentMap.TryGetActorAt(visPos); ok {
				if (actorAt.IsHostileTo(user) || forced) && actorAt.IsAlive() && actorAt != user {
					visEnemies = append(visEnemies, actorAt)
				}
			}
		}
		if len(visEnemies) == 0 {
			g.Msg("No enemies in sight")
			return
		}
		randomEnemy := visEnemies[g.RandomSource.Intn(len(visEnemies))]
		location := randomEnemy.MapPosition()
		isPlayer := user == g.State.GetPlayer()
		moveAction := NewMoveAction(NewActorCause("Telefrag", user), user, location, isPlayer)
		moveAction.DisableAnimation()

		changeHealthAmount := -randomEnemy.GetStats().GetResource(stats.Health).Value()
		killAction := NewChangeResourceAction(NewActorCause("Telefrag", user), randomEnemy, stats.Health, float64(changeHealthAmount))
		g.ActionBroker.Execute(killAction)
		g.ActionBroker.Execute(moveAction)
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString("Magically displace a random visible enemy"),
			util.LabelTextFromString("with your own body, killing it instantly"),
		},
	}
}

func (g *Factory) getRandomTeleportAction(args util.Arguments) UntargetedEffect {
	if args.IsEmpty() {
		g.Msg("Teleport effect requires arguments")
	}
	distanceInterval := util.NewIntervalFromString(args.Get(0))

	effect := func(user *Actor, source EffectSource, forced bool) {
		currentMap := g.State.GetMap()
		origin := user.MapPosition()
		location, found := currentMap.GetRandomLocation(rng, func(location geometry.Point) bool {
			dist := location.ChebyshevDistanceTo(origin)
			return currentMap.IsCurrentlyPassable(location) && dist >= distanceInterval.Min() && dist <= distanceInterval.Max()
		})
		if !found {
			return
		}
		isPlayer := user == g.State.GetPlayer()
		moveAction := NewMoveAction(NewActorCause("Teleport", user), user, location, isPlayer)
		moveAction.DisableAnimation()

		g.ActionBroker.QueueForExecution(moveAction)
	}
	return UntargetedEffect{
		Effect: effect,
		Description: []util.LabelText{
			util.LabelTextFromString(fmt.Sprintf("Teleports you %s steps away at random", distanceInterval.ToString())),
		},
	}
}

func (g *Factory) getPlayerRecallAction(args util.Arguments) UntargetedEffect {
	return UntargetedEffect{
		Effect: func(user *Actor, source EffectSource, forced bool) {
			recallAction := func() {
				var dest gridmap.Transition
				if g.State.IsPlayerInTown() {
					if g.State.DeepestLevel() == "" {
						return // nowhere below yet
					}
					dest = gridmap.Transition{TargetMap: g.State.DeepestLevel(), TargetLocation: "ladder_up"}
				} else {
					dest = gridmap.Transition{TargetMap: g.State.DeepestTown(), TargetLocation: "spawn"}
				}

				g.State.TransitionTo(dest)
			}
			g.QueueAfterTurn(recallAction)
		},
		Description: []util.LabelText{
			util.LabelTextFromString("Returns you to town, or from town to your deepest level"),
		},
	}
}

/*
Name() string

	GetTooltipLines() []util.LabelText
	InventoryIcon() int32
	TintColor() color.Color
*/
type TimedModifier struct {
	user      *Actor
	modifier  *stats.Modifier
	turnsLeft int
	energy    int
	icon      int32
	name      string
}

func (t *TimedModifier) TintColor() color.RGBA {
	return defs.White
}
func (t *TimedModifier) Name() string {
	return t.name
}
func (t *TimedModifier) String() string {
	return t.name
}
func (t *TimedModifier) InventoryIcon() int32 {
	return t.icon
}
func (t *TimedModifier) GetTooltipLines(textParser func(string) []util.ColoredTextPart) []util.LabelText {
	return []util.LabelText{
		{
			Text:               t.modifier.ToString(),
			TextColor:          defs.DarkTextColor,
			UseColorCodes:      false,
			TextWithColorCodes: textParser(t.Name()),
		},
	}
}
func (t *TimedModifier) AddEnergy(energy int) {
	t.energy += energy
}

func (t *TimedModifier) RemoveEnergy(used int) {
	t.energy -= used
}

func (t *TimedModifier) IsAlive() bool {
	return t.turnsLeft > 0
}

func (t *TimedModifier) GetBaseSpeed() int {
	return 10
}

func (t *TimedModifier) GetEnergy() int {
	return t.energy
}

func (t *TimedModifier) Turn() {
	if t.turnsLeft == 0 {
		return
	}
	t.turnsLeft--
	if t.turnsLeft == 0 {
		t.Remove()
	}
}

func (t *TimedModifier) Apply() {
	t.user.GetStats().AddModifier(t.modifier)
}

func (t *TimedModifier) Remove() {
	t.user.GetStats().RemoveModifier(t.modifier)
}
func (t *TimedModifier) Counter() int {
	return t.turnsLeft
}
func NewTimedModifier(name string, user *Actor, modifier *stats.Modifier, turns int) *TimedModifier {
	return &TimedModifier{
		name:      name,
		user:      user,
		modifier:  modifier,
		turnsLeft: turns,
	}
}

func (g *Factory) getVisibleEnemiesByDistance(user *Actor, forced bool) []*Actor {
	currentMap := g.State.GetMap()
	var nearbyEnemies []*Actor
	for _, visPos := range user.GetFOV().Visibles {
		actor, ok := currentMap.TryGetActorAt(visPos)
		if !ok {
			continue
		}
		if (actor.IsHostileTo(user) || forced) && actor.IsAlive() && user != actor {
			nearbyEnemies = append(nearbyEnemies, actor)
		}
	}
	slices.SortStableFunc(nearbyEnemies, func(i, j *Actor) int {
		distI := currentMap.MoveDistance(user.MapPosition(), i.MapPosition())
		distJ := currentMap.MoveDistance(user.MapPosition(), j.MapPosition())
		return cmp.Compare(distI, distJ)
	})
	return nearbyEnemies
}

func (g *Factory) NewConditionFromString(condition string) func(user *Actor) bool {
	if condition == "can_shield" {
		return func(user *Actor) bool { return len(g.unshieldedAllies(user)) > 0 }
	}
	if condition == "can_replicate" {
		return g.canReplicate
	}
	if condition == "can_enchant_armor" {
		return g.hasEnchentableArmor
	}
	if condition == "can_enchant_weapon" {
		return g.hasEnchentableWeapon
	}
	g.Msg(fmt.Sprintf("no condition found for name %s", condition))
	return func(user *Actor) bool { return true }
}

func (g *Factory) hasEnchentableArmor(user *Actor) bool {
	if !user.GetInventory().HasEnchantableArmor() {
		g.Msg("You have no armor to enchant")
		return false
	}
	return true
}

func (g *Factory) hasEnchentableWeapon(user *Actor) bool {
	if !user.GetInventory().HasEnchantableWeapon() {
		g.Msg("You have no weapon to enchant")
		return false
	}
	return true
}

func (g *Factory) canReplicate(user *Actor) bool {
	currentMap := g.State.GetMap()

	copyCount := 0
	// check for map limit
	for _, actor := range currentMap.Actors() {
		if actor.GetInternalName() == user.GetInternalName() {
			copyCount++
		}
	}

	if copyCount >= 60 { // currently hardcoded
		return false
	}

	origin := user.MapPosition()
	cloneNeighbors := currentMap.GetFilteredNeighbors(origin, func(p geometry.Point) bool {
		actorAt, ok := currentMap.TryGetActorAt(p)
		return ok && actorAt.GetInternalName() == user.GetInternalName()
	})

	if len(cloneNeighbors) != 1 {
		// conditions for replication not met
		return false
	}

	// find position for spawning
	freePositions := currentMap.GetFreeCellsForDistribution(origin, 1, func(p geometry.Point) bool {
		return currentMap.IsCurrentlyPassable(p)
	})

	if len(freePositions) == 0 { // no space on map
		return false
	}

	return true
}

// getTrueSight is Lucifer's gift: the drinker sees and lights seven cells further, for good.
func (g *Factory) getTrueSight(util.Arguments) UntargetedEffect {
	return UntargetedEffect{
		Effect: func(user *Actor, _ EffectSource, _ bool) {
			user.GetStats().IncrementBy(stats.VisionRange, 7)
			user.GetStats().IncrementBy(stats.LightRadius, 7)
			g.State.GetFlags().SetTrue(string(FlagHasTrueSight))
			g.ActionBroker.Execute(NewUpdateFoVAction(AbstractCause("TrueSightActivated"), user))
			g.Msg("Your eyes open.")
		},
		Description: []util.LabelText{util.LabelTextFromString("Can this be real? Can it be trusted?"),
			util.LabelTextFromString("Only one way to find out...")},
	}
}

// getReadMessage reads one of the mod's texts (texts.rec) off a note, any number of times, taking no time.
func (g *Factory) getReadMessage(args util.Arguments) UntargetedEffect {
	id := args.Get(0)
	return UntargetedEffect{
		Effect:      func(user *Actor, _ EffectSource, _ bool) { g.say("", id) },
		Description: []util.LabelText{util.LabelTextFromString("Read it.")},
	}
}
