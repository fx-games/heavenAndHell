package defs

import (
	"log/slog"
	"rpg/files"
)

type DataContainer struct {
	ClassDefinitions []LoadoutDefinition
}

// LoadGameData loads the remaining H&H definitions; monsters, items, loot and stores come from package d1.
func LoadGameData(logErr func(err string)) DataContainer {
	classDefs, err := ReadClasses(files.GetFileSystem())
	if err != nil {
		logErr(err.Error())
	}
	slog.Debug("loaded class definitions", "count", len(classDefs))
	return DataContainer{ClassDefinitions: classDefs}
}
