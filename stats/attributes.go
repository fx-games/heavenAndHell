package stats

import (
	"fmt"
	"image/color"
	"math"
	"strings"
)

func StatFromString(statName string) Name {
	return Name(strings.ToLower(statName))
}

type Name string

const (
	// clasic roguelike attributes (SID)
	Strength     Name = "strength"
	Intelligence Name = "intelligence"
	Dexterity    Name = "dexterity"

	// baseline for the game
	Level                    Name = "level"
	Experience               Name = "experience"
	Gold                     Name = "gold"
	BaseSpeed                Name = "speed"
	BaseDamage               Name = "damage"
	BaseArmorClass           Name = "armor_class"
	AttackSpeedFactor        Name = "attack_speed"
	MoveSpeedFactor          Name = "move_speed"
	LifeStealFactor          Name = "life_steal"
	ManaStealFactor          Name = "mana_steal"
	ManaRegeneration     Name = "mana_regeneration"
	HealthRegeneration   Name = "health_regeneration"
	EnhancedDamageFactor Name = "enhanced_damage"
	EnhancedArmorClassFactor Name = "armor_class_multiplier"

	ArmorPenetration Name = "armor_penetration"

	MaxHealth Name = "max_health"
	MaxMana   Name = "max_mana"

	Health Name = "health"
	Mana   Name = "mana"

	VisionRange Name = "vision_range"

	LightRadius Name = "light_radius"
)

func GetAllAttributeNames() []Name {
	return []Name{
		Level,
		Strength,
		Intelligence,
		Dexterity,
		BaseSpeed,
	}
}

func GetAttributesForModifiersByLevel(level int) []Name {
	/*
	   BaseSpeed:            10, // we now would need an attack speed and a movement speed modifier
	              AttackSpeedFactor:    1,
	              MoveSpeedFactor:      1,
	              EnhancedDamageFactor: 1,

	              MaxHealth: 10,
	              MaxMana:   10,
	*/
	switch {
	case level < 3:
		return []Name{Strength, Intelligence, Dexterity}
	case level >= 3 && level < 5:
		return []Name{Strength, Intelligence, Dexterity, MaxHealth, MaxMana}
	case level >= 5 && level < 7:
		return []Name{Strength, Intelligence, Dexterity, MaxHealth, MaxMana, AttackSpeedFactor, MoveSpeedFactor}
	default:
		return []Name{Strength, Intelligence, Dexterity, MaxHealth, MaxMana, AttackSpeedFactor, MoveSpeedFactor, EnhancedDamageFactor}
	}
}

type ModificationMode bool

const (
	ModifyAdditive       ModificationMode = false
	ModifyMultiplicative ModificationMode = true
)

type Modifier struct {
	Attribute      Name
	Name           string
	Value          float64
	Mode           ModificationMode
	IsPercentBased bool
}

func (m Modifier) ToString() string {
	if m.Mode == ModifyAdditive {
		sign := "+"
		if m.Value < 0 {
			sign = "-"
		}
		if m.IsPercentBased {
			intValue := int(math.Ceil(m.Value * 100))
			return fmt.Sprintf("%s%d%% %s", sign, intValue, m.Attribute)
		}
		intValue := int(m.Value)
		return fmt.Sprintf("%s%d %s", sign, intValue, m.Attribute)
	}
	// m.Mode == ModifyMultiplicative
	if m.Value < 1 {
		minusInPercent := int(math.Ceil((1.0 - m.Value) * 100))
		return fmt.Sprintf("-%d%% %s", minusInPercent, m.Attribute)
	}
	plusPercent := int(math.Ceil((m.Value - 1) * 100))
	return fmt.Sprintf("+%d%% %s", plusPercent, m.Attribute)
}

type AttributeHolder struct {
	attributes map[Name]float64
	resources  map[Name]*Resource
	modifiers  map[Name][]*Modifier
}

func NewAttributeHolder() AttributeHolder {
	holder := AttributeHolder{
		attributes: map[Name]float64{
			Level:        1,
			Experience:   0,
			Strength:     10,
			Intelligence: 10,
			Dexterity:    10,

			Gold: 0,

			BaseSpeed:                10, // we now would need an attack speed and a movement speed modifier
			BaseArmorClass: 		  0,
			BaseDamage:               0,

			AttackSpeedFactor:        1,
			MoveSpeedFactor:          1,
			EnhancedDamageFactor:     1,
			EnhancedArmorClassFactor: 1,

			LifeStealFactor: 0,
			ManaStealFactor: 0,

			HealthRegeneration: 0.0, //0.01,
			ManaRegeneration:   0.0, //0.01,

			MaxHealth: 10,
			MaxMana:   10,

			VisionRange:      20,
			LightRadius: 	  0,
			ArmorPenetration: 0,
		},

		modifiers: make(map[Name][]*Modifier),
	}

	resources := map[Name]*Resource{
		Health: NewResource(-10, holder.getMaxForRessource(MaxHealth), 10),
		Mana:   NewResource(0, holder.getMaxForRessource(MaxMana), 0),
	}
	holder.resources = resources
	return holder
}
func (a *AttributeHolder) getMaxForRessource(attr Name) func() int {
	return func() int {
		return a.GetAttribute(attr)
	}
}
func (a *AttributeHolder) GetAttributeBaseValue(name Name) float64 {
	return a.attributes[name]
}
func (a *AttributeHolder) GetAttribute(attr Name) int {
	return int(a.getModifiedValue(attr, a.GetAttributeBaseValue(attr)))
}

type ModifierImpact int

func (i ModifierImpact) ToColor() color.Color {
	switch i {
	case ModifierImpactPositive:
		return color.RGBA{0x00, 0xff, 0x00, 0xff}
	case ModifierImpactNegative:
		return color.RGBA{0xff, 0x00, 0x00, 0xff}
	default:
		return color.RGBA{0xff, 0xff, 0xff, 0xff}
	}
}

const (
	ModifierImpactNone ModifierImpact = iota
	ModifierImpactPositive
	ModifierImpactNegative
)

func (a *AttributeHolder) GetModifierType(attr Name) ModifierImpact {
	unmodified := a.GetAttributeBaseValue(attr)
	modified := a.getModifiedValue(attr, unmodified)
	if modified > unmodified {
		return ModifierImpactPositive
	}
	if modified < unmodified {
		return ModifierImpactNegative
	}
	return ModifierImpactNone
}
func (a *AttributeHolder) GetAttributeAsFloat(attr Name) float64 {
	return a.getModifiedValue(attr, a.GetAttributeBaseValue(attr))
}

func (a *AttributeHolder) GetRessource(name Name) *Resource {
	return a.resources[name]
}

func (a *AttributeHolder) getModifiedValue(name Name, baseValue float64) float64 {
	modifierFactor := float64(1)
	modifierSummand := float64(0)

	allModifiers := a.modifiers[name]
	for i := len(allModifiers) - 1; i >= 0; i-- {
		mod := allModifiers[i]
		if mod.Mode == ModifyAdditive {
			modifierSummand += mod.Value
		} else {
			modifierFactor *= mod.Value
		}
	}

	return (baseValue * modifierFactor) + modifierSummand
}

func (a *AttributeHolder) GetAllModifiersForAttribute(name Name) []*Modifier {
	return a.modifiers[name]
}

func (a *AttributeHolder) AddModifier(modifier *Modifier) {
	attr := modifier.Attribute
	a.modifiers[attr] = append(a.modifiers[attr], modifier)
}
func (a *AttributeHolder) RemoveModifier(modifier *Modifier) {
	attr := modifier.Attribute
	for i, m := range a.modifiers[attr] {
		if m == modifier {
			a.modifiers[attr] = append(a.modifiers[attr][:i], a.modifiers[attr][i+1:]...)
			return
		}
	}
}
func (a *AttributeHolder) Increment(attributeName Name) {
	a.attributes[attributeName]++
}

func (a *AttributeHolder) Decrement(name Name) {
	a.attributes[name]--
}

func (a *AttributeHolder) SetAttribute(attributeName Name, value int) {
	if _, ok := a.attributes[attributeName]; !ok {
		println("attribute not found", attributeName)
		return
	}
	a.attributes[attributeName] = float64(value)
}

func (a *AttributeHolder) IncrementBy(attributeName Name, amount int) {
	a.attributes[attributeName] += float64(amount)
}

func (a *AttributeHolder) IncrementRessourceBy(name Name, amount int) {
	a.resources[name].Increase(amount)
}

func (a *AttributeHolder) DecrementBy(name Name, amount int) {
	a.attributes[name] -= float64(amount)
}
