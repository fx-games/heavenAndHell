package defs

import (
	"io"
	"rpg/recfile"
	"strings"
)

// ClassIcons resolves a class' two sprites from the active mod's actors.rec by the name the mod's classes.rec
// gave the class: "archer" wants class.archer_male and class.archer_female. The engine keeps no roster of class
// names - which classes exist is the mod's business - so a name the mod's icon table does not carry resolves to
// 0 here rather than failing LoadIcons, and is reported when the class is read.
func ClassIcons(internalName string) (male, female int32, ok bool) {
	male, maleOK := IconOK(IconID("class", internalName+"_male"))
	female, femaleOK := IconOK(IconID("class", internalName+"_female"))
	return male, female, maleOK && femaleOK
}

type LoadoutDefinition struct {
	Items            []string
	Icon             int32
	Gold             int
	Attributes       map[string]float64
	Resources        map[string]float64
	UntargetedSkills []string
	TargetedSkills   []string
	InternalName     string
	Name             string
	FemaleIcon       int32 // the other sprite; Icon carries whichever gender was picked
}

// IsFemale reports which of the class' two sprites this loadout carries. Saving and the hero's voice ask it.
func (d LoadoutDefinition) IsFemale() bool { return d.Icon == d.FemaleIcon }

func ReadClassDefinitions(file io.Reader) []LoadoutDefinition {
	records := recfile.Read(file)
	targetedDefs := make([]LoadoutDefinition, len(records))
	for i, record := range records {
		def := NewClassDefFromRecord(record)
		targetedDefs[i] = def
	}
	return targetedDefs
}
func NewClassDefFromRecord(record recfile.Record) LoadoutDefinition {
	def := LoadoutDefinition{
		Attributes: make(map[string]float64),
		Resources:  make(map[string]float64),
	}
	for _, field := range record {
		if strings.HasPrefix(field.Name, "stats_") {
			statName := strings.TrimPrefix(field.Name, "stats_")
			def.Attributes[statName] = field.AsFloat()
			continue
		}
		if strings.HasPrefix(field.Name, "resource_") {
			resourceName := strings.TrimPrefix(field.Name, "resource_")
			def.Resources[resourceName] = field.AsFloat()
			continue
		}
		switch field.Name {
		case "internal_name":
			def.InternalName = field.Value
		case "name":
			def.Name = field.Value
		case "gold":
			def.Gold = field.AsInt()
		case "item":
			def.Items = append(def.Items, field.Value)
		case "untargeted_skill":
			def.UntargetedSkills = append(def.UntargetedSkills, field.Value)
		case "targeted_skill":
			def.TargetedSkills = append(def.TargetedSkills, field.Value)
		}
	}
	def.Icon, def.FemaleIcon, _ = ClassIcons(def.InternalName)
	return def
}
