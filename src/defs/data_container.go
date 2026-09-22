package defs

import (
	"log/slog"
	"path"
	"rpg/files"
)

type DataContainer struct {
	ClassDefinitions []LoadoutDefinition
}

// LoadGameData loads the remaining H&H definitions; monsters, items, loot and stores come from package d1.
func LoadGameData(logErr func(err string)) DataContainer {
	file, _ := files.Open(path.Join("assets", "data", "classes.rec"))
	classDefs := ReadClassDefinitions(file)
	slog.Debug("loaded class definitions", "count", len(classDefs))
	file.Close()

	return DataContainer{
		ClassDefinitions: classDefs,
	}
}
