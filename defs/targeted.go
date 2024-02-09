package defs

import (
	"fmt"
	"io"
	"rpg/recfile"
)

type TargetedActionDefinition struct {
	Name         string
	InternalName string
	Icon         int32
	DungeonLevel int

	HitZone     string
	Targeting   string
	OnHitEffect string
}

func ReadTargetedDefinitions(file io.Reader, logErr func(string)) map[string]TargetedActionDefinition {
	records := recfile.Read(file)
	targetedDefs := make(map[string]TargetedActionDefinition, len(records))
	for _, record := range records {
		def := NewTargetedFromRecord(record)
		if def.InternalName == "" {
			logErr(fmt.Sprintf("Targeted action definition has no internal name: %v", def))
			continue
		}
		if _, exists := targetedDefs[def.InternalName]; exists {
			logErr(fmt.Sprintf("Duplicate targeted action definition: %s", def.InternalName))
			continue
		}
		targetedDefs[def.InternalName] = def
	}
	return targetedDefs
}

func NewTargetedFromRecord(record recfile.Record) TargetedActionDefinition {
	def := TargetedActionDefinition{}
	for _, field := range record {
		switch field.Name {
		case "name":
			def.Name = field.Value
		case "internal_name":
			def.InternalName = field.Value
		case "icon":
			def.Icon = field.AsInt32()
		case "dlvl":
			def.DungeonLevel = field.AsInt()
		case "hit_zone":
			def.HitZone = field.Value
		case "targeting":
			def.Targeting = field.Value
		case "effect":
			def.OnHitEffect = field.Value
		}
	}
	return def
}
