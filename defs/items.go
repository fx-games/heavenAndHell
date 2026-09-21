package defs

import (
	"fmt"
	"io/fs"
	"math/rand"
	"path"
	"strings"

	"rpg/recfile"
	"rpg/stats"
	"rpg/util"
)

// Item is the unified item record every mod supplies in assets/data/items.rec, the item half of what
// defs.Monster is for monsters: the union of the two games' columns, read by one loader, with no per-mod
// branch between here and the file.
//
// Where the two schemas said the same thing in different words they were merged rather than doubled up:
//
//	weapon_damage   D1's iMinDam..iMaxDam and H&H's XdY, both as util.Dice. MinDam/MaxDam are the D1 view.
//	armor_class     D1's iMinAC..iMaxAC and H&H's interval are the same stats.Interval. MinAC/MaxAC likewise.
//	cost            D1's iValue. Cost and Value are the one number under both games' names.
//	dlvl            D1's iMinMLvl.
//	icon            an icon ID the mod defines, never a raw sheet number (see LoadIcons).
//
// The rest are one game's and left zero by the other, which is how a mod says "my game does not have this".
type Item struct {
	Name         string
	InternalName string
	ShortName    string // D1's iSName, the name that fits the belt
	Icon         string // icon ID in the mod's icons/items.rec, e.g. "item.falchion"

	DLvl             int // lowest dungeon level it can appear on
	MaxLevel         int
	OutOfDepthChance float64
	MinLevelForOoD   int

	InventoryCategory ItemCategory
	Armor             ArmorDefinition
	Weapon            WeaponDefinition
	Throw             ThrowDefinition
	Light             LightDefinition
	Cost              int
	Charges           int
	Modifiers         []string
	UntargetedAction  string
	TargetedAction    string

	// Diablo's own columns. Its generation walks this table by index and reads these directly.
	Drop, Class, Loc       string // IDROP_, ICLASS_, ILOC_
	Curs, Type             int    // cursor graphic, ITYPE_ value
	UniqueBase             string // UITYPE_ name uniques refer to
	MinMLvl, Durability    int
	MinDam, MaxDam         int
	MinAC, MaxAC           int
	MinStr, MinMag, MinDex int
	Flags, MiscID, Spell   string
	Value                  int
}

func (d Item) IsArmor() bool       { return d.Armor.IsValid() }
func (d Item) IsWeapon() bool      { return d.Weapon.IsValid() }
func (d Item) IsThrow() bool       { return d.Throw.IsValid() }
func (d Item) IsLightSource() bool { return d.Light.IsValid() }
func (d Item) IsPotion() bool      { return d.InventoryCategory == ItemCategoryPotion }
func (d Item) IsScroll() bool      { return d.InventoryCategory == ItemCategoryScroll }
func (d Item) IsRing() bool        { return d.InventoryCategory == ItemCategoryRing }
func (d Item) IsAmulet() bool      { return d.InventoryCategory == ItemCategoryAmulet }

// CanAppearAt is the depth roll: native depth, or a lucky roll above it.
func (d Item) CanAppearAt(randomSource *rand.Rand, dungeonLevel int) bool {
	if d.MaxLevel > 0 && dungeonLevel > d.MaxLevel {
		return false
	}
	regular := dungeonLevel >= d.DLvl
	ood := dungeonLevel >= d.MinLevelForOoD && randomSource.Float64() < d.OutOfDepthChance
	return regular || ood
}

// ReadItems loads the active mod's items, in file order -- order is content: D1 rolls items by walking this
// table, so a row moving is a gameplay change.
func ReadItems(fsys fs.FS) ([]Item, error) {
	f, err := fsys.Open(path.Join("assets", "data", "items.rec"))
	if err != nil {
		return nil, fmt.Errorf("items.rec: %w", err)
	}
	defer f.Close()
	var out []Item
	for i, rec := range recfile.Read(f) {
		it, err := itemFromRecord(rec)
		if err != nil {
			return nil, fmt.Errorf("items.rec record %d: %w", i+1, err)
		}
		fillEquipModel(&it)
		out = append(out, it)
	}
	return out, nil
}

// fillEquipModel gives a record that describes itself by category, weapon_type and armor_type (H&H's way) the
// item class, type and location the engine's equipment and hero sheet work with (D1's way). A record that set
// its own item_type, class or location keeps them.
func fillEquipModel(it *Item) {
	if it.Type != 0 || it.Class != "" || it.Loc != "" {
		return
	}
	hand := "ILOC_ONEHAND"
	if it.Weapon.IsTwoHanded {
		hand = "ILOC_TWOHAND"
	}
	switch it.InventoryCategory {
	case ItemCategoryWeapon:
		it.Class, it.Loc = "ICLASS_WEAPON", hand
		switch it.Weapon.WeaponType {
		case WeaponTypeAxe:
			it.Type = 2
		case WeaponTypeBow, WeaponTypeCrossbow, WeaponTypeSling:
			it.Type, it.Loc = 3, "ILOC_TWOHAND"
		case WeaponTypeMace:
			it.Type = 4
		case WeaponTypeSpear, WeaponTypeScythe:
			it.Type = 10
		default: // sword, dagger, whip
			it.Type = 1
		}
	case ItemCategoryArmor:
		it.Class = "ICLASS_ARMOR"
		switch it.Armor.ArmorType {
		case ArmorTypeHelmet:
			it.Type, it.Loc = 7, "ILOC_HELM"
		case ArmorTypeShield, ArmorTypeDefensiveWeapon:
			it.Type, it.Loc = 5, "ILOC_ONEHAND"
		case ArmorTypeBoots:
			it.Type, it.Loc = 6, "ILOC_FEET"
		case ArmorTypeGloves:
			it.Type, it.Loc = 6, "ILOC_HANDS"
		case ArmorTypeCape:
			it.Type, it.Loc = 6, "ILOC_BACK"
		default:
			it.Type, it.Loc = 6, "ILOC_ARMOR"
		}
	case ItemCategoryRing:
		it.Class, it.Type, it.Loc = "ICLASS_MISC", 12, "ILOC_RING"
	case ItemCategoryAmulet:
		it.Class, it.Type, it.Loc = "ICLASS_MISC", 13, "ILOC_AMULET"
	case ItemCategoryLightSource:
		it.Class, it.Loc = "ICLASS_MISC", "ILOC_LIGHT"
	}
}

func itemFromRecord(rec recfile.Record) (Item, error) {
	it := Item{InventoryCategory: -1, Weapon: WeaponDefinition{ProjectileIcon: -1}}
	for _, f := range rec {
		switch f.Name {
		case "name":
			it.Name = f.Value
		case "internal_name":
			it.InternalName = f.Value
		case "short_name":
			it.ShortName = f.Value
		case "icon":
			it.Icon = f.Value
		case "dlvl":
			it.DLvl, it.MinMLvl = f.AsInt(), f.AsInt()
		case "max_level":
			it.MaxLevel = f.AsInt()
		case "out_of_depth_chance":
			it.OutOfDepthChance = f.AsFloat()
		case "min_level_for_ood":
			it.MinLevelForOoD = f.AsInt()
		case "cost":
			it.Cost, it.Value = f.AsInt(), f.AsInt()
		case "charges":
			it.Charges = f.AsInt()
		case "category":
			it.InventoryCategory = ItemCategoryFromString(f.Value)
		case "armor_type":
			it.Armor.ArmorType = ArmorTypeFromString(f.Value)
		case "armor_class":
			it.Armor.ArmorClass = stats.NewIntervalFromString(f.Value)
			it.MinAC, it.MaxAC = it.Armor.ArmorClass.Min(), it.Armor.ArmorClass.Max()
		case "weapon_type":
			it.Weapon.WeaponType = WeaponTypeFromString(f.Value)
		case "weapon_two_handed":
			it.Weapon.IsTwoHanded = f.AsBool()
		case "weapon_damage":
			it.Weapon.Damage = util.ParseDice(f.Value)
			it.MinDam, it.MaxDam = it.Weapon.Damage.Min(), it.Weapon.Damage.Max()
		case "weapon_speed":
			it.Weapon.Speed = f.AsFloat()
		case "weapon_armor_penetration":
			it.Weapon.ArmorPenetration = f.AsInt()
		case "ranged_max_range":
			it.Weapon.MaxRange = f.AsInt()
		case "ranged_accuracy_at_max_range":
			it.Weapon.AccuracyAtMaxRange = f.AsFloat()
		case "ranged_accuracy_at_min_range":
			it.Weapon.AccuracyAtMinRange = f.AsFloat()
		case "ranged_projectile_icon":
			it.Weapon.ProjectileIcon = f.AsInt32()
		case "throw_hit_zone":
			it.Throw.HitZone = f.Value
		case "throw_damage":
			it.Throw.Damage = stats.NewIntervalFromString(f.Value)
		case "throw_chance_to_break":
			it.Throw.ChanceToBreak = f.AsFloat()
		case "throw_on_hit_effect":
			it.Throw.OnHitEffect = f.Value
		case "light_color":
			it.Light.LightColor = f.Value
		case "light_flicker_pattern":
			it.Light.FlickerPattern = f.Value
		case "light_flicker_frame_delay":
			it.Light.FlickerFrameDelay = f.AsFloat()
		case "equip_modifier":
			it.Modifiers = append(it.Modifiers, f.Value)
		case "untargeted_action":
			it.UntargetedAction = f.Value
		case "targeted_action":
			it.TargetedAction = f.Value
		case "drop":
			it.Drop = f.Value
		case "class":
			it.Class = f.Value
		case "location":
			it.Loc = f.Value
		case "cursor":
			it.Curs = f.AsInt()
		case "item_type":
			it.Type = f.AsInt()
		case "unique_base":
			it.UniqueBase = f.Value
		case "durability":
			it.Durability = f.AsInt()
		case "requires_strength":
			it.MinStr = f.AsInt()
		case "requires_magic":
			it.MinMag = f.AsInt()
		case "requires_dexterity":
			it.MinDex = f.AsInt()
		case "flags":
			it.Flags = f.Value
		case "misc_id":
			it.MiscID = f.Value
		case "spell":
			it.Spell = f.Value
		}
	}
	if it.InternalName == "" {
		it.InternalName = strings.ToLower(strings.ReplaceAll(it.Name, " ", "_"))
	}
	if it.Name == "" {
		return it, fmt.Errorf("no name")
	}
	return it, nil
}
