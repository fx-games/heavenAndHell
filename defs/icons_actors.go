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

func loadActorsIcons() {
	actorsIcons = loadIconRec("actors")
	IconCharacterRogueMale = actorsIcons["class.rogue_male"]
	IconCharacterRogueFemale = actorsIcons["class.rogue_female"]
	IconCharacterWarriorMale = actorsIcons["class.warrior_male"]
	IconCharacterWarriorFemale = actorsIcons["class.warrior_female"]
	IconCharacterSorcererMale = actorsIcons["class.sorcerer_male"]
	IconCharacterSorcererFemale = actorsIcons["class.sorcerer_female"]
	IconCharacterMonkMale = actorsIcons["class.monk_male"]
	IconCharacterMonkFemale = actorsIcons["class.monk_female"]
	IconCharacterBardMale = actorsIcons["class.bard_male"]
	IconCharacterBardFemale = actorsIcons["class.bard_female"]
	IconCharacterBarbarianMale = actorsIcons["class.barbarian_male"]
	IconCharacterBarbarianFemale = actorsIcons["class.barbarian_female"]
	IconCorpse = actorsIcons["corpse.default"]
}
