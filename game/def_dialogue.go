package game

import (
	"fmt"
	"io"
	"os"
	"path"
	"rpg/common"
	"rpg/stats"
	"rpg/util"
	"strings"
)
type TriggeredDialogue struct {
	NeededFlag   string
	DialogueFile string
	OnShow func()
}
func (g *GameInstance) getTextForNamedNPC(vendor *common.Actor) ([]util.LabelText, func()) { // kirioth
	textMap := map[string]func() []TriggeredDialogue{
		"kirioth": g.getTextForKirioth,
		"shekel":  g.getTextForShekel,
		"abel":    g.getTextForAbel,
	}
	if getTextMapForActor, ok := textMap[vendor.GetInternalName()]; ok {
		return g.getTextFromTriggers(getTextMapForActor())
	}
	return []util.LabelText{common.LabelTextFromString("Hello, I am " + vendor.Name())}, nil
}

func (g *GameInstance) getTextForKirioth() []TriggeredDialogue {
	addMeatJackQuest := func() {
		g.state.GetFlags().SetTrue("quest_received:meat_jack")
		g.Log(fmt.Sprintf("Quest received [:darkText]'%s'", "Kill Meat Jack"))
	}

	addMeatJackQuestReward := func() {
		// TODO: REWARDS ARE STATE CHANGES AND SHOULD THEREFORE BE AN ACTION..
		g.state.GetFlags().SetTrue("quest_rewarded:meat_jack")

		rewardItem := NewSword(g.GetWorldState(), g.randomSource, 2)
		g.state.GetPlayer().GetInventory().Add(rewardItem)

		rewardXP := 1000
		g.state.GetPlayer().GetStats().IncrementBy(stats.Experience, rewardXP)

		g.Log(fmt.Sprintf("Reward received [:darkText]'%s' and %d XP", rewardItem.Name(), rewardXP))
	}

	addLuciferQuest := func() {
		g.state.GetFlags().SetTrue("quest_received:lucifer")
		g.Log(fmt.Sprintf("Quest received [:darkText]'%s'", "Kill Lucifer"))
	}

	addLuciferQuestReward := func() {
		g.state.GetFlags().SetTrue("quest_rewarded:lucifer")

		rewardItem := NewSword(g.GetWorldState(), g.randomSource, 2)
		g.state.GetPlayer().GetInventory().Add(rewardItem)

		rewardXP := 1000
		g.state.GetPlayer().GetStats().IncrementBy(stats.Experience, rewardXP)

		g.Log(fmt.Sprintf("Reward received [:darkText]'%s' and %d XP", rewardItem.Name(), rewardXP))
	}


	dialogueTriggers := []TriggeredDialogue{
		// first quest for the human town
		{"game_started", "kirioth_welcome", addMeatJackQuest},
		{"quest_received:meat_jack", "kirioth_meat_jack", nil},
		{"killed:meat_jack", "kirioth_meat_jack_reward", addMeatJackQuestReward},
		{"quest_rewarded:meat_jack", "kirioth_after_meat_jack", nil},

		// final quest for the human town
		{"killed:skeleton_king", "kirioth_lucifer_quest", addLuciferQuest},
		{"quest_received:lucifer", "kirioth_lucifer_quest", nil},
		{"killed:lucifer", "kirioth_lucifer_quest_reward", addLuciferQuestReward},
		{"quest_rewarded:lucifer", "kirioth_after_lucifer", nil},
	}
	return dialogueTriggers
}

func (g *GameInstance) getTextForShekel() []TriggeredDialogue {
	addSkeletonKingQuest := func() {
		g.state.GetFlags().SetTrue("quest_received:skeleton_king")
		g.Log(fmt.Sprintf("Quest received [:darkText]'%s'", "Kill the Skeleton King"))
	}
	addSkeletonKingQuestReward := func() {
		g.state.GetFlags().SetTrue("quest_rewarded:skeleton_king")

		rewardItem := NewSword(g.GetWorldState(), g.randomSource, 2)
		g.state.GetPlayer().GetInventory().Add(rewardItem)

		rewardXP := 1000
		g.state.GetPlayer().GetStats().IncrementBy(stats.Experience, rewardXP)

		g.Log(fmt.Sprintf("Reward received [:darkText]'%s' and %d XP", rewardItem.Name(), rewardXP))
	}
	dialogueTriggers := []TriggeredDialogue{
		{"game_started", "shekel_welcome", nil},
		{string("all_clear:" + SpecialLevelPoisonedWaterSupply), "shekel_skeleton_king_quest", addSkeletonKingQuest},
		{"quest_received:skeleton_king", "shekel_skeleton_king_quest", nil},
		{"killed:skeleton_king", "shekel_skeleton_king_quest_reward", addSkeletonKingQuestReward},
		{"quest_rewarded:skeleton_king", "shekel_after_skeleton_king", nil},
	}
	return dialogueTriggers
}
func (g *GameInstance) getTextForAbel() []TriggeredDialogue {
	addPoisonQuest := func() {
		g.state.GetFlags().SetTrue("quest_received:poisoned_water")
		g.Log(fmt.Sprintf("Quest received [:darkText]'%s'", "Clean the poisoned water"))
	}

	addPoisonQuestReward := func() {
		g.state.GetFlags().SetTrue("quest_rewarded:poisoned_water")

		rewardItem := NewSword(g.GetWorldState(), g.randomSource, 2)
		g.state.GetPlayer().GetInventory().Add(rewardItem)

		rewardXP := 1000
		g.state.GetPlayer().GetStats().IncrementBy(stats.Experience, rewardXP)

		g.Log(fmt.Sprintf("Reward received [:darkText]'%s' and %d XP", rewardItem.Name(), rewardXP))
	}

	dialogueTriggers := []TriggeredDialogue{
		{"game_started", "abel_welcome", nil},
		{"killed:meat_jack", "abel_poison_quest", addPoisonQuest},
		{"quest_received:poisoned_water", "abel_poison_quest", nil},
		{string("all_clear:" + SpecialLevelPoisonedWaterSupply), "abel_poison_quest_reward", addPoisonQuestReward},
		{"quest_rewarded:poisoned_water", "abel_after_poison_quest", nil},
	}
	return dialogueTriggers
}

func (g *GameInstance) getTextFromTriggers(dialogueTriggers []TriggeredDialogue) ([]util.LabelText, func()) {
	flags := g.state.GetFlags()

	var lastPossibleTrigger TriggeredDialogue
	for _, trigger := range dialogueTriggers {
		if !flags.IsSet(trigger.NeededFlag) {
			break
		}
		lastPossibleTrigger = trigger
	}
	filename := fmt.Sprintf("%s.txt", lastPossibleTrigger.DialogueFile)
	return NewDialogueTextFromFile(filename), lastPossibleTrigger.OnShow
}

func NewDialogueTextFromFile(filename string) []util.LabelText {
	filepath := path.Join("assets", "dialogue", filename)

	file, err := os.Open(filepath)
	if err != nil {
		return nil
	}
	all, err := io.ReadAll(file)
	if err != nil {
		return nil
	}
	return toLabelText(strings.Split(string(all), "\n"))
}
