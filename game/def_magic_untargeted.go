package game

import (
	"cmp"
	"fmt"
	"image/color"
	"io"
	"math/rand"
	"rpg/actions"
	"rpg/common"
	"rpg/geometry"
	"rpg/recfile"
	"rpg/stats"
	"rpg/util"
	"slices"
)

type UntargetedActionDefinition struct {
	Name                string
	InternalName        string
	Icon                int32
	DungeonLevel        int
	ActivationDelay     int
	ActivationCondition string
}

func ReadUntargetedDefinitions(file io.Reader, logErr func(string)) map[string]UntargetedActionDefinition {
	records := recfile.Read(file)
	untargetedDefs := make(map[string]UntargetedActionDefinition, len(records))
	for _, record := range records {
		def := NewUntargetedFromRecord(record)
		if def.InternalName == "" {
			logErr(fmt.Sprintf("Untargeted action definition has no internal name: %v", def))
			continue
		}
		if _, exists := untargetedDefs[def.InternalName]; exists {
			logErr(fmt.Sprintf("Duplicate untargeted action definition: %s", def.InternalName))
			continue
		}
		untargetedDefs[def.InternalName] = def
	}
	return untargetedDefs
}

func NewUntargetedFromRecord(record recfile.Record) UntargetedActionDefinition {
	definition := UntargetedActionDefinition{}
	for _, field := range record {
		switch field.Name {
		case "name":
			definition.Name = field.Value
		case "internal_name":
			definition.InternalName = field.Value
		case "icon":
			definition.Icon = field.AsInt32()
		case "dlvl":
			definition.DungeonLevel = field.AsInt()
		case "activation_delay":
			definition.ActivationDelay = field.AsInt()
		case "activation_condition":
			definition.ActivationCondition = field.Value
		}
	}
	return definition

}


const (
	ActionNameHealHealthAndMana common.UntargetedActionName = "heal_health_mana_full"
	ActionNameHealHealth        common.UntargetedActionName = "heal_health"
	ActionNameBerserk           common.UntargetedActionName = "apply_berserk_self"
	ActionNameConfuse           common.UntargetedActionName = "apply_confuse_self"
	ActionNameHealMana          common.UntargetedActionName = "heal_mana"
	ActionNameTeleport          common.UntargetedActionName = "random_near_teleport"
	ActionNameBuffDamage        common.UntargetedActionName = "buff_damage"
	ActionNameBuffSpeed         common.UntargetedActionName = "buff_speed"
	ActionNameRecall            common.UntargetedActionName = "recall"
	ActionNameBless             common.UntargetedActionName = "buff_armor"
	ActionNameEnchantWeapon     common.UntargetedActionName = "enchant_weapon"
	ActionNameEnchantArmor      common.UntargetedActionName = "enchant_armor"
	ActionNameMultiShot         common.UntargetedActionName = "multi_shot"
	ActionNameTelefrag          common.UntargetedActionName = "telefrag"
	ActionNameReadMessage common.UntargetedActionName = "read_message"
	ActionNameReplicate   common.UntargetedActionName = "replicate_with"
)

func (g *GameInstance) NewUntargetedActionFromName(name string) *common.UntargetedAction {
	if actionDef, exists := g.untargetedDefinitions[name]; exists {
		return g.NewUntargetedActionFromDefinition(actionDef)
	}

	g.ErrLog(fmt.Sprintf("no untargeted action found for name %s", name))
	return nil
}
func (g *GameInstance) GetRandomUntargetedAction(dungeonLevel int) *common.UntargetedAction {
	possibleActions := make([]*common.UntargetedAction, 0)
	for _, def := range g.untargetedDefinitions {
		if def.DungeonLevel <= dungeonLevel {
			possibleActions = append(possibleActions, g.NewUntargetedActionFromDefinition(def))
		}
	}

	if len(possibleActions) > 0 {
		return possibleActions[rand.Intn(len(possibleActions))]
	}
	g.ErrLog(fmt.Sprintf("no untargeted action found for dungeon level %d", dungeonLevel))

	return nil
}

func (g *GameInstance) NewUntargetedActionFromDefinition(def UntargetedActionDefinition) *common.UntargetedAction {
	action := common.NewUntargetedAction(def.Name, def.Icon)
	action.SetDelay(def.ActivationDelay)
	action.SetDescription(fmt.Sprintf("TODO: Description for %s", def.Name))
	effectFromName := g.UntargetedEffectFromName(def.InternalName)
	if effectFromName == nil {
		g.ErrLog(fmt.Sprintf("no untargeted effect found for name %s", def.InternalName))
		return nil
	}
	action.SetEffect(effectFromName)

	if def.ActivationCondition != "" {
		action.SetCondition(g.NewConditionFromString(def.ActivationCondition))
	}

	return action
}

func (g *GameInstance) GetAllUntargetedEffects() map[string]func(user *common.Actor) {
	return map[string]func(user *common.Actor){
		string(ActionNameHealHealthAndMana): g.getHealHealthAndMana,
		string(ActionNameHealHealth):        g.getHealHealth,
		string(ActionNameBerserk):           g.getBerserkSelfAction(util.Tuple[int,int]{Item1: 3, Item2: 6}, 3),
		string(ActionNameConfuse):           g.getApplyStatusAction(common.StatusConfused, "You feel confused", 6),
		string(ActionNameHealMana):          g.getHealManaBy(10),
		string(ActionNameTeleport):      g.getRandomTeleportAction,
		string(ActionNameBuffSpeed):     g.getSpeedBuff(2, 8),
		string(ActionNameBuffDamage):    g.getDamageBuff(util.Tuple[int,int]{Item1: 3, Item2: 6}, 8),
		string(ActionNameRecall):        g.getPlayerRecallAction,
		string(ActionNameBless):         g.getBlessing(util.Tuple[int,int]{Item1: 6, Item2: 8}),
		string(ActionNameEnchantWeapon): g.getEnchantWeapon,
		string(ActionNameEnchantArmor):  g.getEnchantArmor,
		string(ActionNameMultiShot):     g.getMultiShot(5),
		string(ActionNameTelefrag):      g.getRandomTelefragAction,
		string(ActionNameReadMessage):   g.getReadMessage([]util.LabelText{{Text: "TODO: Read message", TextColor: color.White}}, nil),
		string(ActionNameReplicate):     g.getReplicate,
	}
}

func (g *GameInstance) UntargetedEffectFromName(name string) func(user *common.Actor) {
	allEffects := g.GetAllUntargetedEffects()
	if effect, exists := allEffects[name]; exists {
		return effect
	}
	g.ErrLog(fmt.Sprintf("no untargeted effect found for name %s", name))
	return nil
}


func (g *GameInstance) getReplicate(user *common.Actor) {
	currentMap := g.state.GetMap()
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
	g.timekeeper.AddTimed(clonedActor)
	g.Log(fmt.Sprintf("A new %s is born!", clonedActor.Name()))
}

func (g *GameInstance) getReadMessage(textForMessage []util.LabelText, onRead func()) func(user *common.Actor) {
	return func(user *common.Actor) {
		g.uiController.OpenTextWindow(textForMessage)
		if onRead != nil {
			onRead()
		}
	}
}
func (g *GameInstance) getSpeedBuff(amount int, turnCount int) func(user *common.Actor) {
	return func(user *common.Actor) {
		modifier := &stats.Modifier{
			Attribute:      stats.BaseSpeed,
			Name:           "of haste",
			Value:          float64(amount),
			Mode:           stats.ModifyAdditive,
			IsPercentBased: false,
		}
		timedModifier := NewTimedModifier(string(ActionNameBuffSpeed), user, modifier, turnCount)
		timedModifier.Apply()
		g.timekeeper.AddTimed(timedModifier)
		g.Log("You feel the world around you slow down!")
	}
}

func (g *GameInstance) getDamageBuff(intervalRange util.Tuple[int, int], turnCount int)  func(user *common.Actor) {
	return func(user *common.Actor) {
		modifier := AddToBaseAttribute(g.randomSource, stats.BaseDamage, intervalRange)
		timedModifier := NewTimedModifier(string(ActionNameBuffDamage), user, modifier, turnCount)
		timedModifier.Apply()
		g.timekeeper.AddTimed(timedModifier)
		g.Log("You feel stronger!")
	}
}
func (g *GameInstance) getEnchantWeapon(user *common.Actor) {
	isWeapon := func(element common.Item) bool { return element.IsWeapon() }
	selected := func(itemKey rune) {
		inventory := user.GetInventory()
		selectedItem := inventory.GetItemByLetter(itemKey)
		weaponStats := selectedItem.GetWeaponStats()
		weaponStats.Plus += 1
		g.Log(fmt.Sprintf("Your [:darkText]%s[:white] glows [:blue]blue[:white] for a moment.", selectedItem.Name()))
	}

	if user == g.state.GetPlayer() {
		g.openListInventoryForSelection(isWeapon, selected)
	} else if user.GetEquipment().HasWeaponEquipped() {
		// select wielded weapon
		wieldedWeapon := user.GetEquipment().GetBySlot(common.SlotNameMainHand)
		weaponStats := wieldedWeapon.GetWeaponStats()
		weaponStats.Plus += 1
	}
}
func (g *GameInstance) getEnchantArmor(user *common.Actor) {
	isArmor := func(element common.Item) bool { return element.IsArmor() }
	selected := func(itemKey rune) {
		inventory := user.GetInventory()
		selectedItem := inventory.GetItemByLetter(itemKey)
		weaponStats := selectedItem.GetArmorStats()
		weaponStats.Plus += 1
		g.Log(fmt.Sprintf("Your [:darkText]%s[:white] glows [:blue]blue[:white] for a moment.", selectedItem.Name()))
	}

	if user == g.state.GetPlayer() {
		g.openListInventoryForSelection(isArmor, selected)
	} else if user.GetEquipment().HasArmorEquipped() {
		// select worn armor
		wornArmor := user.GetEquipment().GetBySlot(common.SlotNameArmorTorso)
		armorStats := wornArmor.GetWeaponStats()
		armorStats.Plus += 1
	}
}
func (g *GameInstance) getHealHealthAndMana(user *common.Actor) {
	healHealthAction := actions.NewChangeRessourceAction(actions.NewActorCause("Full Heal", user), user, stats.Health, user.GetStats().GetAttribute(stats.MaxHealth))
	healManaAction := actions.NewChangeRessourceAction(actions.NewActorCause("Full Heal", user), user, stats.Mana, user.GetStats().GetAttribute(stats.MaxMana))
	g.actionBroker.Execute(healHealthAction)
	g.actionBroker.Execute(healManaAction)
}
func (g *GameInstance) getHealHealth(user *common.Actor) {
	healHealthAction := actions.NewChangeRessourceAction(actions.NewActorCause("Full Heal", user), user, stats.Health, user.GetStats().GetAttribute(stats.MaxHealth))
	g.actionBroker.Execute(healHealthAction)
	g.updateStatsLabel()
}
func (g *GameInstance) getHealHealthBy(amount int) func(user *common.Actor) {
	return func(user *common.Actor) {
		healHealthAction := actions.NewChangeRessourceAction(actions.NewActorCause("Heal", user), user, stats.Health, amount)
		g.actionBroker.Execute(healHealthAction)
		g.Log("You feel better")
	}
}

func (g *GameInstance) getHealManaBy(amount int) func(user *common.Actor) {
	return func(user *common.Actor) {
		healHealthAction := actions.NewChangeRessourceAction(actions.NewActorCause("Mana Surge", user), user, stats.Mana, amount)
		g.actionBroker.Execute(healHealthAction)
		g.Log("You feel a surge of mana")
	}
}
func (g *GameInstance) getMultiShot(shotCount int) func(user *common.Actor) {
	return func(user *common.Actor) {
		if !user.GetEquipment().HasRangedWeaponEquipped() {
			g.Log("No ranged weapon equipped")
			return
		}
		nearbyEnemies := g.getVisibleEnemiesByDistance(user)
		if len(nearbyEnemies) == 0 {
			g.Log("No enemies in sight")
			return
		}
		hitCount := min(shotCount, len(nearbyEnemies))
		for i := 0; i < hitCount; i++ {
			target := nearbyEnemies[i]
			rangedAttackAction := actions.NewRangedAttackAction(actions.NewActorCause("Ranged attack", user), user, target.MapPosition())
			g.actionBroker.Execute(rangedAttackAction)
		}
	}
}
func (g *GameInstance) getBlessing(interval util.Tuple[int,int]) func(user *common.Actor) {
	return func(user *common.Actor) {
		modifier := AddToBaseAttribute(g.randomSource, stats.BaseArmorClass, interval)
		timedModifier := NewTimedModifier(string(ActionNameBless), user, modifier, 8)
		timedModifier.Apply()
		g.timekeeper.AddTimed(timedModifier)
	}
}
func (g *GameInstance) getBerserkSelfAction(intervalRange util.Tuple[int,int], turnCount int) func(user *common.Actor) {
	return func(user *common.Actor) {
		applyStatusAction := actions.NewApplyStatusAction(actions.NewActorCause("ApplyStatusEffect", user), user, common.StatusBerserk, turnCount)
		g.actionBroker.Execute(applyStatusAction)

		modifier := AddToBaseAttribute(g.randomSource, stats.BaseDamage, intervalRange)
		timedModifier := NewTimedModifier(string(ActionNameBuffDamage), user, modifier, turnCount)
		timedModifier.Apply()
		g.timekeeper.AddTimed(timedModifier)
		g.Log("Blood for the blood god!")
	}
}

func (g *GameInstance) getApplyStatusAction(statusEffect common.StatusEffect, message string, turnCount int) func(user *common.Actor) {
	return func(user *common.Actor) {
		applyStatusAction := actions.NewApplyStatusAction(actions.NewActorCause("ApplyStatusEffect", user), user, statusEffect, turnCount)
		g.actionBroker.Execute(applyStatusAction)
		g.Log(message)
	}
}

func (g *GameInstance) getRandomTelefragAction(user *common.Actor) {
	currentMap := g.state.GetMap()
	var visEnemies []*common.Actor
	for _, visPos := range user.GetFOV().Visibles {
		if currentMap.IsActorAt(visPos) {
			actorAt := currentMap.ActorAt(visPos)
			if (actorAt.IsHostileTo(user) || g.shiftIsBeingPressedThisTick) && actorAt.IsAlive() && actorAt != user {
				visEnemies = append(visEnemies, actorAt)
			}
		}
	}
	if len(visEnemies) == 0 {
		g.Log("No enemies in sight")
		return
	}
	randomEnemy := visEnemies[g.randomSource.Intn(len(visEnemies))]
	location := randomEnemy.MapPosition()
	isPlayer := user == g.state.GetPlayer()
	moveAction := actions.NewMoveAction(actions.NewActorCause("Telefrag", user), user, location, isPlayer)
	moveAction.DisableAnimation()

	if user == g.state.GetPlayer() {
		g.cancelAutoMove()
		g.CancelTargeting()
		g.actionBroker.CancelAllAnimations()
	}

	changeHealthAmount := -randomEnemy.GetStats().GetRessource(stats.Health).Value()
	killAction := actions.NewChangeRessourceAction(actions.NewActorCause("Telefrag", user), randomEnemy, stats.Health, changeHealthAmount)
	g.actionBroker.Execute(killAction)
	g.actionBroker.Execute(moveAction)
}

func (g *GameInstance) getDelayedRandomTeleportAction(user *common.Actor) {
	currentMap := g.state.GetMap()
	origin := user.MapPosition()
	location, found := currentMap.GetRandomLocation(func(location geometry.Point) bool {
		dist := location.ChebyshevDistanceTo(origin)
		return currentMap.IsCurrentlyPassable(location) && dist > 8 && dist < 14
	})
	if !found {
		return
	}
	isPlayer := user == g.state.GetPlayer()
	moveAction := actions.NewMoveAction(actions.NewActorCause("Teleport", user), user, location, isPlayer)
	moveAction.DisableAnimation()

	if user == g.state.GetPlayer() {
		g.cancelAutoMove()
		g.CancelTargeting()
		g.actionBroker.CancelAllAnimations()
	}

	g.actionBroker.QueueForExecution(moveAction)
}
func (g *GameInstance) getRandomTeleportAction(user *common.Actor) {
	currentMap := g.state.GetMap()
	origin := user.MapPosition()
	location, found := currentMap.GetRandomLocation(func(location geometry.Point) bool {
		dist := location.ChebyshevDistanceTo(origin)
		return currentMap.IsCurrentlyPassable(location) && dist > 8 && dist < 14
	})
	if !found {
		return
	}
	isPlayer := user == g.state.GetPlayer()
	moveAction := actions.NewMoveAction(actions.NewActorCause("Teleport", user), user, location, isPlayer)
	moveAction.DisableAnimation()

	if user == g.state.GetPlayer() {
		g.cancelAutoMove()
		g.CancelTargeting()
		g.actionBroker.CancelAllAnimations()
	}

	g.actionBroker.QueueForExecution(moveAction)
}

func (g *GameInstance) getPlayerRecallAction(user *common.Actor) {
	g.recallAtEndOfTurn()
}

/*
Name() string

	GetTooltipLines() []util.LabelText
	InventoryIcon() int32
	TintColor() color.Color
*/
type TimedModifier struct {
	user      *common.Actor
	modifier  *stats.Modifier
	turnsLeft int
	energy    int
	icon      int32
	name      string
}

func (t *TimedModifier) TintColor() color.Color {
	return color.White
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
func (t *TimedModifier) GetTooltipLines() []util.LabelText {
	return []util.LabelText{
		{
			Text:               t.modifier.ToString(),
			TextColor:          common.DarkTextColor,
			UseColorCodes:      false,
			TextWithColorCodes: t.Name(),
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
func NewTimedModifier(name string, user *common.Actor, modifier *stats.Modifier, turns int) *TimedModifier {
	return &TimedModifier{
		name:      name,
		user:      user,
		modifier:  modifier,
		turnsLeft: turns,
	}
}

func (g *GameInstance) activateUntargetedAction(actor *common.Actor, action *common.UntargetedAction) {
	action.Reset()
	if action.IsDelayed() {
		// add to timekeeper
		g.Log(fmt.Sprintf("%s is delayed for %d turns", action.Name(), action.GetDelayInTurns()))
		action.SetCurrentUser(actor)
		g.timekeeper.AddTimed(action)
	} else {
		action.Invoke(actor)
	}
}

func (g *GameInstance) getVisibleEnemiesByDistance(user *common.Actor) []*common.Actor {
	currentMap := g.GetWorldState().GetMap()
	var nearbyEnemies []*common.Actor
	for _, visPos := range user.GetFOV().Visibles {
		if !currentMap.IsActorAt(visPos) {
			continue
		}
		actor := currentMap.ActorAt(visPos)
		if (actor.IsHostileTo(user) || g.shiftIsBeingPressedThisTick) && actor.IsAlive() && user != actor {
			nearbyEnemies = append(nearbyEnemies, actor)
		}
	}
	slices.SortStableFunc(nearbyEnemies, func(i, j *common.Actor) int {
		distI := currentMap.MoveDistance(user.MapPosition(), i.MapPosition())
		distJ := currentMap.MoveDistance(user.MapPosition(), j.MapPosition())
		return cmp.Compare(distI, distJ)
	})
	return nearbyEnemies
}



func  (g *GameInstance) NewConditionFromString(condition string) func(user *common.Actor) bool {
	if condition == "can_replicate" {
		return g.canReplicate
	}
	g.ErrLog(fmt.Sprintf("no condition found for name %s", condition))
	return func(user *common.Actor) bool { return true }
}

func (g *GameInstance) canReplicate(user *common.Actor) bool {
	currentMap := g.state.GetMap()

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
		if !currentMap.Contains(p) || !currentMap.IsActorAt(p) {
			return false
		}
		actorAt := currentMap.ActorAt(p)
		return actorAt.GetInternalName() == user.GetInternalName()
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
