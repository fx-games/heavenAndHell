package common

import (
	"cmp"
	"fmt"
	"image/color"
	"rpg/geometry"
	"rpg/stats"
	"rpg/util"
	"slices"
	"strings"
)

type ItemRarity int

const (
	ItemRarityCommon ItemRarity = iota
	ItemRarityMagic
	ItemRarityUnique
)

type ItemCategory int

func (c ItemCategory) HasAlwaysOneCharge() bool {
	switch c {
	case ItemCategoryPotion:
		return true
	case ItemCategoryScroll:
		return true
	}
	return false
}

func (c ItemCategory) IsValid() bool {
	if c == ItemCategoryInvalid {
		return false
	}
	switch c {
	case ItemCategoryPotion:
		fallthrough
	case ItemCategoryScroll:
		fallthrough
	case ItemCategoryWand:
		fallthrough
	case ItemCategoryLightSource:
		fallthrough
	case ItemCategoryWeapon:
		fallthrough
	case ItemCategoryArmor:
		fallthrough
	case ItemCategoryAccessory:
		fallthrough
	case ItemCategoryThrowable:
		fallthrough
	case ItemCategoryOther:
		return true
	default:
		return false
	}
}

const ( // defines the sort order of the inventory
	ItemCategoryInvalid  ItemCategory = iota -1
	ItemCategoryPotion
	ItemCategoryScroll
	ItemCategoryWand
	ItemCategoryLightSource
	ItemCategoryWeapon
	ItemCategoryArmor
	ItemCategoryAccessory
	ItemCategoryThrowable
	ItemCategoryOther
)

func ItemCategoryFromString(str string) ItemCategory {
	str = strings.ToLower(str)
	switch str {
	case "potion":
		return ItemCategoryPotion
	case "scroll":
		return ItemCategoryScroll
	case "wand":
		return ItemCategoryWand
	case "light":
		return ItemCategoryLightSource
	case "weapon":
		return ItemCategoryWeapon
	case "armor":
		return ItemCategoryArmor
	case "accessory":
		return ItemCategoryAccessory
	case "throwable":
		return ItemCategoryThrowable
	}

	return -1
}

type ArmorType int

func ArmorTypeFromString(str string) ArmorType {
	str = strings.ToLower(str)
	switch str {
	case "helmet":
		return ArmorTypeHelmet
	case "cuirass":
		return ArmorTypeCuirass
	case "gloves":
		return ArmorTypeGloves
	case "boots":
		return ArmorTypeBoots
	case "cape":
		return ArmorTypeCape
	case "amulet":
		return ArmorTypeAmulet
	case "ring":
		return ArmorTypeRing
	case "shield":
		return ArmorTypeShield
	}
	return ArmorTypeNone
}

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

func (t ArmorType) GetInventorySlot() SlotName {
	switch t {
	case ArmorTypeHelmet:
		return SlotNameArmorHead
	case ArmorTypeCuirass:
		return SlotNameArmorTorso
	case ArmorTypeGloves:
		return SlotNameArmorHands
	case ArmorTypeBoots:
		return SlotNameArmorFeet
	case ArmorTypeCape:
		return SlotNameArmorBack
	case ArmorTypeAmulet:
		return SlotNameAmulet
	case ArmorTypeRing:
		return SlotNameRing
	case ArmorTypeShield:
		return SlotNameOffHand
	case ArmorTypeNone:
		return SlotNameNotEquippable
	}
	return SlotNameNotEquippable
}

const (
	ArmorTypeNone ArmorType = iota
	ArmorTypeHelmet
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
	Plus       int
}

func (s ArmorStats) ToString(player *Actor) []util.LabelText {
	return []util.LabelText{
		util.LabelText{Text: fmt.Sprintf("Armor Type: %s", s.Type.ToString())},
		util.LabelText{Text: fmt.Sprintf("Armor Class: %d", s.GetArmorClass())},
		util.LabelText{Text: fmt.Sprintf("Enchantment: +%d", s.Plus)},
		util.LabelText{Text: fmt.Sprintf("Durability: %s", s.Durability.ToString())},
	}
}

func (s ArmorStats) NameParts() string {
	// angband style [3, +3]
	return fmt.Sprintf("[%d, +%d]", s.BaseArmor, s.Plus)
}

func (s ArmorStats) GetArmorClass() int {
	return s.BaseArmor + s.Plus
}

func (s ArmorStats) Clone() *ArmorStats {
	return &ArmorStats{
		BaseArmor:  s.BaseArmor,
		Type:       s.Type,
		Durability: stats.NewResource(s.Durability.Min(), s.Durability.GetMax, s.Durability.Value()),
		Plus:       s.Plus,
	}
}
type LightStats struct {
	LightColor        color.RGBA
	FlickerPattern    []float64
	FlickerFrameDelay float64
}

func (s LightStats) Clone() *LightStats {
	return &LightStats{
		LightColor:        s.LightColor,
		FlickerPattern:    s.FlickerPattern,
		FlickerFrameDelay: s.FlickerFrameDelay,
	}
}


type Item interface {
	Name() string
	BandStyleName() string
	MapPosition() geometry.Point
	SetMapPosition(geometry.Point)

	Icon(tick uint64) int32
	InventoryIcon() int32
	TintColor() color.Color

	GetWeaponStats() *WeaponStats
	GetArmorStats() *ArmorStats
	GetLightStats() *LightStats
	GetThrowStats() *ThrowStats

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
	String() string
	IsModifyingLight() bool
	IsLightSource() bool
	PlusName() string
	IsThrowable() bool
	SetDrawnToScreen(shouldBeDrawn bool)
	IsDrawn() bool
	Clone() Item
}


type BaseItem struct {
	name string

	pos  geometry.Point
	icon int32

	weaponComponent *WeaponStats
	armorComponent  *ArmorStats
	lightComponent  *LightStats
	throwComponent  *ThrowStats

	qualityLevel      int
	equippedModifiers []*stats.Modifier
	getPlayer         func() *Actor
	category          ItemCategory
	rarity            ItemRarity

	droppedByPlayer bool
	isHidden        bool
	shouldBeDrawn   bool
	baseCost        int
}

func NewBaseItem(getPlayerRef func() *Actor, name string, icon int32) *BaseItem {
	return &BaseItem{
		getPlayer: getPlayerRef,
		name:      name,
		icon:      icon,
		shouldBeDrawn: true,
	}
}

func (i *BaseItem) SetDrawnToScreen(shouldBeDrawn bool) {
	i.shouldBeDrawn = shouldBeDrawn
}

func (i *BaseItem) IsDrawn() bool {
	return i.shouldBeDrawn
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
	return i.qualityLevel * 200 // TODO: calculate price based on item stats
}
func (i *BaseItem) InventoryIcon() int32 {
	return i.icon
}

func (i *BaseItem) GetLightStats() *LightStats {
	return i.lightComponent
}

func (i *BaseItem) TintColor() color.Color {
	commonColor := White
	if i.IsLightSource() {
		commonColor = Yellow
	}

	switch i.rarity {
	case ItemRarityCommon:
		return commonColor
	case ItemRarityMagic:
		return MagicHighlightColor
	case ItemRarityUnique:
		return UniqueHighlightColor
	}

	return color.White
}

func (i *BaseItem) Name() string {
	name := i.name
	return name
}
func (i *BaseItem) String() string {
	return i.Name()
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
	if i.SlotName() != other.SlotName() {
		return false
	}

	if i.SlotName() != SlotNameNotEquippable && i.SlotName() != SlotNameQuiver {
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
	if i.IsLightSource() {
		return SlotNameLightSource
	}
	if i.IsRangedWeapon() {
		return SlotNameMissileLauncher
	}
	if i.IsArmor() {
		return i.armorComponent.Type.GetInventorySlot()
	}
	if i.IsWeapon() {
		return i.weaponComponent.Type.GetInventorySlot()
	}
	if i.IsThrowable() {
		return SlotNameQuiver
	}
	return SlotNameNotEquippable
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


func (i *BaseItem) GetThrowStats() *ThrowStats {
	return i.throwComponent
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

func (i *BaseItem) SetThrowComponent(throwComponent *ThrowStats) {
	i.throwComponent = throwComponent
}

func (i *BaseItem) IsThrowable() bool {
	return i.throwComponent != nil
}

func (i *BaseItem) SetBaseCost(cost int) {
	i.baseCost = cost
}

func (i *BaseItem) Clone() *BaseItem {
	clone := &BaseItem{
		name:              i.name,
		pos:               i.pos,
		icon:              i.icon,
		weaponComponent:   nil,
		armorComponent:    nil,
		lightComponent:    nil,
		throwComponent:    nil,
		qualityLevel:      i.qualityLevel,
		equippedModifiers: i.equippedModifiers,
		getPlayer:         i.getPlayer,
		category:          i.category,
		rarity:            i.rarity,
		droppedByPlayer:   false,
		isHidden:          i.isHidden,
		shouldBeDrawn:     i.shouldBeDrawn,
		baseCost:          i.baseCost,
	}
	if i.weaponComponent != nil {
		clone.weaponComponent = i.weaponComponent.Clone()
	}
	if i.armorComponent != nil {
		clone.armorComponent = i.armorComponent.Clone()
	}
	if i.lightComponent != nil {
		clone.lightComponent = i.lightComponent.Clone()
	}
	if i.throwComponent != nil {
		clone.throwComponent = i.throwComponent.Clone()
	}
	return clone
}