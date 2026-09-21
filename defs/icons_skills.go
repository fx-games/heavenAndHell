package defs

// skillsIcons is the name->icon lookup for files/assets/data/icons/skills.rec ("spell.<name>", built with
// defs.IconID, class skills included), edited at runtime with ./remap_skills.sh.
var skillsIcons map[string]int32

func loadSkillsIcons() {
	skillsIcons = loadIconRec("skills")
}
