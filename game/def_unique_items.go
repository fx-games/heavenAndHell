package game

import (
	"fmt"
	"rpg/common"
	"rpg/defs"
	"rpg/stats"
	"rpg/util"
)

func (g *GameInstance) NewPotionOfTrueSight() *common.Item {
	baseItem := common.NewItem(g.GetWorldState().GetPlayer, "Lucifer's gift", defs.IconItemPotionCrimson)
	baseItem.SetInternalName("potion_of_true_sight")
	baseItem.SetCategory(defs.ItemCategoryPotion)
	baseItem.SetRarity(common.ItemRarityUnique)
	baseItem.SetQualityLevel(10)
	baseItem.SetCharges(1)
	baseItem.SetDestroyAfterUse(true)
	drinkAction := common.NewUntargetedAction("Drink", 34)
	effect := func(user *common.Actor, source common.EffectSource) {
		g.GetWorldState().GetPlayer().GetStats().IncrementBy(stats.VisionRange, 7)
		g.GetWorldState().GetPlayer().GetStats().IncrementBy(stats.LightRadius, 7)
		g.GetWorldState().GetPlayer().GetInventory().Remove(baseItem)
		g.GetWorldState().GetFlags().SetTrue(string(common.FlagHasTrueSight))
		g.actionBroker.Execute(common.NewUpdateFoVAction(common.AbstractCause("TrueSightActivated"), g.state.GetPlayer()))

		g.state.GetFlags().SetTrue("quest_received:evidence_one")
		g.state.GetFlags().SetTrue("quest_received:evidence_two")
		g.state.GetFlags().SetTrue("quest_received:evidence_three")
		g.state.GetFlags().SetTrue("quest_received:evidence_four")
		g.PlayerMessage(fmt.Sprintf("Quest received [:darkText]'%s'", "Find evidence against humanity"))
	}

	drinkAction.SetEffect(&common.UntargetedEffect{
		Effect: effect,
	})

	baseItem.SetTooltip([]util.LabelText{
		util.LabelTextFromString(""),
		util.LabelTextFromString("Can this be real? Can it be trusted?"),
		util.LabelTextFromString("Only one way to find out..."),
	})
	return baseItem
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
func (g *GameInstance) NewEvidenceOne() *common.Item {
	setFlag := func() { g.state.GetFlags().SetTrue("found:evidence_one") }
	title := "Purity report"
	textFile := "note_evidence_one.txt"
	note := g.factory.NewHiddenNote(title, common.NewDialogueTextFromFile(textFile), setFlag)
	return note
}

func (g *GameInstance) NewEvidenceTwo() *common.Item {
	setFlag := func() { g.state.GetFlags().SetTrue("found:evidence_two") }
	title := "King Zedek's last will"
	textFile := "note_evidence_two.txt"
	note := g.factory.NewHiddenNote(title, common.NewDialogueTextFromFile(textFile), setFlag)
	return note
}

func (g *GameInstance) NewEvidenceThree() *common.Item {
	setFlag := func() { g.state.GetFlags().SetTrue("found:evidence_three") }
	title := "Kirioth's schemes"
	textFile := "note_evidence_three.txt"
	note := g.factory.NewHiddenNote(title, common.NewDialogueTextFromFile(textFile), setFlag)
	return note
}

func (g *GameInstance) NewEvidenceFour() *common.Item {
	setFlag := func() { g.state.GetFlags().SetTrue("found:evidence_four") }
	title := "From K. to S."
	textFile := "note_evidence_four.txt"
	note := g.factory.NewHiddenNote(title, common.NewDialogueTextFromFile(textFile), setFlag)
	return note
}

func NewMasterSword(state *common.State) *common.Item {
	// spell on attack
	baseItem := common.NewItem(state.GetPlayer, "a master sword", defs.IconItemUniqueSword)
	baseItem.SetInternalName("master_sword")
	baseItem.SetCategory(defs.ItemCategoryWeapon)
	baseItem.SetQualityLevel(1)
	baseItem.SetRarity(common.ItemRarityUnique)
	weaponPart := &common.WeaponStats{
		Damage:            util.ParseDice("2d8+3"),
		Durability:        stats.NewResource(0, flatFunc(20), 20),
		Type:              defs.WeaponTypeSword,
		MeleeRange:        1,
		AttackSpeedFactor: 1,
		Requirements: map[stats.Name]int{
			stats.Level:    3,
			stats.Strength: 4,
		},
	}

	baseItem.SetWeaponComponent(weaponPart)

	// stat modifiers
	modifiers := []*stats.Modifier{
		{
			Description: "of strength",
			Attribute:   stats.Strength,
			Value:       5,
			Mode:        stats.ModifyAdditive,
		},
		{
			Description:    "speedy",
			Attribute:      stats.AttackSpeedFactor,
			Value:          1.2,
			Mode:           stats.ModifyMultiplicative,
			IsPercentBased: true,
		},
		{
			Description:    "deadly",
			Attribute:      stats.DamageBonus,
			Value:          1,
			Mode:           stats.ModifyAdditive,
			IsPercentBased: false,
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
	return baseItem
}

func flatFunc(i int) func() int {
	return func() int {
		return i
	}
}
