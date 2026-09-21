package defs

// actorsIcons is the name->icon lookup for files/assets/data/icons/actors.rec, edited at runtime with ./remap_actors.sh.
var actorsIcons map[string]int32

var (
	IconCharacterRogueMale       int32
	IconCharacterRogueFemale     int32
	IconCharacterWarriorMale     int32
	IconCharacterWarriorFemale   int32
	IconCharacterSorcererMale    int32
	IconCharacterSorcererFemale  int32
	IconCharacterMonkMale        int32
	IconCharacterMonkFemale      int32
	IconCharacterBardMale        int32
	IconCharacterBardFemale      int32
	IconCharacterBarbarianMale   int32
	IconCharacterBarbarianFemale int32
	IconCorpse                   int32
)

func actorIcon(name string) int32 { return namedIcon(actorsIcons, name) }

func loadActorsIcons() {
	actorsIcons = loadIconRec("actors")
	IconCharacterRogueMale = actorIcon("class.rogue_male")
	IconCharacterRogueFemale = actorIcon("class.rogue_female")
	IconCharacterWarriorMale = actorIcon("class.warrior_male")
	IconCharacterWarriorFemale = actorIcon("class.warrior_female")
	IconCharacterSorcererMale = actorIcon("class.sorcerer_male")
	IconCharacterSorcererFemale = actorIcon("class.sorcerer_female")
	IconCharacterMonkMale = actorIcon("class.monk_male")
	IconCharacterMonkFemale = actorIcon("class.monk_female")
	IconCharacterBardMale = actorIcon("class.bard_male")
	IconCharacterBardFemale = actorIcon("class.bard_female")
	IconCharacterBarbarianMale = actorIcon("class.barbarian_male")
	IconCharacterBarbarianFemale = actorIcon("class.barbarian_female")
	IconCorpse = actorIcon("corpse.default")
}
