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


func flatFunc(i int) func() int {
	return func() int {
		return i
	}
}
