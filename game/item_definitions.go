package game

import (
	"fmt"
	"image/color"
	"io"
	"math/rand"
	"regexp"
	"rpg/common"
	"rpg/recfile"
	"rpg/stats"
	"rpg/util"
	"strconv"
)


//

// item line for typical armor
// name, icon, dlvl, armor_type, armor_class

type LightDefinition struct {
	LightColor string
	FlickerPattern string
	FlickerFrameDelay float64
}

func (d LightDefinition) IsValid() bool {
	return d.LightColor != "" && d.FlickerPattern != "" && d.FlickerFrameDelay > 0
}

// item line for typical weapon
// name, icon, dlvl, weapon_type, weapon_damage, weapon_speed
type ItemDefinition struct {
	Name         string
	InternalName string
	Icon         int32
	DLvl         int // min dungeon level at which this item can appear (=native depth)
	MaxLevel     int // max level at which this item can appear

	OutOfDepthChance float64 // chance to appear out of depth
	MinLevelForOoD   int

	UntargetedAction string
	TargetedAction string

	InventoryCategory common.ItemCategory
	Armor             ArmorDefinition
	Weapon            WeaponDefinition
	Throw             ThrowDefinition
	Light             LightDefinition
	Cost              int
	Modifiers         []string

	Charges          int
}

func (d ItemDefinition) CanAppearAt(randomSource *rand.Rand, dungeonLevel int) bool {
	// reject early
	if d.MaxLevel > 0 && dungeonLevel > d.MaxLevel {
		return false
	}
	regularAppearance := dungeonLevel >= d.DLvl

	// apply out of depth chance
	oodAppearance := dungeonLevel >= d.MinLevelForOoD && randomSource.Float64() < d.OutOfDepthChance

	return regularAppearance || oodAppearance
}

func (d ItemDefinition) IsArmor() bool {
	return d.Armor.IsValid()
}

func (d ItemDefinition) IsWeapon() bool {
	return d.Weapon.IsValid()
}

func (d ItemDefinition) IsThrow() bool {
	return d.Throw.IsValid()
}

func (d ItemDefinition) IsAccessory() bool {
	return d.InventoryCategory == common.ItemCategoryAccessory
}

func (d ItemDefinition) IsLightSource() bool {
	return d.Light.IsValid()
}

func (d ItemDefinition) IsPotion() bool {
	return d.InventoryCategory == common.ItemCategoryPotion
}

func (d ItemDefinition) IsScroll() bool {
	return d.InventoryCategory == common.ItemCategoryScroll
}

type ArmorDefinition struct {
	ArmorType  common.ArmorType
	ArmorClass stats.Interval
}

func (d ArmorDefinition) IsValid() bool {
	return d.ArmorType != common.ArmorTypeNone
}

type WeaponDefinition struct {
	WeaponType common.WeaponType
	Damage     stats.Interval
	Speed      float64
}

func (d WeaponDefinition) IsValid() bool {
	return d.WeaponType != common.WeaponTypeNone
}

type ThrowDefinition struct {
	Damage        stats.Interval
	ChanceToBreak float64
	HitZone       string
	OnHitEffect   string
}

func (d ThrowDefinition) IsValid() bool {
	return d.OnHitEffect != "" || d.Damage.NotZero()
}

func (g *GameInstance) GetItemDefinitions() map[string]ItemDefinition {
	return g.itemDefinitions
}
func ReadItemDefinitions(file io.Reader, logErr func(errString string)) map[string]ItemDefinition {
	records := recfile.Read(file)
	itemDefinitions := make(map[string]ItemDefinition, len(records))
	for _, record := range records {
		definition := NewItemDefinitionFromRecord(record)
		if definition.InternalName == "" {
			logErr(fmt.Sprintf("Item definition has no internal name: %v", definition))
			continue
		}
		if _, exists := itemDefinitions[definition.InternalName]; exists {
			logErr(fmt.Sprintf("Duplicate item definition: %s", definition.InternalName))
			continue
		}
		if definition.Armor.ArmorClass.NotZero() && definition.Armor.ArmorType == common.ArmorTypeNone {
			logErr(fmt.Sprintf("Armor definition has invalid armor type: %v", definition))
			continue
		}
		if definition.Weapon.Damage.NotZero() && definition.Weapon.WeaponType == common.WeaponTypeNone {
			logErr(fmt.Sprintf("Weapon definition has invalid weapon type: %v", definition))
			continue
		}
		itemDefinitions[definition.InternalName] = definition
	}
	return itemDefinitions
}
func (g *GameInstance) NewItemFromDefinitionName(itemString string, dungeonLevel int) *GameItem {
	if def, exists := g.itemDefinitions[itemString]; exists {
		return g.NewItemFromDefinition(def, dungeonLevel)
	}
	return nil
}
func NewItemDefinitionFromRecord(record recfile.Record) ItemDefinition {
	itemDefinition := ItemDefinition{}
	for _, field := range record {
		switch field.Name {
		case "name":
			itemDefinition.Name = field.Value
		case "internal_name":
			itemDefinition.InternalName = field.Value
		case "icon":
			itemDefinition.Icon = field.AsInt32()
		case "dlvl":
			itemDefinition.DLvl = field.AsInt()
		case "cost": itemDefinition.Cost = field.AsInt()
		case "charges":
			itemDefinition.Charges = field.AsInt()
		case "max_level":
			itemDefinition.MaxLevel = field.AsInt()
		case "out_of_depth_chance":
			itemDefinition.OutOfDepthChance = field.AsFloat()
		case "min_level_for_ood":
			itemDefinition.MinLevelForOoD = field.AsInt()
		case "category":
			itemDefinition.InventoryCategory = common.ItemCategoryFromString(field.Value)
		case "armor_type":
			itemDefinition.Armor.ArmorType = common.ArmorTypeFromString(field.Value)
		case "armor_class":
			itemDefinition.Armor.ArmorClass = stats.NewIntervalFromString(field.Value)
		case "weapon_type":
			itemDefinition.Weapon.WeaponType = common.WeaponTypeFromString(field.Value)
		case "weapon_damage":
			itemDefinition.Weapon.Damage = stats.NewIntervalFromString(field.Value)
		case "weapon_speed":
			itemDefinition.Weapon.Speed = field.AsFloat()
		case "throw_hit_zone":
			itemDefinition.Throw.HitZone = field.Value
		case "throw_damage":
			itemDefinition.Throw.Damage = stats.NewIntervalFromString(field.Value)
		case "throw_chance_to_break":
			itemDefinition.Throw.ChanceToBreak = field.AsFloat()
		case "throw_on_hit_effect":
			itemDefinition.Throw.OnHitEffect = field.Value
		case "light_color":
			itemDefinition.Light.LightColor = field.Value
		case "light_flicker_pattern":
			itemDefinition.Light.FlickerPattern = field.Value
		case "light_flicker_frame_delay":
			itemDefinition.Light.FlickerFrameDelay = field.AsFloat()
		case "equip_modifier": itemDefinition.Modifiers = append(itemDefinition.Modifiers, field.Value)
		case "untargeted_action": itemDefinition.UntargetedAction = field.Value
		case "targeted_action": itemDefinition.TargetedAction = field.Value
		}
	}

	return itemDefinition
}

func (g *GameInstance) NewItemFromDefinition(itemDefinition ItemDefinition, dungeonLevel int) *GameItem {
	baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, itemDefinition.Name, itemDefinition.Icon)
	baseItem.SetBaseCost(itemDefinition.Cost)
	baseItem.SetCategory(itemDefinition.InventoryCategory)
	baseItem.SetQualityLevel(itemDefinition.DLvl)

	if itemDefinition.Armor.IsValid() {
		armorPart := &common.ArmorStats{
			BaseArmor:  itemDefinition.Armor.ArmorClass.LinearRandom(),
			Durability: stats.NewResource(0, flatFunc(20), 20),
			Type:       itemDefinition.Armor.ArmorType,
		}
		baseItem.SetArmorComponent(armorPart)
	}

	if itemDefinition.Weapon.IsValid() {
		weaponPart := &common.WeaponStats{
			Damage:            itemDefinition.Weapon.Damage,
			Durability:        stats.NewResource(0, flatFunc(20), 20),
			Type:              itemDefinition.Weapon.WeaponType,
			MeleeRange:        1,
			AttackSpeedFactor: itemDefinition.Weapon.Speed,
		}
		baseItem.SetWeaponComponent(weaponPart)
	}

	if itemDefinition.Throw.IsValid() {
		hitZone := g.NewHitZoneFromString(itemDefinition.Throw.HitZone)
		if hitZone != nil {
			throwPart := &common.ThrowStats{
				Damage:        itemDefinition.Throw.Damage,
				HitZone:       hitZone,
				ChanceToBreak: itemDefinition.Throw.ChanceToBreak,
				OnHitEffect:   g.NewOnHitEffectFromString(itemDefinition.Throw.OnHitEffect),
			}
			baseItem.SetThrowComponent(throwPart)
		}
	}

	if itemDefinition.Light.IsValid() {
		lightPart := &common.LightStats{
			LightColor: GetColorFromRGBString(itemDefinition.Light.LightColor),
			FlickerPattern: GetFlickerPatternFromName(itemDefinition.Light.FlickerPattern),
			FlickerFrameDelay: itemDefinition.Light.FlickerFrameDelay,
		}
		baseItem.SetLightComponent(lightPart)
	}

	for _, modifierName := range itemDefinition.Modifiers {
		var mod *stats.Modifier
		if util.LooksLikeAFunction(modifierName) {
			modName, args := util.GetNameAndArgs(modifierName)
			modValue := args.GetFloat(0)
			mod = g.NewModifierFromNameWithFixedBonus(modName, modValue)
		} else {
			mod = g.NewModifierFromNameWithRandomRange(modifierName, dungeonLevel)
		}
		if mod != nil {
			baseItem.AddEquippedModifier(mod)
		}
	}
	charges := itemDefinition.Charges
	destroyAfterUse := false

	if itemDefinition.InventoryCategory.HasAlwaysOneCharge() {
		charges = 1
		destroyAfterUse = true
	}

	var untargetedAction *common.UntargetedAction
	if itemDefinition.UntargetedAction != "" {
		untargetedAction = g.NewUntargetedActionFromName(itemDefinition.UntargetedAction)
	}

	var targetedAction *common.TargetedAction
	if itemDefinition.TargetedAction != "" {
		targetedAction = g.NewTargetedActionFromName(itemDefinition.TargetedAction)
	}

	return &GameItem{
		BaseItem: baseItem,
		actionCharges: charges,
		destroyAfterUse: destroyAfterUse,
		untargetedAction: untargetedAction,
		targetedAction: targetedAction,
	}
}

func GetColorFromRGBString(lightColor string) color.RGBA {
	// looks like "234 12 124"
	regexpPattern := regexp.MustCompile(`([0-9]{1,3}) +([0-9]{1,3}) +([0-9]{1,3})`)
	matches := regexpPattern.FindStringSubmatch(lightColor)
	if matches == nil {
		return color.RGBA{}
	}
	r, _ := strconv.Atoi(matches[1])
	g, _ := strconv.Atoi(matches[2])
	b, _ := strconv.Atoi(matches[3])
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

func GetFlickerPatternFromName(pattern string) []float64 {
	switch pattern {
	case "fire":
		return getFireFlickerPattern()
	case "smooth":
		return getSmoothFlickerPattern()
	}
	return nil
}
