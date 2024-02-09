package defs

import (
	"io"
	"rpg/recfile"
	"rpg/stats"
)

type VendorDefinition struct {
	InternalName   string
	Purse          int
	ItemCount      stats.Interval
	TurnoverItems  int
	SometimesCount stats.Interval
	Always         []string
	Sometimes      []string
}

func ReadVendorDefinitions(file io.Reader) map[string]VendorDefinition {
	records := recfile.Read(file)
	targetedDefs := make(map[string]VendorDefinition, len(records))
	for _, record := range records {
		def := NewVendorFromRecord(record)
		targetedDefs[def.InternalName] = def
	}
	return targetedDefs
}

func NewVendorFromRecord(record recfile.Record) VendorDefinition {
	def := VendorDefinition{}
	for _, field := range record {
		switch field.Name {
		case "internal_name":
			def.InternalName = field.Value
		case "purse":
			def.Purse = field.AsInt()
		case "item_count":
			def.ItemCount = stats.NewIntervalFromString(field.Value)
		case "turnover_items":
			def.TurnoverItems = field.AsInt()
		case "sometimes_count":
			def.SometimesCount = stats.NewIntervalFromString(field.Value)
		case "always":
			def.Always = append(def.Always, field.Value)
		case "sometimes":
			def.Sometimes = append(def.Sometimes, field.Value)
		}
	}
	return def
}
