package defs

import (
	"math/rand"
	"rpg/recfile"
	"strings"
)

type EnemyDefinition struct {
	Icon               int32
	DLvl               int
	MaxLevel           int
	OutOfDepthChance   float64
	MinLevelForOoD     int
	Name               string
	InternalName       string
	BaseSpeed          int
	ResourceHealth     int
	Stats              map[string]int
	Equipment          []string
	DeathStringFormats []string

	TargetedSkills   []string
	UntargetedSkills []string
	GroupSpawn       []string

	AggroRange int
}

func NewEnemyDefinitionFromRecord(record recfile.Record) EnemyDefinition {
	enemyDefinition := EnemyDefinition{
		Stats: make(map[string]int),
	}
	for _, field := range record {

		// special case for stats
		if strings.HasPrefix(field.Name, "stats_") {
			statName := strings.TrimPrefix(field.Name, "stats_")
			enemyDefinition.Stats[statName] = field.AsInt()
			continue
		}
		switch field.Name {
		case "icon":
			enemyDefinition.Icon = field.AsInt32()
		case "dlvl":
			enemyDefinition.DLvl = field.AsInt()
		case "name":
			enemyDefinition.Name = field.Value
		case "internal_name":
			enemyDefinition.InternalName = field.Value
		case "base_speed":
			enemyDefinition.BaseSpeed = field.AsInt()
		case "resource_health":
			enemyDefinition.ResourceHealth = field.AsInt()
		case "equipment":
			enemyDefinition.Equipment = append(enemyDefinition.Equipment, field.Value)
		case "death_string_formats":
			enemyDefinition.DeathStringFormats = append(enemyDefinition.DeathStringFormats, field.Value)
		case "targeted_skill":
			enemyDefinition.TargetedSkills = append(enemyDefinition.TargetedSkills, field.Value)
		case "untargeted_skill":
			enemyDefinition.UntargetedSkills = append(enemyDefinition.UntargetedSkills, field.Value)
		case "group_spawn":
			enemyDefinition.GroupSpawn = append(enemyDefinition.GroupSpawn, field.Value)
		case "aggro_range":
			enemyDefinition.AggroRange = field.AsInt()
		}
	}
	return enemyDefinition
}

func (d EnemyDefinition) CanAppearAt(randomSource *rand.Rand, dungeonLevel int) bool {
	// reject early
	if d.MaxLevel > 0 && dungeonLevel > d.MaxLevel {
		return false
	}
	regularAppearance := dungeonLevel >= d.DLvl

	// apply out of depth chance
	oodAppearance := dungeonLevel >= d.MinLevelForOoD && randomSource.Float64() < d.OutOfDepthChance

	return regularAppearance || oodAppearance
}
