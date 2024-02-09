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

func ReadItemDefinitions(file io.Reader, logErr func(errString string)) map[string]ItemDefinition {
	records := recfile.Read(file)
	itemDefinitions := make(map[string]ItemDefinition, len(records))
	for _, record := range records {
		definition := NewItemDefinitionFromRecord(record)
		if definition.InternalName == "" {
			logErr(fmt.Sprintf("Item definition has no internal name: %v", definition))
			continue
		}
		if _, exists := itemDefinitions[definition.InternalName]; exists {
			logErr(fmt.Sprintf("Duplicate item definition: %s", definition.InternalName))
			continue
		}
		if definition.Armor.ArmorClass.NotZero() && definition.Armor.ArmorType == ArmorTypeNone {
			logErr(fmt.Sprintf("Armor definition has invalid armor type: %v", definition))
			continue
		}
		if definition.Weapon.Damage.NotZero() && definition.Weapon.WeaponType == WeaponTypeNone {
			logErr(fmt.Sprintf("Weapon definition has invalid weapon type: %v", definition))
			continue
		}
		itemDefinitions[definition.InternalName] = definition
	}
	return itemDefinitions
}
