package common

import (
	"fmt"
	"io"
	"path"
	"rpg/defs"
	"rpg/files"
	"rpg/stats"
	"rpg/util"
	"strings"
)

type TriggeredDialogue struct {
	NeededFlag   string
	DialogueFile string
	OnShow       func()
}

func (s *State) GetTextForNamedNPC(vendor *Actor) ([]util.LabelText, func()) { // kirioth
	textMap := map[string]func() []TriggeredDialogue{
		"kirioth": s.getTextForKirioth,
		"shekel":  s.getTextForShekel,
		"abel":    s.getTextForAbel,
		"mary":    s.getTextForMary,
	}
	if getTextMapForActor, ok := textMap[vendor.GetInternalName()]; ok {
		return s.getTextFromTriggers(getTextMapForActor())
	}
	return []util.LabelText{util.LabelTextFromString("Hello, I am " + vendor.Name())}, nil
}
func (s *State) getTextForMary() []TriggeredDialogue {
	dialogueTriggers := []TriggeredDialogue{
		// first quest for the human town
		{"game_started", "mary_welcome", nil},
	}
	return dialogueTriggers
}

func (s *State) getTextForKirioth() []TriggeredDialogue {
	addMeatJackQuest := func() {
		s.GetFlags().SetTrue("quest_received:meat_jack")
		s.Log(fmt.Sprintf("Quest received [:darkText]'%s'", "Kill Meat Jack"))
	}

	addMeatJackQuestReward := func() {
		// TODO: REWARDS ARE STATE CHANGES AND SHOULD THEREFORE BE AN ACTION..
		s.GetFlags().SetTrue("quest_rewarded:meat_jack")

		rewardItem := s.factory.NewRandomWeapon(4)

		s.GetPlayer().GetInventory().Add(rewardItem)

		rewardXP := 1000
		s.GetPlayer().GetStats().IncrementBy(stats.Experience, rewardXP)

		s.Log(fmt.Sprintf("Reward received [:darkText]'%s' and %d XP", rewardItem.Name(), rewardXP))
	}

	addLuciferQuest := func() {
		s.GetFlags().SetTrue("quest_received:lucifer")
		s.Log(fmt.Sprintf("Quest received [:darkText]'%s'", "Kill Lucifer"))
	}

	addLuciferQuestReward := func() {
		s.GetFlags().SetTrue("quest_rewarded:lucifer")

		rewardItem := s.factory.NewRandomWeapon(10)
		s.GetPlayer().GetInventory().Add(rewardItem)

		rewardXP := 1000
		s.GetPlayer().GetStats().IncrementBy(stats.Experience, rewardXP)

		s.Log(fmt.Sprintf("Reward received [:darkText]'%s' and %d XP", rewardItem.Name(), rewardXP))
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

func (s *State) getTextForShekel() []TriggeredDialogue {
	addSkeletonKingQuest := func() {
		s.GetFlags().SetTrue("quest_received:skeleton_king")
		s.Log(fmt.Sprintf("Quest received [:darkText]'%s'", "Kill the Skeleton King"))
	}
	addSkeletonKingQuestReward := func() {
		s.GetFlags().SetTrue("quest_rewarded:skeleton_king")

		rewardItem := s.factory.NewRandomWeapon(6)
		s.GetPlayer().GetInventory().Add(rewardItem)

		rewardXP := 1000
		s.GetPlayer().GetStats().IncrementBy(stats.Experience, rewardXP)

		s.Log(fmt.Sprintf("Reward received [:darkText]'%s' and %d XP", rewardItem.Name(), rewardXP))
	}
	dialogueTriggers := []TriggeredDialogue{
		{"game_started", "shekel_welcome", nil},
		{string("all_clear:" + defs.SpecialLevelPoisonedWaterSupply), "shekel_skeleton_king_quest", addSkeletonKingQuest},
		{"quest_received:skeleton_king", "shekel_skeleton_king_quest", nil},
		{"killed:skeleton_king", "shekel_skeleton_king_quest_reward", addSkeletonKingQuestReward},
		{"quest_rewarded:skeleton_king", "shekel_after_skeleton_king", nil},
	}
	return dialogueTriggers
}
func (s *State) getTextForAbel() []TriggeredDialogue {
	addPoisonQuest := func() {
		s.GetFlags().SetTrue("quest_received:poisoned_water")
		s.Log(fmt.Sprintf("Quest received [:darkText]'%s'", "Clean the poisoned water"))
	}

	addPoisonQuestReward := func() {
		s.GetFlags().SetTrue("quest_rewarded:poisoned_water")

		rewardItem := s.factory.NewRandomWeapon(6)
		s.GetPlayer().GetInventory().Add(rewardItem)

		rewardXP := 1000
		s.GetPlayer().GetStats().IncrementBy(stats.Experience, rewardXP)

		s.Log(fmt.Sprintf("Reward received [:darkText]'%s' and %d XP", rewardItem.Name(), rewardXP))
	}

	dialogueTriggers := []TriggeredDialogue{
		{"game_started", "abel_welcome", nil},
		{"killed:meat_jack", "abel_poison_quest", addPoisonQuest},
		{"quest_received:poisoned_water", "abel_poison_quest", nil},
		{string("all_clear:" + defs.SpecialLevelPoisonedWaterSupply), "abel_poison_quest_reward", addPoisonQuestReward},
		{"quest_rewarded:poisoned_water", "abel_after_poison_quest", nil},
	}
	return dialogueTriggers
}

func (s *State) getTextFromTriggers(dialogueTriggers []TriggeredDialogue) ([]util.LabelText, func()) {
	flags := s.GetFlags()

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

	file, err := files.Open(filepath)
	if err != nil {
		return nil
	}
	all, err := io.ReadAll(file)
	if err != nil {
		return nil
	}
	return util.ToLabelText(strings.Split(string(all), "\n"))
}
