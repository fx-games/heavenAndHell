package defs

import (
	"fmt"
	"io"
	"rpg/recfile"
)

func ReadEnemyDefinitions(file io.Reader, logErr func(string)) map[string]EnemyDefinition {
	records := recfile.Read(file)
	enemyDefs := make(map[string]EnemyDefinition, len(records))
	for _, record := range records {
		def := NewEnemyDefinitionFromRecord(record)
		if def.InternalName == "" {
			logErr(fmt.Sprintf("Enemy definition has no internal name: %v", def))
			continue
		}
		if _, exists := enemyDefs[def.InternalName]; exists {
			logErr(fmt.Sprintf("Duplicate enemy definition: %s", def.InternalName))
			continue
		}
		enemyDefs[def.InternalName] = def
	}
	return enemyDefs
}

