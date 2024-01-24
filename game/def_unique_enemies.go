package game

import (
	"rpg/common"
	"rpg/stats"
)

func NewUniqueEnemyFromName(name string) *common.Actor {
	switch name {
	case "meat_jack":
		return NewUniqueMeatJack()
	case "king_zedek":
		return NewKingZedek()
	case "lucifer":
		return NewLucifer()
	case "adventurer_knight": return NewAdventurerKnight()
	case "adventurer_priest": return NewAdventurerPriest()
	case "adventurer_barbarian": return NewAdventurerBarbarian()
	case "adventurer_archer": return NewAdventurerArcher()
	case "rogue": return NewRogue()
	case "rogue_dog": return NewRogueDog()
	case "evil_shekel": return NewEvilShekel()
	case "evil_kirioth": return NewEvilKirioth()
	}
	return nil
}

func NewLucifer() *common.Actor {
	actor := common.NewActor(common.IconUniqueLucifer)
	//index := len(emptyMap.Actors())
	actor.SetName("Lucifer")
	actor.SetInternalName("lucifer")
	actor.SetUnique()
	actor.SetHostile()
	actor.SetNoCorpse()
	actor.SetDeathStringFormats([]string{
		"%s vanishes in a puff of smoke.",
		"%s disappears in a flash of light.",
		"%s is gone.",
		"%s disappears.",
		"%s has left.",
	})
	actor.SetSpeech(map[common.ActorSpeechTrigger][]string{
		common.SpeechTriggerSight: {
			"I am the morning star!",
			"I am the fallen one!",
			"I am the lightbringer!",
			"I am the prince of darkness!",
			"I am just a scapegoat!",
			"I am the antichrist!",
			"I am the challenger!",
			"I am the prosecutor!",
			"I am the accuser!",
			"I am the tempter!",
			"I am the deceiver!",
			"I am the rebel!",
			"I am the adversary!",
			"I am the enemy!",
			"I am the opposer!",
			"I am the destroyer!",
			"I am the lord of the flies!",
			"I am the lord of the underworld!",
			"I am the lord of the night!",
			"I am the lord of hell!",
		},
	})
	actor.SetBaseSpeed(11)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 1200)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.MaxHealth, 420)
	actorStats.GetRessource(stats.Health).SetToMax()
	return actor
}

func NewKingZedek() *common.Actor {
	actor := common.NewActor(common.IconUniqueKingZedek)
	//index := len(emptyMap.Actors())
	actor.SetName("King Zedek")
	actor.SetInternalName("skeleton_king")
	actor.SetDeathStringFormats([]string{
		"%s crumbles to legendary dust.",
		"%s falls apart into royal pieces.",
		"%s is gone from the throne.",
		"%s dies again.",
	})
	actor.SetUnique()
	actor.SetHostile()
	actor.SetSpeech(map[common.ActorSpeechTrigger][]string{
		common.SpeechTriggerSight: []string{
			"We've all sworn the eternal vow!",
			"Your time to die is now!",
			"Still brave, beyond the grave!",
			"Victory! Victory! Eternal glory!",
		},
	})
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 200)
	actorStats.SetAttribute(stats.Strength, 9)
	actorStats.SetAttribute(stats.MaxHealth, 110)
	actorStats.GetRessource(stats.Health).SetToMax()
	return actor
}

func NewUniqueMeatJack() *common.Actor {
	actor := common.NewActor(common.IconUniqueMeatJack)
	//index := len(emptyMap.Actors())
	actor.SetName("Meat Jack")
	actor.SetInternalName("meat_jack")
	actor.SetUnique()
	actor.SetHostile()
	actor.SetDeathStringFormats([]string{
		"%s explodes into a pile of meat.",
		"%s blows up into a heap of blood and guts.",
		"%s was butchered.",
		"%s died an agonizing death.",
		"%s was slaughtered.",
		"%s was minced.",
		"%s was chopped.",
		"%s was diced.",
		"%s was sliced.",
		"%s was cut to pieces.",
	})
	actor.SetSpeech(map[common.ActorSpeechTrigger][]string{
		common.SpeechTriggerSight: []string{
			"Meet Meat Jack!",
			"Meat, meet Jack!",
			"Meat Jack meets meat!",
			"Jack, meet meat!",
			"Meat Jack, meet meat!",
			"I am Meat Jack!",
			"My name is.. Ji.. Jack!",
			//"But I was just a young boy\nwhen I was taken from my home\nand forced to work in the\nkitchens of the castle.",
		},
	})
	actor.SetBaseSpeed(11)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 200)
	actorStats.SetAttribute(stats.Strength, 9)
	actorStats.SetAttribute(stats.MaxHealth, 110)
	actorStats.GetRessource(stats.Health).SetToMax()
	return actor
}

func NewRogue() *common.Actor {
	actor := common.NewActor(common.IconUniqueRogue)
	//index := len(emptyMap.Actors())
	actor.SetName("The Rogue")
	actor.SetInternalName("rogue")
	actor.SetDeathStringFormats([]string{
		"%s ascends forever.",
		"%s descends forever.",
		"%s quaffed his last potion.",
		"%s dies again.",
	})
	actor.SetUnique()
	actor.SetHostile()
	actor.SetSpeech(map[common.ActorSpeechTrigger][]string{
		common.SpeechTriggerSight: []string{
			"qf",
			"za",
			"rb",
			"f",
		},
	})
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 200)
	actorStats.SetAttribute(stats.Strength, 9)
	actorStats.SetAttribute(stats.MaxHealth, 110)
	actorStats.GetRessource(stats.Health).SetToMax()
	return actor
}

func NewRogueDog() *common.Actor {
	actor := common.NewActor(common.IconUniqueRogueDog)
	//index := len(emptyMap.Actors())
	actor.SetName("a rogue dog")
	actor.SetInternalName("rogue_dog")
	actor.SetDeathStringFormats([]string{
		"%s goes to dog heaven.",
	})
	actor.SetUnique()
	actor.SetHostile()
	actor.SetSpeech(map[common.ActorSpeechTrigger][]string{
		common.SpeechTriggerSight: []string{
			"f",
		},
	})
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 200)
	actorStats.SetAttribute(stats.Strength, 9)
	actorStats.SetAttribute(stats.MaxHealth, 110)
	actorStats.GetRessource(stats.Health).SetToMax()
	return actor
}

func NewEvilShekel() *common.Actor {
	actor := common.NewActor(common.IconUniqueEvilShekel)
	//index := len(emptyMap.Actors())
	actor.SetName("the true Shekel")
	actor.SetInternalName("evil_shekel")
	actor.SetDeathStringFormats([]string{
		"%s dies.",
	})
	actor.SetUnique()
	actor.SetHostile()
	actor.SetSpeech(map[common.ActorSpeechTrigger][]string{
		common.SpeechTriggerSight: []string{
			"Your time has come!",
		},
	})
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 200)
	actorStats.SetAttribute(stats.Strength, 9)
	actorStats.SetAttribute(stats.MaxHealth, 110)
	actorStats.GetRessource(stats.Health).SetToMax()
	return actor
}

func NewEvilKirioth() *common.Actor {
	actor := common.NewActor(common.IconUniqueEvilKirioth)
	//index := len(emptyMap.Actors())
	actor.SetName("the true Kirioth")
	actor.SetInternalName("evil_kirioth")
	actor.SetDeathStringFormats([]string{
		"%s dies.",
	})
	actor.SetUnique()
	actor.SetHostile()
	actor.SetSpeech(map[common.ActorSpeechTrigger][]string{
		common.SpeechTriggerSight: []string{
			"Your time has come!",
		},
	})
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 200)
	actorStats.SetAttribute(stats.Strength, 9)
	actorStats.SetAttribute(stats.MaxHealth, 110)
	actorStats.GetRessource(stats.Health).SetToMax()
	return actor
}

func NewAdventurerKnight() *common.Actor {
	actor := common.NewActor(common.IconUniqueAdventurerKnight)
	//index := len(emptyMap.Actors())
	actor.SetName("a red knight")
	actor.SetInternalName("adventurer_knight")
	actor.SetDeathStringFormats([]string{
		"%s dies.",
	})
	actor.SetUnique()
	actor.SetHostile()
	actor.SetSpeech(map[common.ActorSpeechTrigger][]string{
		common.SpeechTriggerSight: []string{
			"Your time has come!",
		},
	})
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 200)
	actorStats.SetAttribute(stats.Strength, 9)
	actorStats.SetAttribute(stats.MaxHealth, 110)
	actorStats.GetRessource(stats.Health).SetToMax()
	return actor
}

func NewAdventurerPriest() *common.Actor {
	actor := common.NewActor(common.IconUniqueAdventurerPriest)
	//index := len(emptyMap.Actors())
	actor.SetName("a false prophet")
	actor.SetInternalName("adventurer_priest")
	actor.SetDeathStringFormats([]string{
		"%s dies.",
	})
	actor.SetUnique()
	actor.SetHostile()
	actor.SetSpeech(map[common.ActorSpeechTrigger][]string{
		common.SpeechTriggerSight: []string{
			"Your time has come!",
		},
	})
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 200)
	actorStats.SetAttribute(stats.Strength, 9)
	actorStats.SetAttribute(stats.MaxHealth, 110)
	actorStats.GetRessource(stats.Health).SetToMax()
	return actor
}

func NewAdventurerBarbarian() *common.Actor {
	actor := common.NewActor(common.IconUniqueAdventurerBarbarian)
	//index := len(emptyMap.Actors())
	actor.SetName("a barbarian")
	actor.SetInternalName("adventurer_barbarian")
	actor.SetDeathStringFormats([]string{
		"%s dies.",
	})
	actor.SetUnique()
	actor.SetHostile()
	actor.SetSpeech(map[common.ActorSpeechTrigger][]string{
		common.SpeechTriggerSight: []string{
			"Your time has come!",
		},
	})
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 200)
	actorStats.SetAttribute(stats.Strength, 9)
	actorStats.SetAttribute(stats.MaxHealth, 110)
	actorStats.GetRessource(stats.Health).SetToMax()
	return actor
}

func NewAdventurerArcher() *common.Actor {
	actor := common.NewActor(common.IconUniqueAdventurerArcher)
	//index := len(emptyMap.Actors())
	actor.SetName("an archer in tights")
	actor.SetInternalName("adventurer_archer")
	actor.SetDeathStringFormats([]string{
		"%s dies.",
	})
	actor.SetUnique()
	actor.SetHostile()
	actor.SetSpeech(map[common.ActorSpeechTrigger][]string{
		common.SpeechTriggerSight: []string{
			"Your time has come!",
		},
	})
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 200)
	actorStats.SetAttribute(stats.Strength, 9)
	actorStats.SetAttribute(stats.MaxHealth, 110)
	actorStats.GetRessource(stats.Health).SetToMax()
	return actor
}