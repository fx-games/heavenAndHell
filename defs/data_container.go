package defs

import (
	"fmt"
	"math/rand"
	"path"
	"rpg/files"
	"rpg/stats"
)

type DataContainer struct {
	EnemyDefinitions       map[string]EnemyDefinition
	UniqueEnemyDefinitions map[string]EnemyDefinition

	Modifiers map[string]func(source *rand.Rand, level int) *stats.Modifier

	TargetedDefinitions   map[string]TargetedActionDefinition
	UntargetedDefinitions map[string]UntargetedActionDefinition

	ClassDefinitions []LoadoutDefinition
}

// LoadGameData loads the remaining H&H definitions; items, loot and stores come from package d1.
func LoadGameData(logErr func(err string)) DataContainer {
	enemyDir := path.Join("assets", "data", "enemies")
	effectsDir := path.Join("assets", "data", "effects")
	otherDir := path.Join("assets", "data")

	file, _ := files.Open(path.Join(enemyDir, "enemies.rec"))
	enemyDefs := ReadEnemyDefinitions(file, logErr)
	println(fmt.Sprintf("Loaded %d enemy definitions", len(enemyDefs)))
	file.Close()

	file, _ = files.Open(path.Join(enemyDir, "unique_enemies.rec"))
	uniqueEnemyDefs := ReadEnemyDefinitions(file, logErr)
	println(fmt.Sprintf("Loaded %d unique enemy definitions", len(uniqueEnemyDefs)))
	file.Close()

	file, _ = files.Open(path.Join(effectsDir, "untargeted.rec"))
	untargetedDefs := ReadUntargetedDefinitions(file, logErr)
	println(fmt.Sprintf("Loaded %d untargeted effect definitions", len(untargetedDefs)))
	file.Close()

	file, _ = files.Open(path.Join(effectsDir, "targeted.rec"))
	targetedDefs := ReadTargetedDefinitions(file, logErr)
	println(fmt.Sprintf("Loaded %d targeted effect definitions", len(targetedDefs)))
	file.Close()

	file, _ = files.Open(path.Join(otherDir, "classes.rec"))
	classDefs := ReadClassDefinitions(file)
	println(fmt.Sprintf("Loaded %d class definitions", len(classDefs)))
	file.Close()

	return DataContainer{
		EnemyDefinitions:       enemyDefs,
		UniqueEnemyDefinitions: uniqueEnemyDefs,
		TargetedDefinitions:    targetedDefs,
		UntargetedDefinitions:  untargetedDefs,
		ClassDefinitions:       classDefs,
		Modifiers:              GetAllModifiers(),
	}
}
