package game

import (
    "fmt"
    "io"
	"math/rand"
	"rpg/actions"
	"rpg/common"
	"rpg/recfile"
	"rpg/stats"
	"rpg/util"
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
	BaseHealth         int
	Stats              map[string]int
	Equipment          []string
	DeathStringFormats []string

	TargetedSkills     []string
	UntargetedSkills   []string
	GroupSpawn 		   []string

	AggroRange         int
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

func (g *GameInstance) NewRandomEnemy(dungeonLevel int) (*common.Actor, []EnemyDefinition) {
	// get defs
	enemyDefinitions := g.enemyDefinitions

	// filter for dungeon level
	var filteredEnemies []EnemyDefinition
    for _, enemyDef := range enemyDefinitions {
		if !enemyDef.CanAppearAt(g.randomSource, dungeonLevel) {
			continue
		}
		filteredEnemies = append(filteredEnemies, enemyDef)
	}

    if len(filteredEnemies) == 0 {
        g.ErrLog(fmt.Sprintf("no enemies found for dungeon level %d", dungeonLevel))
		return nil, nil
    }
	// pick random enemy definition
	enemyDef := filteredEnemies[g.randomSource.Intn(len(filteredEnemies))]

	// wants to spawn as a group?
	var groupDefs []EnemyDefinition
	if len(enemyDef.GroupSpawn) > 0 {
		for _, otherDef := range enemyDef.GroupSpawn {
			otherDefName := otherDef
			spawnCount := 1
			if util.LooksLikeAFunction(otherDef) {
				oDefName, args := util.GetNameAndArgs(otherDef)
				otherDefName = oDefName
				spawnCount = stats.NewIntervalFromString(args.Get(0)).LinearRandom()
			}
			for i := 0; i < spawnCount; i++ {
				if otherEnemyDef, exists := enemyDefinitions[otherDefName]; exists {
					groupDefs = append(groupDefs, otherEnemyDef)
				}
			}
		}
	}
	// create enemy
	return g.NewEnemyFromDefinition(enemyDef, dungeonLevel), groupDefs
}

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
		case "base_health":
			enemyDefinition.BaseHealth = field.AsInt()
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

func (g *GameInstance) NewEnemyFromDefinition(enemyDef EnemyDefinition, dungeonLevel int) *common.Actor {
	actor := common.NewActor(enemyDef.Icon)
	//index := len(emptyMap.Actors())
	actor.SetName(enemyDef.Name)
	actor.SetInternalName(enemyDef.InternalName)
	actor.SetDeathStringFormats(enemyDef.DeathStringFormats)
	actor.SetHostile()
	actor.SetBaseSpeed(enemyDef.BaseSpeed)
	actorStats := actor.GetStats()

	actorStats.SetAttribute(stats.MaxHealth, enemyDef.BaseHealth)
	actorStats.GetRessource(stats.Health).SetToMax()

	for statName, statValue := range enemyDef.Stats {
		actorStats.SetAttribute(stats.StatFromString(statName), statValue)
	}

	for _, itemName := range enemyDef.Equipment {
		item := g.NewItemFromDefinitionName(itemName, dungeonLevel)
		if item == nil {
			g.ErrLog(fmt.Sprintf("could not find item definition %s", itemName))
			continue
		}
		g.giveAndEquipItemToActor(actor, item)
	}

	var targetedSkills []*common.TargetedAction
	for _, skillName := range enemyDef.TargetedSkills {
		skillAction := g.NewTargetedActionFromName(skillName)
		if skillAction != nil {
			targetedSkills = append(targetedSkills, skillAction)
		}
	}
	actor.SetTargetedSkills(targetedSkills)

	var untargetedSkills []*common.UntargetedAction
	for _, skillName := range enemyDef.UntargetedSkills {
		skillAction := g.NewUntargetedActionFromName(skillName)
		if skillAction != nil {
			untargetedSkills = append(untargetedSkills, skillAction)
		}
	}
	actor.SetUntargetedSkills(untargetedSkills)
	actor.SetAggroRange(enemyDef.AggroRange)

	return actor
}


func (g *GameInstance) giveAndEquipItemToActor(actor *common.Actor, item *GameItem) {
	actor.GetInventory().Add(item)
	g.actionBroker.Execute(actions.NewEquipItemAction(actions.AbstractCause("Equip"), actor, item))
}
