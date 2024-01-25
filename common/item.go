package common

import (
	"cmp"
	"fmt"
	"image/color"
	"rpg/geometry"
	"rpg/stats"
	"rpg/util"
	"slices"
)

type ItemRarity int

const (
	ItemRarityCommon ItemRarity = iota
	ItemRarityMagic
	ItemRarityUnique
)

type ItemCategory int

const (
	ItemCategoryPotion ItemCategory = iota
	ItemCategoryScroll
	ItemCategoryWand
	ItemCategoryWeapon
	ItemCategoryArmor
	ItemCategoryLightSource
	ItemCategoryOther
)

type ArmorType int

func (t ArmorType) ToString() string {
	switch t {
	case ArmorTypeHelmet:
		return "Helmet"
	case ArmorTypeCuirass:
		return "Cuirass"
	case ArmorTypeGloves:
		return "Gloves"
	case ArmorTypeBoots:
		return "Boots"
	case ArmorTypeCape:
		return "Cape"
	case ArmorTypeAmulet:
		return "Amulet"
	case ArmorTypeRing:
		return "Ring"
	case ArmorTypeShield:
		return "Shield"
	}
	return "Unknown"
}

const (
	ArmorTypeHelmet ArmorType = iota
	ArmorTypeCuirass
	ArmorTypeBoots
	ArmorTypeGloves
	ArmorTypeCape
	ArmorTypeAmulet
	ArmorTypeRing
	ArmorTypeShield
)

type ArmorStats struct {
	BaseArmor  int
	Type       ArmorType
	Durability *stats.Resource
}

func (s ArmorStats) ToString(player *Actor) []util.LabelText {
	return []util.LabelText{
		util.LabelText{Text: fmt.Sprintf("Armor Type: %s", s.Type.ToString())},
		util.LabelText{Text: fmt.Sprintf("Armor Class: %d", s.BaseArmor)},
		util.LabelText{Text: fmt.Sprintf("Durability: %s", s.Durability.ToString())},
	}
}
type LightStats struct {
	LightColor        color.RGBA
	FlickerPattern    []float64
	FlickerFrameDelay float64
}

type Item interface {
	Name() string
	MapPosition() geometry.Point
	SetMapPosition(geometry.Point)

	Icon(tick uint64) int32
	InventoryIcon() int32
	TintColor() color.Color

	GetContextActions(worldState State) []util.MenuItem

	GetWeaponStats() *WeaponStats
	GetArmorStats() *ArmorStats
	GetLightStats() *LightStats

	IsWeapon() bool
	IsArmor() bool
	IsMeleeWeapon() bool
	IsRangedWeapon() bool

	CanStackWith(other Item) bool
	SlotName() SlotName

	GetTooltipLines() []util.LabelText

	BuyPrice() int
	GetEquippedModifiers() []*stats.Modifier
	Counter() int
	GetItemType() ItemCategory
	DroppedByPlayer()
	IsDroppedByPlayer() bool
	IsHidden() bool
	Show()

	IsModifyingLight() bool
	IsLightSource() bool
}

type BaseItem struct {
	name string

	pos  geometry.Point
	icon int32

	weaponComponent   *WeaponStats
	armorComponent    *ArmorStats
	lightComponent    *LightStats

	slot              SlotName
	buyPrice          int
	qualityLevel      int
	equippedModifiers []*stats.Modifier
	contextActions    []util.MenuItem
	getPlayer         func() *Actor
	category          ItemCategory
	rarity            ItemRarity

	droppedByPlayer bool
	isHidden        bool
}

func NewBaseItem(getPlayerRef func() *Actor, name string, icon int32, slot SlotName, buyPrice int) *BaseItem {
	return &BaseItem{
		getPlayer: getPlayerRef,
		name:      name,
		icon:      icon,
		slot:      slot,
		buyPrice:  buyPrice,
	}
}
func (i *BaseItem) IsModifyingLight() bool {
	for _, modifier := range i.equippedModifiers {
		if modifier.Attribute == stats.LightRadius {
			return true
		}
	}
	return false
}

func (i *BaseItem) SetIcon(icon int32) {
	i.icon = icon
}
func (i *BaseItem) SetName(name string) {
	i.name = name
}

func (i *BaseItem) Hide() {
	i.isHidden = true
}
func (i *BaseItem) Show() {
	i.isHidden = false
}
func (i *BaseItem) IsHidden() bool {
	return i.isHidden
}
func (i *BaseItem) DroppedByPlayer() {
	i.droppedByPlayer = true
}

func (i *BaseItem) IsDroppedByPlayer() bool {
	return i.droppedByPlayer
}
func (i *BaseItem) GetRarity() ItemRarity {
	return i.rarity
}
func (i *BaseItem) SetRarity(rarity ItemRarity) {
	i.rarity = rarity
}
func (i *BaseItem) GetItemType() ItemCategory {
	return i.category
}
func (i *BaseItem) GetEquippedModifiers() []*stats.Modifier {
	return i.equippedModifiers
}
func (i *BaseItem) Counter() int {
	return -1
}
func (i *BaseItem) BuyPrice() int {
	return i.buyPrice
}
func (i *BaseItem) InventoryIcon() int32 {
	return i.icon
}

func (i *BaseItem) GetLightStats() *LightStats {
	return i.lightComponent
}

func (i *BaseItem) TintColor() color.Color {
	return color.White
}

func (i *BaseItem) Name() string {
	return i.name
}

func (i *BaseItem) IsWeapon() bool {
	return i.weaponComponent != nil
}

func (i *BaseItem) IsArmor() bool {
	return i.armorComponent != nil
}

func (i *BaseItem) IsMeleeWeapon() bool {
	return i.IsWeapon() && i.weaponComponent.IsMeleeWeapon()
}

func (i *BaseItem) IsRangedWeapon() bool {
	return i.IsWeapon() && i.weaponComponent.IsRangedWeapon()
}
func (i *BaseItem) CanStackWith(other Item) bool {
	// only non-equippable items can stack
	if i.SlotName() != SlotNameNotEquippable || other.SlotName() != SlotNameNotEquippable {
		return false
	}

	if i.GetItemType() != other.GetItemType() {
		return false
	}

	if i.Name() != other.Name() {
		return false
	}

	if i.InventoryIcon() != other.InventoryIcon() {
		return false
	}

	return true
}

func (i *BaseItem) SlotName() SlotName {
	return i.slot
}

func (i *BaseItem) MapPosition() geometry.Point {
	return i.pos
}

func (i *BaseItem) Icon(tick uint64) int32 {
	return i.icon
}

func (i *BaseItem) SetMapPosition(pos geometry.Point) {
	i.pos = pos
}

func (i *BaseItem) GetContextActions(worldState State) []util.MenuItem {
	return i.contextActions
}

func (i *BaseItem) GetWeaponStats() *WeaponStats {
	return i.weaponComponent
}

func (i *BaseItem) GetArmorStats() *ArmorStats {
	return i.armorComponent
}

func (i *BaseItem) SetQualityLevel(level int) {
	i.qualityLevel = level
}

func (i *BaseItem) SetWeaponComponent(component *WeaponStats) {
	i.weaponComponent = component
}

func (i *BaseItem) SetEquippedModifiers(modifiers []*stats.Modifier) {
	i.equippedModifiers = modifiers
	i.sortModifiers()
}
func (i *BaseItem) sortModifiers() {
	slices.SortStableFunc(i.equippedModifiers, func(i, j *stats.Modifier) int {
		return cmp.Compare(i.Attribute, j.Attribute)
	})
}

func (i *BaseItem) RemoveEquippedModifier(modifier *stats.Modifier) {
	for index, m := range i.equippedModifiers {
		if m == modifier {
			i.equippedModifiers = append(i.equippedModifiers[:index], i.equippedModifiers[index+1:]...)
			return
		}
	}
}

func (i *BaseItem) AddEquippedModifier(modifier *stats.Modifier) {
	i.equippedModifiers = append(i.equippedModifiers, modifier)
	i.sortModifiers()
}
func (i *BaseItem) SetLightComponent(component *LightStats) {
	i.lightComponent = component
}
func (i *BaseItem) SetArmorComponent(component *ArmorStats) {
	i.armorComponent = component
}

func (i *BaseItem) SetContextActions(actions []util.MenuItem) {
	i.contextActions = actions
}

func (i *BaseItem) GetWeaponStatsAsString() []util.LabelText {
	return i.GetWeaponStats().ToString(i.getPlayer())
}
func (i *BaseItem) GetArmorStatsAsString() []util.LabelText {
	return i.GetArmorStats().ToString(i.getPlayer())
}

func (i *BaseItem) SetCategory(category ItemCategory) {
	i.category = category
}

func (i *BaseItem) IsLightSource() bool {
	return i.lightComponent != nil
}