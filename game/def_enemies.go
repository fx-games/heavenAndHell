package game

import (
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/stats"
)
/*

Starting Enemy
HP: 3-10
AC: 0-1
Dmg.: 1-5

Mid Game Enemy
HP: 10-500
AC: 2-50
Dmg.: 5-30

End Boss Enemy
HP: 1000
AC: 60-100
Dmg.: 30-60

 */
func (g *GameInstance) addRandomEnemy(emptyMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], dungeonLevel int, pos geometry.Point) {
	source := g.randomSource

	createFunctionsPerLevel := map[int][]func(dungeonLevel int) *common.Actor{
		1: {
			g.NewBat,
			g.NewSkeleton,
			g.NewFallen,
		},
		2: {
			g.NewBat,
			g.NewSkeleton,
			g.NewSkeletonWarrior,
			g.NewSkeletonArcher,
			g.NewFallen,
		},
		3: {
			g.NewBat,
			g.NewSkeleton,
			g.NewSkeletonWarrior,
			g.NewSkeletonArcher,
			g.NewSkeletonSorcerer,
			g.NewSkeletonCaptain,
		},
		4: {
			g.NewSkeletonWarrior,
			g.NewSkeletonArcher,
			g.NewSkeletonSorcerer,
			g.NewSkeletonCaptain,
			g.NewFallen,
		},
		5: {
			g.NewFallen,
			g.NewCaveling,
			g.NewCavelingWithWeapon,
		},
		6: {
			g.NewCaveling,
			g.NewCavelingWithWeapon,
			g.NewCavelingWithAxes,
			g.NewCavelingWithBigAxe,
			g.NewCaveBear,
		},
		7: {
			g.NewFiend,
			g.NewFiendHorned,
			g.NewFiendHornedWithKnife,
		},
		8: {
			g.NewFiendHornedWithGlaive,
			g.NewFiendHornedBig,
			g.NewFiendHornedWithWhip,
		},
		9: {
			g.NewRogueBat,
			g.NewRogueKestrel,
			g.NewRogueJackal,
		},
		10: {
			g.NewRogueKestrel,
			g.NewRogueJackal,
			g.NewRogueOrc,
			g.NewRogueSnake,
			g.NewRogueEmu,
		},
	}
	createFunctions := createFunctionsPerLevel[dungeonLevel]
	if createFunctions == nil {
		createFunctions = createFunctionsPerLevel[1]
	}

	creationFunction := createFunctions[source.Intn(len(createFunctions))]
	actor := creationFunction(dungeonLevel)
	actor.SetDrawPosition(pos.ToPointF())
	emptyMap.AddActor(actor, pos)
}

func (g *GameInstance) NewBat(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterBat)
	actor.SetName("a bat")
	actor.SetInternalName("bat")
	actor.SetHostile()
	actor.SetBaseSpeed(12)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 15)
	actorStats.SetAttribute(stats.Strength, 1)
	return actor
}

func (g *GameInstance) NewSkeleton(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterSkeleton)
	//index := len(emptyMap.Actors())
	actor.SetName("a skeleton")
	actor.SetInternalName("skeleton")
	actor.SetDeathStringFormats([]string{"%s crumbles to dust.", "%s falls apart."})
	actor.SetHostile()
	actor.SetBaseSpeed(9)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 10)
	actorStats.SetAttribute(stats.Strength, 2)
	return actor
}

func (g *GameInstance) NewSkeletonArcher(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterSkeletonArcher)
	//index := len(emptyMap.Actors())
	actor.SetName("a skeleton archer")
	actor.SetInternalName("skeleton_archer")
	actor.SetDeathStringFormats([]string{"%s crumbles to dust.", "%s falls apart."})
	actor.SetHostile()
	actor.SetBaseSpeed(9)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 12)
	actorStats.SetAttribute(stats.Strength, 2)
	actorStats.SetAttribute(stats.Dexterity, 6)

	weapon := NewBow(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(weapon)
	actor.GetEquipment().Equip(weapon)
	return actor
}

func (g *GameInstance) NewSkeletonWarrior(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterSkeletonWarrior)
	//index := len(emptyMap.Actors())
	actor.SetName("a skeleton warrior")
	actor.SetInternalName("skeleton_warrior")
	actor.SetDeathStringFormats([]string{"%s crumbles to dust.", "%s falls apart."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 12)
	actorStats.SetAttribute(stats.Strength, 10)
	actorStats.SetAttribute(stats.Dexterity, 6)

	weapon := NewSword(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(weapon)
	actor.GetEquipment().Equip(weapon)
	return actor
}
func (g *GameInstance) NewSkeletonCaptain(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterSkeletonCaptain)
	//index := len(emptyMap.Actors())
	actor.SetName("a skeleton captain")
	actor.SetInternalName("skeleton_captain")
	actor.SetDeathStringFormats([]string{"%s crumbles to dust.", "%s falls apart."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)

	weapon := NewSword(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(weapon)
	actor.GetEquipment().Equip(weapon)

	helmet := NewHelmet(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(helmet)
	actor.GetEquipment().Equip(helmet)

	return actor
}

func (g *GameInstance) NewSkeletonSorcerer(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterSkeletonSorcerer)
	//index := len(emptyMap.Actors())
	actor.SetName("a skeleton sorcerer")
	actor.SetInternalName("skeleton_sorcerer")
	actor.SetDeathStringFormats([]string{"%s crumbles to dust.", "%s falls apart."})
	actor.SetHostile()
	actor.SetBaseSpeed(9)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 15)
	actorStats.SetAttribute(stats.Strength, 2)
	actorStats.SetAttribute(stats.Dexterity, 6)
	actorStats.SetAttribute(stats.Intelligence, 6)

	pushAction := g.NewTargetedActionFromName(ActionNameMindPush)
	actor.SetTargetedSkills([]*common.TargetedAction{pushAction})
	return actor
}


func (g *GameInstance) NewCaveling(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterCaveling)
	//index := len(emptyMap.Actors())
	actor.SetName("a caveling")
	actor.SetInternalName("caveling")
	actor.SetDeathStringFormats([]string{"%s dies a bloody death.", "%s bites the dust."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 8)
	actorStats.SetAttribute(stats.Strength, 6)
	return actor
}
func (g *GameInstance) NewCavelingWithWeapon(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterCavelingWithWeapon)
	//index := len(emptyMap.Actors())
	actor.SetName("an armed caveling")
	actor.SetInternalName("caveling_armed")
	actor.SetDeathStringFormats([]string{"%s dies a bloody death.", "%s bites the dust."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 8)
	actorStats.SetAttribute(stats.Strength, 6)

	weapon := NewSword(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(weapon)
	actor.GetEquipment().Equip(weapon)
	return actor
}

func (g *GameInstance) NewCavelingWithAxes(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterCaveImpWithAxes)
	//index := len(emptyMap.Actors())
	actor.SetName("an axe caveling")
	actor.SetInternalName("caveling_axe")
	actor.SetDeathStringFormats([]string{"%s dies a bloody death.", "%s bites the dust."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 8)
	actorStats.SetAttribute(stats.Strength, 6)

	weapon := NewSword(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(weapon)
	actor.GetEquipment().Equip(weapon)
	return actor
}
func (g *GameInstance) NewCavelingWithBigAxe(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterCaveImpWithBigAxe)
	//index := len(emptyMap.Actors())
	actor.SetName("an big axe caveling")
	actor.SetInternalName("caveling_big_axe")
	actor.SetDeathStringFormats([]string{"%s dies a bloody death.", "%s bites the dust."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 8)
	actorStats.SetAttribute(stats.Strength, 6)

	weapon := NewSword(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(weapon)
	actor.GetEquipment().Equip(weapon)
	return actor
}

func (g *GameInstance) NewCaveBear(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterCaveBear)
	//index := len(emptyMap.Actors())
	actor.SetName("an cave bear")
	actor.SetInternalName("caveling_bear")
	actor.SetDeathStringFormats([]string{"%s dies a bloody death.", "%s bites the dust."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 8)
	actorStats.SetAttribute(stats.Strength, 14)
	return actor
}




func (g *GameInstance) NewFallen(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterFallen)
	//index := len(emptyMap.Actors())
	actor.SetName("an imp")
	actor.SetInternalName("fallen")
	actor.SetDeathStringFormats([]string{"%s stops existing.", "%s bursts into pieces."})
	actor.SetHostile()
	actor.SetBaseSpeed(8)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 8)
	actorStats.SetAttribute(stats.Strength, 2)
	return actor
}


func (g *GameInstance) NewFallenWarrior(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterFallenWarrior)
	//index := len(emptyMap.Actors())
	actor.SetName("a fallen warrior")
	actor.SetInternalName("fallen_warrior")
	actor.SetDeathStringFormats([]string{"%s stops existing.", "%s bursts into pieces."})
	actor.SetHostile()
	actor.SetBaseSpeed(8)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 12)
	actorStats.SetAttribute(stats.Strength, 8)
	actorStats.SetAttribute(stats.Dexterity, 6)

	weapon := NewSword(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(weapon)
	actor.GetEquipment().Equip(weapon)
	return actor
}

func (g *GameInstance) NewFallenArcher(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterFallenArcher)
	//index := len(emptyMap.Actors())
	actor.SetName("a fallen warrior")
	actor.SetInternalName("fallen_warrior")
	actor.SetDeathStringFormats([]string{"%s stops existing.", "%s bursts into pieces."})
	actor.SetHostile()
	actor.SetBaseSpeed(8)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 12)
	actorStats.SetAttribute(stats.Strength, 8)
	actorStats.SetAttribute(stats.Dexterity, 8)


	weapon := NewBow(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(weapon)
	actor.GetEquipment().Equip(weapon)
	return actor
}

func (g *GameInstance) NewFallenShaman(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterFallenShaman)
	//index := len(emptyMap.Actors())
	actor.SetName("a fallen shaman")
	actor.SetInternalName("fallen_shaman")
	actor.SetDeathStringFormats([]string{"%s stops existing.", "%s bursts into pieces."})
	actor.SetHostile()
	actor.SetBaseSpeed(8)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.Experience, 12)
	actorStats.SetAttribute(stats.Strength, 8)
	actorStats.SetAttribute(stats.Dexterity, 8)
	actorStats.SetAttribute(stats.Intelligence, 8)

	pushAction := g.NewTargetedActionFromName(ActionNameFireBresen)
	actor.SetTargetedSkills([]*common.TargetedAction{pushAction})
	return actor
}


func (g *GameInstance) NewFiend(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterFiend)
	//index := len(emptyMap.Actors())
	actor.SetName("a fiend")
	actor.SetInternalName("fiend")
	actor.SetDeathStringFormats([]string{"%s bursts into flames.", "%s violently explodes."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)

	return actor
}

func (g *GameInstance) NewFiendHorned(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterHornedFiend)
	//index := len(emptyMap.Actors())
	actor.SetName("a horned fiend")
	actor.SetInternalName("fiend_horned")
	actor.SetDeathStringFormats([]string{"%s bursts into flames.", "%s violently explodes."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)

	return actor
}

func (g *GameInstance) NewFiendHornedWithKnife(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterHornedWithKnife)
	//index := len(emptyMap.Actors())
	actor.SetName("a armed fiend")
	actor.SetInternalName("fiend_armed")
	actor.SetDeathStringFormats([]string{"%s bursts into flames.", "%s violently explodes."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)


	weapon := NewSword(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(weapon)
	actor.GetEquipment().Equip(weapon)

	return actor
}

func (g *GameInstance) NewFiendHornedWithWhip(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterHornedWithWhip)
	//index := len(emptyMap.Actors())
	actor.SetName("a whipper fiend")
	actor.SetInternalName("fiend_whip")
	actor.SetDeathStringFormats([]string{"%s bursts into flames.", "%s violently explodes."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)


	weapon := NewSword(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(weapon)
	actor.GetEquipment().Equip(weapon)

	return actor
}

func (g *GameInstance) NewFiendHornedWithGlaive(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterHornedWithGlaive)
	//index := len(emptyMap.Actors())
	actor.SetName("a glaive fiend")
	actor.SetInternalName("fiend_glaive")
	actor.SetDeathStringFormats([]string{"%s bursts into flames.", "%s violently explodes."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)


	weapon := NewSword(g.GetWorldState(), g.randomSource, dungeonLevel)
	actor.GetInventory().Add(weapon)
	actor.GetEquipment().Equip(weapon)

	return actor
}

func (g *GameInstance) NewFiendHornedBig(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconMonsterHornedBig)
	//index := len(emptyMap.Actors())
	actor.SetName("a big fiend")
	actor.SetInternalName("fiend_big")
	actor.SetDeathStringFormats([]string{"%s bursts into flames.", "%s violently explodes."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)


	return actor
}

func (g *GameInstance) NewRogueKestrel(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconRogueKestrel)
	//index := len(emptyMap.Actors())
	actor.SetName("a kestrel")
	actor.SetInternalName("rogue_kestrel")
	actor.SetDeathStringFormats([]string{"%s dies."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actor.SetCorpseIcon(common.IconRogueCorpse)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)


	return actor
}

func (g *GameInstance) NewRogueBat(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconRogueBat)
	//index := len(emptyMap.Actors())
	actor.SetName("a bat")
	actor.SetInternalName("rogue_bat")
	actor.SetDeathStringFormats([]string{"%s dies."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actor.SetCorpseIcon(common.IconRogueCorpse)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)


	return actor
}

func (g *GameInstance) NewRogueEmu(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconRogueEmu)
	//index := len(emptyMap.Actors())
	actor.SetName("an emu")
	actor.SetInternalName("rogue_emu")
	actor.SetDeathStringFormats([]string{"%s dies."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actor.SetCorpseIcon(common.IconRogueCorpse)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)


	return actor
}

func (g *GameInstance) NewRogueJackal(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconRogueJackal)
	//index := len(emptyMap.Actors())
	actor.SetName("a jackal")
	actor.SetInternalName("rogue_jackal")
	actor.SetDeathStringFormats([]string{"%s dies."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actor.SetCorpseIcon(common.IconRogueCorpse)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)


	return actor
}

func (g *GameInstance) NewRogueOrc(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconRogueOrc)
	//index := len(emptyMap.Actors())
	actor.SetName("an orc")
	actor.SetInternalName("rogue_orc")
	actor.SetDeathStringFormats([]string{"%s dies."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actor.SetCorpseIcon(common.IconRogueCorpse)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)


	return actor
}

func (g *GameInstance) NewRogueSnake(dungeonLevel int) *common.Actor {
	actor := common.NewActor(common.IconRogueSnake)
	//index := len(emptyMap.Actors())
	actor.SetName("a snake")
	actor.SetInternalName("rogue_snake")
	actor.SetDeathStringFormats([]string{"%s dies."})
	actor.SetHostile()
	actor.SetBaseSpeed(10)
	actor.SetCorpseIcon(common.IconRogueCorpse)
	actorStats := actor.GetStats()
	actorStats.SetAttribute(stats.MaxHealth, 26)
	actorStats.GetRessource(stats.Health).SetToMax()

	actorStats.SetAttribute(stats.Experience, 20)
	actorStats.SetAttribute(stats.Strength, 12)
	actorStats.SetAttribute(stats.Dexterity, 12)


	return actor
}