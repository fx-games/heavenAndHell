package game

import (
    "io"
    "math/rand"
    "rpg/actions"
    "rpg/common"
    "rpg/geometry"
    "rpg/recfile"
    "rpg/stats"
)

//

// item line for typical armor
// name, icon, dlvl, armor_type, armor_class

// item line for typical weapon
// name, icon, dlvl, weapon_type, weapon_damage, weapon_speed
type ItemDefinition struct {
    Name              string
    Icon              int32
    DLvl              int // min dungeon level at which this item can appear (=native depth)
    MaxLevel          int // max level at which this item can appear

    OutOfDepthChance  float64 // chance to appear out of depth
    MinLevelForOoD int

    InventoryCategory common.ItemCategory
    Armor             ArmorDefinition
    Weapon            WeaponDefinition
    Throw             ThrowDefinition
}

func (d ItemDefinition) CanAppearAt(randomSource *rand.Rand, dungeonLevel int) bool {
    // reject early
    if d.MaxLevel > 0 && dungeonLevel > d.MaxLevel {
        return false
    }
    regularAppearance := dungeonLevel >= d.DLvl

    // apply out of depth chance
    oodAppearance :=  dungeonLevel >= d.MinLevelForOoD && randomSource.Float64() < d.OutOfDepthChance

    return regularAppearance || oodAppearance
}
type ArmorDefinition struct {
    ArmorType  common.ArmorType
    ArmorClass int
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
    return d.HitZone != ""
}
func ReadItemDefinitions(file io.Reader) []ItemDefinition {
    records := recfile.Read(file)
    itemDefinitions := make([]ItemDefinition, len(records))
    for i, record := range records {
        itemDefinitions[i] = NewItemDefinitionFromRecord(record)
    }
    return itemDefinitions
}

func NewItemDefinitionFromRecord(record recfile.Record) ItemDefinition {
    itemDefinition := ItemDefinition{}
    for _, field := range record {
        switch field.Name {
        case "name":
            itemDefinition.Name = field.Value
        case "icon":
            itemDefinition.Icon = field.AsInt32()
        case "dlvl":
            itemDefinition.DLvl = field.AsInt()
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
            itemDefinition.Armor.ArmorClass = field.AsInt()
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
        }
    }

    return itemDefinition
}

func (g *GameInstance) NewItemFromDefinition(itemDefinition ItemDefinition) *GameItem {
    baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, itemDefinition.Name, itemDefinition.Icon)
    baseItem.SetCategory(itemDefinition.InventoryCategory)
    baseItem.SetQualityLevel(itemDefinition.DLvl)

    if itemDefinition.Armor.IsValid() {
        armorPart := &common.ArmorStats{
            BaseArmor:  itemDefinition.Armor.ArmorClass,
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
        throwPart := &common.ThrowStats{
            Damage:        itemDefinition.Throw.Damage,
            HitZone:       NewHitZoneFromString(g.GetWorldState(), itemDefinition.Throw.HitZone),
            ChanceToBreak: itemDefinition.Throw.ChanceToBreak,
            OnHitEffect:   g.NewOnHitEffectFromString(itemDefinition.Throw.OnHitEffect),
        }
        baseItem.SetThrowComponent(throwPart)
    }

    return &GameItem{
        BaseItem: baseItem,
    }
}

// Throwable on hit effects come from here
func (g *GameInstance) NewOnHitEffectFromString(effect string) func(attacker *common.Actor, hitLocation geometry.Point) {
    switch effect {
    case "confuse":
        return g.confuseOnHitProc
    }
    return nil
}

// These determine aoe zones for spells and throwable items
func NewHitZoneFromString(state actions.WorldState, zone string) common.AoEResolver {
    switch zone {
    case "target_only":
        return OneTargetTile
    case "dijkstra_aoe": // TODO: parse dijkstra_aoe(2) later on..
        return DijkstraAoE(state.GetMap(), 2)
    }
    return nil
}