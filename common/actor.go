package common

import (
	"fmt"
	"image/color"
	"math/rand"
	"rpg/geometry"
	"rpg/stats"
	"rpg/util"
	"strings"
)

type VendorType string

const (
	VendorTypeNone    VendorType = ""
	VendorTypeGeneral VendorType = "VendorGeneral"
	VendorTypeWands   VendorType = "VendorWands"
	VendorTypeArmor   VendorType = "VendorArmor"
	VendorTypeWeapons VendorType = "VendorWeapons"
)

type Actor struct {
	mapPosition  geometry.Point
	drawPosition geometry.PointF
	icon         int32
	fov          *geometry.FOV
	name         string

	inventory           *Inventory
	equipment           *Equipment
	statusEffects       *Flags
	isAnimatingMovement bool

	stats      stats.AttributeHolder
	timeEnergy int

	state ActorState

	hurtTicks int

	isVendorOfType     VendorType
	lastMoveWasWest    bool
	isHostile          bool
	speechMap          map[ActorSpeechTrigger][]string
	isUnique           bool
	internalName       string
	disableCorpse      bool
	deathStringFormats []string
	targetedSkills     []*TargetedAction
	untargetedSkills   []*UntargetedAction
	corpseIcon         int32
}

type AttackType int

const (
	MeleeAttack AttackType = iota
	RangedAttack
)

func (a *Actor) Tick() {
	if a.IsHurt() {
		a.hurtTicks--
	}
}
func (a *Actor) GetHurtTicks() int {
	return a.hurtTicks
}

func (a *Actor) SetHurtTicks(hurtTicks int) {
	a.hurtTicks = hurtTicks
}

func (a *Actor) SetHurt() {
	a.hurtTicks = int(a.hurtTicksNeeded())
}
func (a *Actor) hurtTicksNeeded() int {
	return int(util.SecondsToTicks(0.2))
}

func (a *Actor) GetHurtTicksAsPercentage() float64 {
	return float64(a.hurtTicks) / float64(a.hurtTicksNeeded())
}

func (a *Actor) IsHurt() bool {
	return a.hurtTicks > 0
}
func (a *Actor) AddEnergy(energy int) {
	//println(fmt.Sprintf("%s restored %d energy", a.name, energy))
	a.timeEnergy += energy
}

func NewActor(icon int32) *Actor {
	actorStats := stats.NewAttributeHolder()
	visionRange := actorStats.GetAttribute(stats.VisionRange)
	return &Actor{
		icon:      icon,
		stats:     actorStats,
		inventory: NewInventory(32),
		equipment: NewEquipment(),
		fov:       geometry.NewFOV(geometry.NewRect(-visionRange, -visionRange, visionRange, visionRange)),
		statusEffects: NewFlags(),
		corpseIcon: IconCorpse,
	}
}

func (a *Actor) GetInventory() *Inventory {
	return a.inventory
}
func (a *Actor) GetEquipment() *Equipment {
	return a.equipment
}
func (a *Actor) VisionMap() []geometry.Point {
	return a.fov.Visibles
}

func (a *Actor) SetIcon(newIcon int32) {
	a.icon = newIcon
}

func (a *Actor) IsAlive() bool {
	return a.GetStats().GetRessource(stats.Health).IsPositive()
}

func (a *Actor) Name() string {
	return a.name
}

func (a *Actor) GetStats() *stats.AttributeHolder {
	return &a.stats
}

func (a *Actor) MapPosition() geometry.Point {
	return a.mapPosition
}
func (a *Actor) SetMapPosition(pos geometry.Point) {
	if a.mapPosition != pos {
		a.SetLookDirFromPoI(pos)
	}
	a.mapPosition = pos
}

func (a *Actor) SetLookDirFromPoI(location geometry.Point) {
	if a.mapPosition.X > location.X {
		a.lastMoveWasWest = true
	} else if a.mapPosition.X < location.X {
		a.lastMoveWasWest = false
	}
}

func (a *Actor) DrawPosition() geometry.PointF {
	return a.drawPosition
}

func (a *Actor) SetDrawPosition(pos geometry.PointF) {
	a.drawPosition = pos
}

func (a *Actor) Icon(tick uint64) int32 {
	if !a.IsAlive() {
		return a.corpseIcon
	}
	return a.icon
}

func (a *Actor) IsHostileTo(actor *Actor) bool {
	// TODO: implement factions
	return a != actor && a.isHostile
}

func (a *Actor) CanSee(pos geometry.Point) bool {
	return a.fov.Visible(pos)
}

func (a *Actor) GetFOV() *geometry.FOV {
	return a.fov
}

func (a *Actor) IsAnimatingMovement() bool {
	return a.isAnimatingMovement
}

func (a *Actor) SetMovingAnimation(isAnimatingMovement bool) {
	a.isAnimatingMovement = isAnimatingMovement
}

func (a *Actor) SetName(name string) {
	a.name = name
}

func (a *Actor) GetEnergy() int {
	return a.timeEnergy
}

func (a *Actor) RemoveEnergy(energy int) {
	a.timeEnergy -= energy
	//println(fmt.Sprintf("%s used %d energy", a.name, energy))
	a.BeforeTurn()
}

func (a *Actor) GetAttackSpeed(attackType AttackType) int {
	// TODO - missing:
	// - class based attack speed modifier per weapon type (eg. barb swinging axes faster, mages using staves faster)
	baseAttackSpeed := float64(10) * a.stats.GetAttributeAsFloat(stats.AttackSpeedFactor)

	weapons := a.GetEquipment().GetWeapons(attackType)

	if len(weapons) == 0 {
		return int(baseAttackSpeed)
	}

	slowerWeapon := weapons[0]

	if len(weapons) > 1 && weapons[1].GetWeaponStats().AttackSpeedFactor < weapons[0].GetWeaponStats().AttackSpeedFactor {
		slowerWeapon = weapons[1]
	}

	attackSpeed := int(baseAttackSpeed * slowerWeapon.GetWeaponStats().AttackSpeedFactor)
	return attackSpeed
}

func (a *Actor) GetMoveSpeed() int {
	baseSpeed := float64(10)
	moveFactor := a.stats.GetAttributeAsFloat(stats.MoveSpeedFactor)
	moveSpeed := int(baseSpeed * moveFactor)
	return moveSpeed
}
func (a *Actor) GetDamageInterval(attackType AttackType) stats.Interval {
	weapons := a.GetEquipment().GetWeapons(attackType)
	hasWeapons := len(weapons) > 0

	var statBonus int // a bonus from the corresponding stat for this activity, reflects the skill of the actor
	if attackType == RangedAttack  {
		statBonus = a.GetStats().GetAttribute(stats.Dexterity)
	} else if attackType == MeleeAttack {
		statBonus = a.GetStats().GetAttribute(stats.Strength)
	}

	// base damage bonus, defaults to zero, usually from equipment providing a bonus to damage
	baseDamage := a.GetStats().GetAttribute(stats.BaseDamage)

	minDmg := baseDamage + (statBonus/4)
	maxDmg := baseDamage + (statBonus/4)
	weaponPlus := 0
	if hasWeapons {
		for _, w := range weapons { // damage is summed for all equipped weapons
			// weapon damage interval comes from the weapon itself
			// plus the base damage bonus
			// plus the stat bonus divided by 4
			weaponPlus += w.GetWeaponStats().Plus
			minDmg += w.GetWeaponStats().Damage.Min() + weaponPlus
			maxDmg += w.GetWeaponStats().Damage.Max() + weaponPlus
		}
	} else { // unarmed receives the stat bonus 1:1 as damage, -1..+1
		minDmg = max(1, baseDamage + (statBonus/4))
		maxDmg = baseDamage+(statBonus/4) + 2
	}

	// last step is applying the enhanced damage factor, which is a multiplier
	// defaults to 1.0 and usually is provided from equipment providing a percentage bonus to damage
	enhancedDamageFactor := a.GetStats().GetAttributeAsFloat(stats.EnhancedDamageFactor)
	minDmg = int(enhancedDamageFactor*float64(minDmg))
	maxDmg = int(enhancedDamageFactor*float64(maxDmg))

	return stats.NewInterval(minDmg, maxDmg)
}
func (a *Actor) SetBaseSpeed(speed int) {
	a.stats.SetAttribute(stats.BaseSpeed, speed)
}
func (a *Actor) GetBaseSpeed() int {
	return a.stats.GetAttribute(stats.BaseSpeed)
}
func (a *Actor) GetInfoText() []string {
	tableLines := []string{
		fmt.Sprintf("HP: %s", a.GetStats().GetRessource(stats.Health).ToString()),
		fmt.Sprintf("MP: %s", a.GetStats().GetRessource(stats.Mana).ToString()),
		fmt.Sprintf("Str: %d", a.GetStats().GetAttribute(stats.Strength)),
		fmt.Sprintf("Dex: %d", a.GetStats().GetAttribute(stats.Dexterity)),
		fmt.Sprintf("Int: %d", a.GetStats().GetAttribute(stats.Intelligence)),
		fmt.Sprintf("ADR: %d%%", int(a.GetArmorDamageReduction(0)*100)),
		fmt.Sprintf("Energy: %d", a.GetEnergy()),
		fmt.Sprintf("Movement speed: %d", a.GetMoveSpeed()),
	}

	armor := a.GetEquipment().GetArmor()
	if len(armor) > 0 {
		var armorInfo []string
		armorInfo = append(armorInfo, "", "Armor")
		for _, armorPiece := range armor {
			armorInfo = append(armorInfo, " " + armorPiece.Name())
		}
		tableLines = append(tableLines, armorInfo...)
	}

	meleeWeapons := a.GetEquipment().GetWeapons(MeleeAttack)
	var meleeInfo []string
	if len(meleeWeapons) > 0 {
		meleeInfo = append(meleeInfo, "", "Melee weapons")
		for _, weapon := range meleeWeapons {
			meleeInfo = append(meleeInfo, " " + weapon.Name())
		}
	} else {
		meleeInfo = append(meleeInfo, "", "Melee weapon")
		meleeInfo = append(meleeInfo, " bare hands")
	}
	meleeInfo = append(meleeInfo, []string{
		fmt.Sprintf("Melee damage: %s", a.GetDamageInterval(MeleeAttack).ToString()),
		fmt.Sprintf("Melee speed: %d", a.GetAttackSpeed(MeleeAttack)),
	}...)
	tableLines = append(tableLines, meleeInfo...)

	hasRanged := a.GetEquipment().HasRangedWeaponEquipped()
	if hasRanged {
		var rangedInfo []string
		rangedWeapons := a.GetEquipment().GetWeapons(RangedAttack)
		if len(rangedWeapons) > 0 {
			rangedInfo = append(rangedInfo, "", "Ranged weapons")
			for _, weapon := range rangedWeapons {
				rangedInfo = append(rangedInfo, " " + weapon.Name())
			}

			rangedInfo = append(rangedInfo, []string{
				fmt.Sprintf("Ranged damage: %s", a.GetDamageInterval(RangedAttack).ToString()),
				fmt.Sprintf("Ranged speed: %d", a.GetAttackSpeed(RangedAttack)),
			}...)
		}
		tableLines = append(tableLines, rangedInfo...)
	}

	utSkills := a.GetUntargetedSkills()
	taSkills := a.GetTargetedSkills()
	if len(utSkills) > 0 || len(taSkills) > 0 {
		tableLines = append(tableLines, "", "Skills")
	}
	for _, utSkill := range utSkills {
		tableLines = append(tableLines, " " + utSkill.Name())
	}
	for _, taSkill := range taSkills {
		tableLines = append(tableLines, " " + taSkill.Name())
	}

	baseInfoTable := tableLines

	nameInfo := []string{a.name}
	if a.IsVendor() {
		nameInfo = append(nameInfo, "Vendor", "")
	} else {
		nameInfo = append(nameInfo, "")
	}

	statusEffects := a.statusEffects.AsStrings()

	if len(statusEffects) > 0 {
		baseInfoTable = append(baseInfoTable, "")
		baseInfoTable = append(baseInfoTable, statusEffects...)
	}

	return append(nameInfo, baseInfoTable...)
}

func (a *Actor) IsVendor() bool {
	return a.isVendorOfType != VendorTypeNone
}

func (a *Actor) IsHostile() bool {
	return a.isHostile
}

type ActorState int

const (
	ActorStateIdle ActorState = iota
	ActorStateAttacking
)

type ActorSpeechTrigger int

const (
	SpeechTriggerSight ActorSpeechTrigger = iota
	SpeechTriggerHurt
)

func (a *Actor) SetState(newState ActorState) {
	a.state = newState
}

func (a *Actor) GetState() ActorState {
	return a.state
}

func (a *Actor) GetDamageLocationPredictions(attackType AttackType, targetPos geometry.Point) []LocationDamagePrediction {
	weapons := a.GetEquipment().GetWeapons(attackType)

	if len(weapons) == 0 {
		return []LocationDamagePrediction{{Location: targetPos, Damage: a.GetDamageInterval(attackType)}}
	}
	attackerPos := a.MapPosition()

	var damageLocations []LocationDamagePrediction

	totalMeleeDamage := a.GetDamageInterval(attackType)
	weaponCount := len(weapons)
	for _, weapon := range weapons {
		damageLocations = append(damageLocations, weapon.GetWeaponStats().GetDamagePerLocationPrediction(attackerPos, targetPos, totalMeleeDamage, weaponCount)...)
	}

	return damageLocations
}

func (a *Actor) GetArmorClass() int {
	baseArmorClass := a.stats.GetAttribute(stats.BaseArmorClass)
	baseArmorClassMultiplier := a.stats.GetAttributeAsFloat(stats.EnhancedArmorClassFactor)
	armor := baseArmorClass
	for _, item := range a.GetEquipment().GetArmor() {
		armor += item.GetArmorStats().GetArmorClass()
	}
	armor = int(float64(armor) * baseArmorClassMultiplier)
	return armor
}
/*
Armor Rating	Damage Reduction in %
1	0.01
5	0.05
25	0.20
50	0.33
100	0.50
200	0.67
1000	0.91
*/
func (a *Actor) GetArmorDamageReduction(attackerArmorPenetration int) float64 {
	armorClass := float64(max(0, a.GetArmorClass()-attackerArmorPenetration))
	magicalConstantForDiminishingReturns := 100.0
	armorDamageReduction := util.Clamp(0, 0.9, armorClass/(armorClass+magicalConstantForDiminishingReturns))
	return armorDamageReduction
}

func (a *Actor) HealCompletely() {
	a.GetStats().GetRessource(stats.Health).SetToMax()
	a.GetStats().GetRessource(stats.Mana).SetToMax()
}

func (a *Actor) SetVendorType(vendorType VendorType) {
	a.isVendorOfType = vendorType
	if vendorType != VendorTypeNone {
		a.isHostile = false
	}
}

func (a *Actor) SetNeutral() {
	a.isHostile = false
	a.state = ActorStateIdle
}

func (a *Actor) SetHostile() {
	a.isHostile = true
}

func (a *Actor) GetVendorType() VendorType {
	return a.isVendorOfType
}

func (a *Actor) CanLevelUp() bool {
	currentLevel := a.GetStats().GetAttribute(stats.Level)
	currentXP := a.GetStats().GetAttribute(stats.Experience)
	xpNeeded := a.GetXPForLevel(currentLevel + 1)
	if currentXP >= xpNeeded {
		return true
	}
	return false
}

func (a *Actor) GetXPForLevel(level int) int {
	if level == 1 {
		return 0
	}
	if level == 2 {
		return 100
	} else {
		return a.GetXPForLevel(level-1) + 250
	}
}

func (a *Actor) LevelUp(chosenAttribute stats.Name) {
	pStats := a.GetStats()
	pStats.SetAttribute(stats.Level, pStats.GetAttribute(stats.Level)+1)
	pStats.SetAttribute(chosenAttribute, pStats.GetAttribute(chosenAttribute)+1)

	strength := pStats.GetAttribute(stats.Strength)
	maxHealth := pStats.GetAttribute(stats.MaxHealth)
	pStats.SetAttribute(stats.MaxHealth, maxHealth+int(float64(strength)*0.5))

	intelligence := pStats.GetAttribute(stats.Intelligence)
	maxMana := pStats.GetAttribute(stats.MaxMana)
	pStats.SetAttribute(stats.MaxMana, maxMana+int(float64(intelligence)*0.5))

	pStats.GetRessource(stats.Health).SetToMax()
	pStats.GetRessource(stats.Mana).SetToMax()
}

func (a *Actor) FlipX() bool {
	// spritesheet default is looking right
	return a.lastMoveWasWest
}

func (a *Actor) SetFlipLook(flip bool) {
	a.lastMoveWasWest = flip
}

func (a *Actor) CanPickup() bool {
	return !a.inventory.IsFull()
}


func (a *Actor) BeforeTurn() {
	a.regenerateBasicResources()
	a.handleStatusEffects()
}

func (a *Actor) regenerateBasicResources() {
	statsForActor := a.GetStats()
	hpRegenPerTurn := statsForActor.GetAttributeAsFloat(stats.HealthRegeneration)
	mpRegenPerTurn := statsForActor.GetAttributeAsFloat(stats.ManaRegeneration)

	statsForActor.GetRessource(stats.Health).IncreaseByFloat(hpRegenPerTurn)
	statsForActor.GetRessource(stats.Mana).IncreaseByFloat(mpRegenPerTurn)
}
func (a *Actor) handleStatusEffects() {
	for flagName, value := range a.statusEffects.AllEntries() {
		//effectName := StatusEffect(flagName)
		if value > 0 {
			newValue := value - 1
			if newValue == 0 {
				a.statusEffects.SetFalse(flagName)
			} else {
				a.statusEffects.SetToValue(flagName, newValue)
				if StatusEffect(flagName) == StatusDamageOverTime {
					// hmm, we now need parameters for the status effect
				}
			}
		}
	}
}

func (a *Actor) HasSpeech(trigger ActorSpeechTrigger) bool {
	return len(a.speechMap[trigger]) > 0
}

func (a *Actor) GetSpeech(random *rand.Rand, trigger ActorSpeechTrigger) []string {
	allSpeech := a.speechMap[trigger]
	if len(allSpeech) == 0 {
		return []string{}
	}
	chosenIndex := random.Intn(len(allSpeech))
	chosenLines := allSpeech[chosenIndex]
	//remove
	a.speechMap[trigger] = append(allSpeech[:chosenIndex], allSpeech[chosenIndex+1:]...)
	return strings.Split(chosenLines, "\n")
}

func (a *Actor) SetSpeech(speechMap map[ActorSpeechTrigger][]string) {
	a.speechMap = speechMap
}

func (a *Actor) SetUnique() {
	a.isUnique = true
}

func (a *Actor) IsUnique() bool {
	return a.isUnique
}

func (a *Actor) GetInternalName() string {
	return a.internalName
}

func (a *Actor) SetInternalName(internalName string) {
	a.internalName = internalName
}

func (a *Actor) SetNoCorpse() {
	a.disableCorpse = true
}

func (a *Actor) HasCorpse() bool {
	return !a.disableCorpse
}

func (a *Actor) GetDeathStringFormat() string {
	randomSource := rand.New(rand.NewSource(int64(a.GetStats().GetAttribute(stats.Health))))
	formats := a.deathStringFormats
	if len(formats) == 0 {
		formats = []string{
			"%s died.",
			"%s was killed.",
			"%s was slain.",
		}
	}
	return formats[randomSource.Intn(len(formats))]
}

func (a *Actor) SetDeathStringFormats(formats []string) {
	a.deathStringFormats = formats
}

func (a *Actor) GetStatusEffects() *Flags {
	return a.statusEffects
}

func (a *Actor) AddStatusEffect(effect StatusEffect, turnCount int) {
	a.statusEffects.SetToValue(string(effect), turnCount)
}
func (a *Actor) HasStatusEffect(effect StatusEffect) bool {
	return a.statusEffects.IsSet(string(effect))
}

func (a *Actor) SetTargetedSkills(skills []*TargetedAction) {
	a.targetedSkills = skills
}

func (a *Actor) GetTargetedSkills() []*TargetedAction {
	return a.targetedSkills
}

func (a *Actor) SetUntargetedSkills(skills []*UntargetedAction) {
	a.untargetedSkills = skills
}

func (a *Actor) GetUntargetedSkills() []*UntargetedAction {
	return a.untargetedSkills
}

func (a *Actor) VisionRange() int {
	return a.stats.GetAttribute(stats.VisionRange)
}

func (a *Actor) SetCorpseIcon(corpse int32) {
	a.corpseIcon = corpse
}

func (a *Actor) GetStealableItems() []Item {
	var stealableItems []Item
	for _, item := range a.GetInventory().Items() {
		if !a.GetEquipment().IsEquipped(item) {
			stealableItems = append(stealableItems, item)
		}
	}
	return stealableItems
}

func (a *Actor) CanAutoMove() bool {
	if !a.IsAlive() {
		return false
	}
	if a.HasStatusEffect(StatusStunned) {
		return false
	}
	if a.HasStatusEffect(StatusBerserk) {
		return false
	}
	if a.HasStatusEffect(StatusConfused) {
		return false
	}
	if a.HasStatusEffect(StatusImmobile) {
		return false
	}
	return true
}


func (a *Actor) CanUseItems() bool {
	if !a.IsAlive() {
		return false
	}
	if a.HasStatusEffect(StatusStunned) {
		return false
	}
	if a.HasStatusEffect(StatusBerserk) {
		return false
	}
	return true
}

func (a *Actor) GetFullStats() []util.LabelText {
	ourStats := a.GetStats()
	var hpColor color.Color
	var manaColor color.Color
	defaultColor := DarkTextColor
	hpColor = defaultColor
	manaColor = defaultColor
	colorFromModifierType := func(mType stats.ModifierImpact) color.Color {
		switch mType {
		case stats.ModifierImpactPositive:
			return PositiveHighlightColor
		case stats.ModifierImpactNegative:
			return NegativeHighlightColor
		default:
			return defaultColor
		}
	}
	if ourStats.GetRessource(stats.Health).IsLessThanPercent(0.25) {
		hpColor = color.RGBA{R: 255, G: 30, B: 30, A: 255}
	} else if ourStats.GetRessource(stats.Health).IsLessThanPercent(0.5) {
		hpColor = color.RGBA{R: 255, G: 150, B: 30, A: 255}
	} else if ourStats.GetRessource(stats.Health).IsLessThanPercent(0.75) {
		hpColor = color.RGBA{R: 255, G: 255, B: 30, A: 255}
	}

	if ourStats.GetRessource(stats.Mana).IsLessThanPercent(0.25) {
		manaColor = color.RGBA{R: 255, G: 30, B: 30, A: 255}
	} else if ourStats.GetRessource(stats.Mana).IsLessThanPercent(0.5) {
		manaColor = color.RGBA{R: 255, G: 150, B: 30, A: 255}
	} else if ourStats.GetRessource(stats.Mana).IsLessThanPercent(0.75) {
		manaColor = color.RGBA{R: 255, G: 255, B: 30, A: 255}
	}
	levelColor := defaultColor
	if a.CanLevelUp() {
		levelColor = LightTextColor
	}

	fullStats := []util.LabelText{
		{
			Text:      fmt.Sprintf("Level: %d", ourStats.GetAttribute(stats.Level)),
			TextColor: levelColor,
		},
		{
			Text:      fmt.Sprintf("Exp: %d", ourStats.GetAttribute(stats.Experience)),
			TextColor: levelColor,
		},
		{
			Text:      fmt.Sprintf("Next: %d", a.GetXPForLevel(ourStats.GetAttribute(stats.Level)+1)),
			TextColor: levelColor,
		},
		{
			Text:      fmt.Sprintf("Gold: %d", a.GetStats().GetAttribute(stats.Gold)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("HP: %s", ourStats.GetRessource(stats.Health).ToString()),
			TextColor: hpColor,
		},
		{
			Text:      fmt.Sprintf("MP: %s", ourStats.GetRessource(stats.Mana).ToString()),
			TextColor: manaColor,
		},
		{
			Text:      fmt.Sprintf("Str: %d", ourStats.GetAttribute(stats.Strength)),
			TextColor: colorFromModifierType(ourStats.GetModifierType(stats.Strength)),
		},
		{
			Text:      fmt.Sprintf("Int: %d", ourStats.GetAttribute(stats.Intelligence)),
			TextColor: colorFromModifierType(ourStats.GetModifierType(stats.Intelligence)),
		},
		{
			Text:      fmt.Sprintf("Dex: %d", ourStats.GetAttribute(stats.Dexterity)),
			TextColor: colorFromModifierType(ourStats.GetModifierType(stats.Dexterity)),
		},
		{
			Text:      fmt.Sprintf("AC: %d", a.GetArmorClass()),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("ADR: %d%%", int(a.GetArmorDamageReduction(0)*100)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("Melee dmg: %s", a.GetDamageInterval(MeleeAttack).ToString()),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("Ranged dmg: %s", a.GetDamageInterval(RangedAttack).ToString()),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("Dmg mult: %d%%", int(a.GetStats().GetAttributeAsFloat(stats.EnhancedDamageFactor)*100)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("AC mult: %d%%", int(a.GetStats().GetAttributeAsFloat(stats.EnhancedArmorClassFactor)*100)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("HP regen: %.1f", a.GetStats().GetAttributeAsFloat(stats.HealthRegeneration)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("MP regen: %.1f", a.GetStats().GetAttributeAsFloat(stats.ManaRegeneration)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("HP steal: %d", int(a.GetStats().GetAttributeAsFloat(stats.LifeStealFactor)*100)),
			TextColor: defaultColor,
		},
		{
			Text:      fmt.Sprintf("MP steal: %d%%", int(a.GetStats().GetAttributeAsFloat(stats.ManaStealFactor)*100)),
			TextColor: defaultColor,
		},
	}
	return fullStats
}
