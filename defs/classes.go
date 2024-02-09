package defs

import (
	"io"
	"rpg/recfile"
	"strings"
)

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
		case "icon":
			def.Icon = field.AsInt32()
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
	return def
}
