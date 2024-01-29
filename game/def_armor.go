package game

import (
	"math/rand"
	"os"
	"path"
	"rpg/actions"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
)

/*

Starting Items
Dmg.: 2-6
AC: 1-5

Mid Game Items
Dmg.: 6-12
AC: 6-23

End Game Items
Dmg.: 12-30
AC: 24-100


Armor Rating	Damage Reduction in %
1	0.01
5	0.05
25	0.20
50	0.33
100	0.50
200	0.67
1000	0.91
*/
func (g *GameInstance) NewRandomArmorPiece(dungeonLevel int) *GameItem {
	// hacky read file here..
	file, _ := os.Open(path.Join("assets", "data", "armor.rec"))
	g.armorDefinitions = ReadItemDefinitions(file)

	armorDefinitions := g.armorDefinitions

	// filter for dungeon level
	var filteredArmorDefinitions []ItemDefinition
	armorTypeMap := make(map[common.ArmorType][]ItemDefinition)
	for _, armorDefinition := range armorDefinitions {

		if !armorDefinition.CanAppearAt(g.randomSource, dungeonLevel) {
			continue
		}
		
		filteredArmorDefinitions = append(filteredArmorDefinitions, armorDefinition)
		armorTypeMap[armorDefinition.Armor.ArmorType] = append(armorTypeMap[armorDefinition.Armor.ArmorType], armorDefinition)
	}

	// group by armor type
	availableArmorTypes := make([]common.ArmorType, 0)
	for armoreType, _ := range armorTypeMap {
		availableArmorTypes = append(availableArmorTypes, armoreType)
	}

	// pick random armor type
	armorType := availableArmorTypes[g.randomSource.Intn(len(availableArmorTypes))]

	// pick random armor definition
	armorDefinition := armorTypeMap[armorType][g.randomSource.Intn(len(armorTypeMap[armorType]))]

	// create item
	return g.NewItemFromDefinition(armorDefinition)
}
func (g *GameInstance) addRandomArmor(emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) *GameItem {
	armorPiece := g.NewRandomArmorPiece(dungeonLevel)
	emptyMap.AddItem(armorPiece, pos)
	return armorPiece
}
func NewLeatherArmor(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a leather armor", common.IconItemLeatherArmor1)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(1)

	armorPart := &common.ArmorStats{
		BaseArmor:  5,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeCuirass,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}
func NewChainMail(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a chain mail", common.IconItemChainMail)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(1)

	armorPart := &common.ArmorStats{
		BaseArmor:  7,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeCuirass,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewHelmet(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a helmet", common.IconItemHelmet)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(1)

	armorPart := &common.ArmorStats{
		BaseArmor:  2,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeHelmet,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewGloves(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a pair of gloves", common.IconItemGloves1)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(1)

	armorPart := &common.ArmorStats{
		BaseArmor:  2,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeGloves,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewBoots(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a pair of boots", common.IconItemBoots1)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(1)

	armorPart := &common.ArmorStats{
		BaseArmor:  2,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeBoots,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewCape(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a cape", common.IconItemCape1)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(1)

	armorPart := &common.ArmorStats{
		BaseArmor:  2,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeCape,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewShield(state actions.WorldState, source *rand.Rand, qLevel int) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "a shield", common.IconItemShield)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(1)

	armorPart := &common.ArmorStats{
		BaseArmor:  6,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeShield,
	}
	weaponPart := &common.WeaponStats{
		Damage:            stats.NewInterval(1, 6).Mul(float64(qLevel)),
		Durability:        stats.NewResource(0, flatFunc(20), 20),
		Type:              common.WeaponTypeShield,
		MeleeRange:        1,
		AttackSpeedFactor: 1 + float64(qLevel)/25,
	}

	baseItem.SetWeaponComponent(weaponPart)
	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}
