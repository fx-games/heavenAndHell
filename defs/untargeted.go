package defs

import (
	"fmt"
	"io"
	"rpg/recfile"
	"rpg/util"
	"strings"
)

type UntargetedActionDefinition struct {
	Name                string
	InternalName        string
	Icon                int32
	DungeonLevel        int
	ActivationDelay     int
	ActivationCondition string
	DefaultArgs         util.Arguments
	AllowedUsage        ActionUsage
}

func ReadUntargetedDefinitions(file io.Reader, logErr func(string)) map[string]UntargetedActionDefinition {
	records := recfile.Read(file)
	untargetedDefs := make(map[string]UntargetedActionDefinition, len(records))
	for _, record := range records {
		def := NewUntargetedFromRecord(record)
		if def.InternalName == "" {
			logErr(fmt.Sprintf("Untargeted action definition has no internal name: %v", def))
			continue
		}
		if _, exists := untargetedDefs[def.InternalName]; exists {
			logErr(fmt.Sprintf("Duplicate untargeted action definition: %s", def.InternalName))
			continue
		}
		untargetedDefs[def.InternalName] = def
	}
	return untargetedDefs
}

func NewUntargetedFromRecord(record recfile.Record) UntargetedActionDefinition {
	definition := UntargetedActionDefinition{}
	for _, field := range record {
		switch field.Name {
		case "name":
			definition.Name = field.Value
		case "internal_name":
			definition.InternalName = field.Value
		case "icon":
			definition.Icon = field.AsInt32()
		case "dlvl":
			definition.DungeonLevel = field.AsInt()
		case "activation_delay":
			definition.ActivationDelay = field.AsInt()
		case "activation_condition":
			definition.ActivationCondition = field.Value
		case "default_args":
			definition.DefaultArgs = util.NewArgumentsFromString(field.Value)
		case "allowed_usage":
			definition.AllowedUsage = ActionUsageFrom(field.Value)
		}
	}
	return definition

}

func ActionUsageFrom(value string) ActionUsage {
	// parse a bitmask from a string that looks like this "Shrine | Wand"
	// and return the corresponding bitmask
	var result ActionUsage
	for _, part := range strings.Split(value, "|") {
		part = strings.ToLower(strings.TrimSpace(part))
		switch part {
		case "shrine":
			result |= ForShrine
		case "wand":
			result |= ForWand
		}
	}
	return result
}

type ActionUsage int

func (u ActionUsage) Allows(usage ActionUsage) bool {
	// check if the bits from usage are set in u
	return u&usage == usage
}

const ( // bitmask
	ForShrine ActionUsage = 1 << iota
	ForWand
)

