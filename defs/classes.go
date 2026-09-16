package defs

import (
	"io"
	"rpg/recfile"
	"strings"
)

// FemaleIconByClass maps a class' internal_name to its female icon; gender only swaps the sprite.
var FemaleIconByClass = map[string]int32{
	"warrior":   IconCharacterWarriorFemale,
	"rogue":     IconCharacterRogueFemale,
	"sorcerer":  IconCharacterSorcererFemale,
	"monk":      IconCharacterMonkFemale,
	"bard":      IconCharacterBardFemale,
	"barbarian": IconCharacterBarbarianFemale,
}

// MaleIconByClass maps a class' internal_name to its male icon, kept in sync with actors.rec by
// ./remap_actors.sh instead of the frozen "icon" numbers classes.rec used to carry.
var MaleIconByClass = map[string]int32{
	"warrior":   IconCharacterWarriorMale,
	"rogue":     IconCharacterRogueMale,
	"sorcerer":  IconCharacterSorcererMale,
	"monk":      IconCharacterMonkMale,
	"bard":      IconCharacterBardMale,
	"barbarian": IconCharacterBarbarianMale,
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
}

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
	def.Icon = MaleIconByClass[def.InternalName]
	return def
}
