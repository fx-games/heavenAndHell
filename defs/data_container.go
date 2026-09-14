package defs

import (
	"fmt"
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
	println(fmt.Sprintf("Loaded %d class definitions", len(classDefs)))
	file.Close()

	return DataContainer{
		ClassDefinitions: classDefs,
	}
}
