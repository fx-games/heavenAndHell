package game

import (
	"fmt"
	"rpg/actions"
	"rpg/common"
	"rpg/stats"
	"rpg/util"
)
type FlagName string

const (
	FlagHasTrueSight FlagName = "has_true_sight"
	FlagStoryModeEnabled FlagName = "story_mode_enabled"
)
func (g *GameInstance) NewPotionOfTrueSight() *GameItem {
	baseItem := common.NewBaseItem(g.GetWorldState().GetPlayer, "Lucifer's gift", common.IconItemPotionCrimson)
	baseItem.SetCategory(common.ItemCategoryPotion)
	baseItem.SetQualityLevel(10)

	gameItem := &GameItem{
		BaseItem:         baseItem,
		actionCharges:    1,
		destroyAfterUse: true,
	}
	drinkAction := common.NewUntargetedAction("Drink", 34)
	drinkAction.SetEffect(func(user *common.Actor) {
		g.GetWorldState().GetPlayer().GetStats().IncrementBy(stats.VisionRange, 7)
		g.GetWorldState().GetPlayer().GetStats().IncrementBy(stats.LightRadius, 7)
		g.GetWorldState().GetPlayer().GetInventory().Remove(gameItem)
		g.GetWorldState().GetFlags().SetTrue(string(FlagHasTrueSight))
		g.actionBroker.Execute(actions.NewUpdateFoVAction(actions.AbstractCause("TrueSightActivated"), g.state.GetPlayer()))

		g.state.GetFlags().SetTrue("quest_received:evidence_one")
		g.state.GetFlags().SetTrue("quest_received:evidence_two")
		g.state.GetFlags().SetTrue("quest_received:evidence_three")
		g.state.GetFlags().SetTrue("quest_received:evidence_four")
		g.Log(fmt.Sprintf("Quest received [:darkText]'%s'", "Find evidence against humanity"))
	})

	gameItem.SetTooltip([]util.LabelText{
		common.LabelTextFromString(""),
		common.LabelTextFromString("Can this be real? Can it be trusted?"),
		common.LabelTextFromString("Only one way to find out..."),
	})
	return gameItem
}
/*
1. Shekel poisoned the water supply and framed the workers union
 - Report from the workers found at the poisoned water supply
2. Abel backstabbed the skeleton king - Last will of the dead good king (now skeleton king)
 - Last will of the dead good king (now skeleton king), at his tomb
3. Kirioth persuaded both of them to follow his plan and promised the wealth and status
Kirioth sacrificed some poor people when he summoned Lucifer, trying to control him
 - Shekel's notes on Kirioth's plan, security to be used for blackmailing, in Shekel's house
4. Shekel & Kirioth are responsible for the creation of meat jack
 - List of subject for the super soldier experiment, there is a lab in the caves.-- TODO*
 */
func (g *GameInstance) NewEvidenceOne() *GameItem {
	setFlag := func() {g.state.GetFlags().SetTrue("found:evidence_one")}
	title := "Purity report"
	textFile := "note_evidence_one.txt"
	note := g.NewHiddenNote(title, NewDialogueTextFromFile(textFile), setFlag)
	return note
}

func (g *GameInstance) NewEvidenceTwo() *GameItem {
	setFlag := func() {g.state.GetFlags().SetTrue("found:evidence_two")}
	title := "King Zedek's last will"
	textFile := "note_evidence_two.txt"
	note := g.NewHiddenNote(title, NewDialogueTextFromFile(textFile), setFlag)
	return note
}

func (g *GameInstance) NewEvidenceThree() *GameItem {
	setFlag := func() {g.state.GetFlags().SetTrue("found:evidence_three")}
	title := "Kirioth's schemes"
	textFile := "note_evidence_three.txt"
	note := g.NewHiddenNote(title, NewDialogueTextFromFile(textFile), setFlag)
	return note
}

func (g *GameInstance) NewEvidenceFour() *GameItem {
	setFlag := func() {g.state.GetFlags().SetTrue("found:evidence_four")}
	title := "From K. to S."
	textFile := "note_evidence_four.txt"
	note := g.NewHiddenNote(title, NewDialogueTextFromFile(textFile), setFlag)
	return note
}

func NewMasterSword(state actions.WorldState) *GameItem {
	// spell on attack
	baseItem := common.NewBaseItem(state.GetPlayer, "a master sword", common.IconItemUniqueSword)
	baseItem.SetCategory(common.ItemCategoryWeapon)
	baseItem.SetQualityLevel(1)
	baseItem.SetRarity(common.ItemRarityUnique)
	weaponPart := &common.WeaponStats{
		Damage:            stats.NewInterval(1, 6),
		Durability:        stats.NewResource(0, flatFunc(20), 20),
		Type:              common.WeaponTypeSword,
		MeleeRange:        1,
		AttackSpeedFactor: 1,
		Requirements: map[stats.Name]int{
			stats.Level:    3,
			stats.Strength: 4,
		},
		// attack pattern for an axe..
		DamagePerLocationPrediction: getBigAxeDamageLocations(state),
	}

	baseItem.SetWeaponComponent(weaponPart)

	// stat modifiers
	modifiers := []*stats.Modifier{
		{
			Name:      "of strength",
			Attribute: stats.Strength,
			Value:     5,
			Mode:      stats.ModifyAdditive,
		},
		{
			Name:           "speedy",
			Attribute:      stats.AttackSpeedFactor,
			Value:          1.2,
			Mode:           stats.ModifyMultiplicative,
			IsPercentBased: true,
		},
		{
			Name:           "deadly",
			Attribute:      stats.EnhancedDamageFactor,
			Value:          0.2,
			Mode:           stats.ModifyAdditive,
			IsPercentBased: true,
		},
	}

	baseItem.SetEquippedModifiers(modifiers)

	// TODO: Diablo Style Loot Branding
	// 1. TRIGGERED ACTION
	// -> X % chance to execute Action Y on Hit/Attack/Kill/BeingHit/BeingAttacked
	//    - Invokable Action X (with optional maximum charges)
	//    - X Damage of Type Y to Attacker on Attack/Hit
	//    - Steal X Mana/HP per hit
	//    - X% to Apply Status Effect X
	// 2. STAT MODIFIER
	// - +X to Skill Y
	// - +U..V Physical/Fire Damage, +W Poison damage over X seconds
	// - +X to Maximum Damage
	// - +X% HP/Mana Regeneration per AUT
	// - Requirements -60%
	// - Resistance / Immunity to Status Effect X
	// - Resistance / Immunity / Absorption to Damage Type X
	masterSword := &GameItem{
		BaseItem: baseItem,
		itemHooks: []actions.ItemActionHook{ // these need to be registered during onEquip, at the same time the ItemContext must be set
			returnDamageOnHit(),
			randomlyTripleDamageOnMelee(),
		},
	}
	return masterSword
}


/*
Some End Game Uniques
 - Veil of Darkness - Unique cloak [+10, +0]
 - Black Hands - Unique gloves [+10, +0]
 - Boots of the Shadow Dancer - Unique boots [+10, +0]
 - Ancient Nightmare's Hood - Unique helmet [+10, +0]
 - Plate Mail of the Dark Lord - Unique plate mail [+40, +0]
 - Never-ending Night - Unique ring [+0, +0]
 - Eternal Darkness - Unique ring [+0, +0]
 - Amulet of Shadows - Unique amulet [+0, +0]
 - Shadow's Embrace - Unique amulet [+0, +0]
 - Wall of Thorns - Unique shield [+30, +0]
 */

func NewVeilOfDarkness(state actions.WorldState) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "Veil of Darkness", common.IconCloakDark)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(10)
	baseItem.SetRarity(common.ItemRarityUnique)

	armorPart := &common.ArmorStats{
		BaseArmor: 10,
		Plus: 0,
		Type: common.ArmorTypeCape,
		Durability: stats.NewResource(0, flatFunc(20), 20),
	}

	baseItem.SetArmorComponent(armorPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewBootsOfTheShadowDancer(state actions.WorldState) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "Boots of the Shadow Dancer", common.IconBootsDark)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(10)
	baseItem.SetRarity(common.ItemRarityUnique)

	armorPart := &common.ArmorStats{
		BaseArmor: 10,
		Plus: 0,
		Type: common.ArmorTypeBoots,
		Durability: stats.NewResource(0, flatFunc(20), 20),
	}

	baseItem.SetArmorComponent(armorPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewBlackHands(state actions.WorldState) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "Black Hands", common.IconGlovesDark)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(10)
	baseItem.SetRarity(common.ItemRarityUnique)

	armorPart := &common.ArmorStats{
		BaseArmor: 10,
		Plus: 0,
		Type: common.ArmorTypeGloves,
		Durability: stats.NewResource(0, flatFunc(20), 20),
	}

	baseItem.SetArmorComponent(armorPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewAncientNightmaresHood(state actions.WorldState) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "Ancient Nightmare's Hood", common.IconHoodDark)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(10)
	baseItem.SetRarity(common.ItemRarityUnique)

	armorPart := &common.ArmorStats{
		BaseArmor: 10,
		Plus: 0,
		Type: common.ArmorTypeHelmet,
		Durability: stats.NewResource(0, flatFunc(20), 20),
	}

	baseItem.SetArmorComponent(armorPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewPlateMailOfTheDarkLord(state actions.WorldState) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "Plate Mail of the Dark Lord", common.IconArmorDark)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(10)
	baseItem.SetRarity(common.ItemRarityUnique)

	armorPart := &common.ArmorStats{
		BaseArmor: 40,
		Plus: 0,
		Type: common.ArmorTypeCuirass,
		Durability: stats.NewResource(0, flatFunc(20), 20),
	}

	baseItem.SetArmorComponent(armorPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewWallOfThorns(state actions.WorldState) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "Wall of Thorns", common.IconShieldDarkThorns)
	baseItem.SetCategory(common.ItemCategoryArmor)
	baseItem.SetQualityLevel(10)
	baseItem.SetRarity(common.ItemRarityUnique)

	armorPart := &common.ArmorStats{
		BaseArmor: 30,
		Plus: 0,
		Type: common.ArmorTypeShield,
		Durability: stats.NewResource(0, flatFunc(20), 20),
	}

	baseItem.SetArmorComponent(armorPart)

	return &GameItem{
		BaseItem: baseItem,
		itemHooks: []actions.ItemActionHook{
			returnDamageOnHit(),
		},
	}
}

func NewAmuletOfShadows(state actions.WorldState) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "Amulet of Shadows", common.IconAmuletDark)
	baseItem.SetCategory(common.ItemCategoryAccessory)
	baseItem.SetQualityLevel(10)

	armorPart := &common.ArmorStats{
		BaseArmor:  0,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeAmulet,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewShadowsEmbrace(state actions.WorldState) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "Shadow's Embrace", common.IconAmuletDark)
	baseItem.SetCategory(common.ItemCategoryAccessory)
	baseItem.SetQualityLevel(10)

	armorPart := &common.ArmorStats{
		BaseArmor:  0,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeAmulet,
	}

	baseItem.SetArmorComponent(armorPart)
	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewEternalDarkness(state actions.WorldState) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "Eternal Darkness", common.IconRingDark)
	baseItem.SetCategory(common.ItemCategoryAccessory)
	baseItem.SetQualityLevel(10)

	armorPart := &common.ArmorStats{
		BaseArmor:  0,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeRing,
	}

	baseItem.SetArmorComponent(armorPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewNeverEndingNight(state actions.WorldState) *GameItem {
	baseItem := common.NewBaseItem(state.GetPlayer, "Never-ending Night", common.IconRingDark)
	baseItem.SetCategory(common.ItemCategoryAccessory)
	baseItem.SetQualityLevel(10)

	armorPart := &common.ArmorStats{
		BaseArmor:  0,
		Durability: stats.NewResource(0, flatFunc(20), 20),
		Type:       common.ArmorTypeRing,
	}

	baseItem.SetArmorComponent(armorPart)

	return &GameItem{
		BaseItem: baseItem,
	}
}

func NewSoulReaver(state actions.WorldState) *GameItem {
	// spell on attack
	baseItem := common.NewBaseItem(state.GetPlayer, "Soul Reaver", common.IconScytheDark)
	baseItem.SetCategory(common.ItemCategoryWeapon)
	baseItem.SetQualityLevel(10)
	baseItem.SetRarity(common.ItemRarityUnique)
	weaponPart := &common.WeaponStats{
		Damage:            stats.NewInterval(20, 35),
		Durability:        stats.NewResource(0, flatFunc(50), 50),
		Type:              common.WeaponTypeScythe,
		MeleeRange:        1,
		AttackSpeedFactor: 0.9,
		Requirements: map[stats.Name]int{
			stats.Level:    15,
			stats.Strength: 9,
		},
		// attack pattern for an axe..
		DamagePerLocationPrediction: getBigAxeDamageLocations(state),
	}

	baseItem.SetWeaponComponent(weaponPart)

	// stat modifiers
	modifiers := []*stats.Modifier{
		{
			Name:           "deadly",
			Attribute:      stats.EnhancedDamageFactor,
			Value:          0.2,
			Mode:           stats.ModifyAdditive,
			IsPercentBased: true,
		},
	}

	baseItem.SetEquippedModifiers(modifiers)

	masterSword := &GameItem{
		BaseItem: baseItem,
		itemHooks: []actions.ItemActionHook{ // these need to be registered during onEquip, at the same time the ItemContext must be set
			randomlyTripleDamageOnMelee(),
		},
	}
	return masterSword
}

func NewMalleusMaleficarum(state actions.WorldState) *GameItem {
	// spell on attack
	baseItem := common.NewBaseItem(state.GetPlayer, "Malleus Maleficarum", common.IconMalletGolden)
	baseItem.SetCategory(common.ItemCategoryWeapon)
	baseItem.SetQualityLevel(10)
	baseItem.SetRarity(common.ItemRarityUnique)
	weaponPart := &common.WeaponStats{
		Damage:            stats.NewInterval(15, 45),
		Durability:        stats.NewResource(0, flatFunc(35), 35),
		Type:              common.WeaponTypeMace,
		MeleeRange:        1,
		AttackSpeedFactor: 0.9,
		Requirements: map[stats.Name]int{
			stats.Level:    15,
			stats.Strength: 14,
		},
		// attack pattern for an axe..
		DamagePerLocationPrediction: getBigAxeDamageLocations(state),
	}

	baseItem.SetWeaponComponent(weaponPart)

	// stat modifiers
	modifiers := []*stats.Modifier{
		{
			Name:           "deadly",
			Attribute:      stats.EnhancedDamageFactor,
			Value:          0.2,
			Mode:           stats.ModifyAdditive,
			IsPercentBased: true,
		},
	}

	baseItem.SetEquippedModifiers(modifiers)

	masterSword := &GameItem{
		BaseItem: baseItem,
		itemHooks: []actions.ItemActionHook{ // these need to be registered during onEquip, at the same time the ItemContext must be set
			randomlyTripleDamageOnMelee(),
		},
	}
	return masterSword
}
