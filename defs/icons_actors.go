package defs

// actorsIcons is the name->icon lookup for files/assets/data/icons/actors.rec, edited at runtime with ./remap_actors.sh.
var actorsIcons = loadIconRec("actors")

var (
	IconCharacterRogueMale       = actorsIcons["class.rogue_male"]
	IconCharacterRogueFemale     = actorsIcons["class.rogue_female"]
	IconCharacterWarriorMale     = actorsIcons["class.warrior_male"]
	IconCharacterWarriorFemale   = actorsIcons["class.warrior_female"]
	IconCharacterSorcererMale    = actorsIcons["class.sorcerer_male"]
	IconCharacterSorcererFemale  = actorsIcons["class.sorcerer_female"]
	IconCharacterMonkMale        = actorsIcons["class.monk_male"]
	IconCharacterMonkFemale      = actorsIcons["class.monk_female"]
	IconCharacterBardMale        = actorsIcons["class.bard_male"]
	IconCharacterBardFemale      = actorsIcons["class.bard_female"]
	IconCharacterBarbarianMale   = actorsIcons["class.barbarian_male"]
	IconCharacterBarbarianFemale = actorsIcons["class.barbarian_female"]
	IconCorpse                   = actorsIcons["corpse.default"]
)
