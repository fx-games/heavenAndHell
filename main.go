package main

import (
	"errors"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	_ "image/png"
	"log"
	"os"
	"path"
	"rpg/common"
	"rpg/console"
	"rpg/d1"
	"rpg/defs"
	"rpg/files"
	"rpg/game"
	"rpg/gui"
	"rpg/osr"
	"rpg/renderer"
	"rpg/util"
	"strconv"
	"strings"
)

func main() {
	/*
		cpuProfileFile, err := os.Create("cpu.prof")
		if err != nil {
			panic(err)
		}
		defer cpuProfileFile.Close()

		// start CPU profiling
		if err := pprof.StartCPUProfile(cpuProfileFile); err != nil {
			panic(err)
		}
		defer pprof.StopCPUProfile()
	*/
	if len(os.Args) > 1 {
		argCmd := strings.ToLower(os.Args[1])
		switch argCmd {
		case "debug_tints":
			theGame, _ := createGameAndUIContext()
			debugTints(theGame, os.Args[2:])
		case "loot":
			theGame, _ := createGameAndUIContext()
			genLoot(theGame)
		case "dun_gen":
			maptest()
		case "ascii":
			runGameWithConsole(createGameAndUIContext())
		case "osr":
			osrSim()
		case "validate":
			return
		case "playthrough":
			playthrough()
		}
	} else {
		runGameWithEbiten(createGameAndUIContext())
	}
}

func createGameAndUIContext() (*game.GameInstance, common.UIContext) {
	userConfig := common.NewConfigurationFromRecFile(common.ConfigFile)
	startInfos := common.StartInfo{
		Title:       "RPG",
		ScreenSize:  userConfig.WindowSize,
		TileScale:   4.0,
		LDTKMapFile: "assets/maps.ldtk",
		FontInfos: common.FontInfo{
			TTFFile: userConfig.DefaultFont,
			TTFSize: userConfig.DefaultFontSize,
		},
		UserConfiguration: &userConfig,
		FileSystem:        files.GetFileSystem(),
	}
	theGame := game.NewGameInstance(startInfos, NewDummyUI())
	uiContext := common.UIContext{
		GameState:      theGame.GetWorldState,
		PlayerAction:   theGame.GetActions,
		AudioPlayer:    theGame.GetAudio(),
		StartMenuItems: theGame.StartMenuItems,
		Animator: func() common.Animator {
			return theGame.GetActionBroker()
		},
		GetLog:      theGame.GetLog,
		StartInfos:  startInfos,
		Definitions: theGame.GetGameData(),
	}
	return theGame, uiContext
}

func osrSim() {
	// Helmet / Shield
	// 0 Unarmored +1, +2, +3
	// 1 Light +1, +2, +3
	// 2 Heavy +1, +2, +3
	// 2*5 = 10
	// Armor
	// 0 Unarmored +1, +2, +3
	// 2 Light +1, +2, +3
	// 4 Heavy +1, +2, +3
	// 4+3 = 7
	// => Leather Armor AC = 2
	// => Full Light AC = 4
	// => Full Heavy AC = 8
	// => FULLY ENCHANTED HEAVY AC = 17
	// => Full Enchantment adds 3x3 = 9 to AC

	// Hit Points (1d8 as hit dice)
	// Level 1: 1-8
	// Level 5: 5-40
	// Level 10: 10-80
	// => For a MAX HP and FULLY PROTECTED Fighter it would take 80+17 = 97 damage to kill him in one hit
	// => Anything dealing less than 17 damage in one blow won't even scratch him
	fighterLevelOne := osr.NewAverageFighter("Young Conan")
	fighterLevelOne.SetArmorClass(0) // leather & shield
	fighterLevelFive := osr.NewAverageFighter("Teenage Conan")
	fighterLevelFive.SetArmorClass(4) // chain & shield
	fighterLevelFive.LevelTo(osr.GetFighterSpecs(), 5)

	fighterLevelTen := osr.NewAverageFighter("Conan")
	fighterLevelTen.SetArmorClass(8) // magical plate & shield
	fighterLevelTen.SetDamageDice(util.ParseDice("1d12+2"))
	fighterLevelTen.LevelTo(osr.GetFighterSpecs(), 10)
	// max fighter always wins vs a kobold
	// min fighter still has 20-40% chance to win the fight
	// random fighter usually has a 50-80% chance to win the fight

	kobold := osr.NewMonster(osr.MonsterSpecs{
		Name:       "Kobold",
		AC:         0, // 0 protection
		AttackDice: util.ParseDice("1d6"),
		HitDice:    util.ParseDice("1d8"),
	})

	ghoul := osr.NewMonster(osr.MonsterSpecs{
		Name:       "Ghoul",
		AC:         4, // 1 protection
		AttackDice: util.ParseDice("1d6"),
		HitDice:    util.ParseDice("5d8"),
	})

	manticore := osr.NewMonster(osr.MonsterSpecs{
		Name:       "Manticore",
		AC:         8, // 2 protection
		AttackDice: util.ParseDice("1d12+2"),
		HitDice:    util.ParseDice("10d8"),
	})

	enemies := []*osr.Character{kobold, ghoul, manticore}

	iterations := 10000
	silent := true

	table := make(map[*osr.Character]map[*osr.Character]float64)
	table[fighterLevelOne] = make(map[*osr.Character]float64)
	table[fighterLevelFive] = make(map[*osr.Character]float64)
	table[fighterLevelTen] = make(map[*osr.Character]float64)
	for _, enemy := range enemies {
		srFX := osr.SurvivalRate(fighterLevelOne, enemy, silent, iterations)
		table[fighterLevelOne][enemy] = srFX
	}

	for _, enemy := range enemies {
		srFX := osr.SurvivalRate(fighterLevelFive, enemy, silent, iterations)
		table[fighterLevelFive][enemy] = srFX
	}

	for _, enemy := range enemies {
		srFX := osr.SurvivalRate(fighterLevelTen, enemy, silent, iterations)
		table[fighterLevelTen][enemy] = srFX
	}

	for _, fighter := range []*osr.Character{fighterLevelOne, fighterLevelFive, fighterLevelTen} {
		fmt.Println(fighter.CombatInfo())
	}
	for _, enemy := range enemies {
		fmt.Println(enemy.CombatInfo())
	}
	// print as csv matrix table
	fmt.Println("/," + namesFrom(enemies))
	for _, fighter := range []*osr.Character{fighterLevelOne, fighterLevelFive, fighterLevelTen} {
		fmt.Print(fighter.GetName() + ",")
		for _, enemy := range enemies {
			fmt.Printf("%.2f,", table[fighter][enemy])
		}
		fmt.Println()
	}
}

func namesFrom(enemies []*osr.Character) string {
	names := make([]string, len(enemies))
	for i, enemy := range enemies {
		names[i] = enemy.GetName()
	}
	return strings.Join(names, ",")
}

func genLoot(theGame *game.GameInstance) {
	if len(os.Args) < 3 {
		fmt.Println("usage: loot <dungeon_level>")
		return
	}
	dungeonLevel, _ := strconv.Atoi(os.Args[2])
	items := theGame.GenerateLoot(dungeonLevel, 10)
	for _, item := range items {
		fmt.Println(item.BandStyleName())
	}
}

// maptest prints D1 levels: dun_gen [dungeon level] [seed].
func maptest() {
	dlvl, seed := 1, int64(42)
	if len(os.Args) > 2 {
		dlvl, _ = strconv.Atoi(os.Args[2])
	}
	if len(os.Args) > 3 {
		seed, _ = strconv.ParseInt(os.Args[3], 10, 64)
	}
	d1Data, err := d1.Load(files.GetFileSystem())
	if err != nil {
		log.Fatal(err)
	}
	seeds := d1.LevelSeeds(int32(seed))
	quests := d1.InitQuests(seeds)
	gen := d1.NewGenerator(d1Data, int32(seed))
	lvl, err := gen.CreateLevel(&quests, seeds, dlvl, d1.SLNone, d1.EntryMain)
	if err != nil {
		log.Fatal(err)
	}
	for y := 0; y < d1.LevelSize; y++ {
		for x := 0; x < d1.LevelSize; x++ {
			if lvl.CellSolid(x, y) {
				fmt.Print("#")
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println()
	}
}

func runGameWithConsole(theGame *game.GameInstance, context common.UIContext) {
	theGame.SetConsoleMode(true)
	con := console.NewConsole(context)
	theGame.SetAndInitUserInterface(con)

	if err := con.Run(theGame.ShowStartMenu); err != nil {
		log.Fatal(err)
	}
	theGame.SaveGame() // decision 11: autosave on quit

}

// runGameWithEbiten will start the infinitely running game loop
// game will receive the update and draw calls
func runGameWithEbiten(theGame *game.GameInstance, context common.UIContext) {
	startInfos := context.StartInfos
	tileRenderer := renderer.NewTileRenderer(startInfos.TileScale)
	tileRenderer.SetColorFromName(defs.ColorFromName)
	tileRenderer.SetDefaultAtlas(theGame.GetWorldState().GetWorldTexture())
	tileRenderer.SetDefaultColors(defs.DarkTextColor, color.Black)
	tileRenderer.SetWhiteTile(defs.IconWhite)

	tileMapAnimations := map[int32]gui.TilemapAnimInfo{
		28:                       {FrameCount: 2, DelayInSeconds: 1.5},
		32:                       {FrameCount: 2, DelayInSeconds: 0.2},
		defs.IconEffectFireSmall: {FrameCount: 2, DelayInSeconds: 0.2},
	}
	uiController := gui.NewUIController(tileRenderer, context, startInfos.UserConfiguration)
	uiController.SetTileMapAnimations(tileMapAnimations)
	uiController.SetTTFFont(files.MustOpen(startInfos.FontInfos.TTFFile), startInfos.FontInfos.TTFSize)
	uiController.SetAsciiAtlas(renderer.NewTextureAtlas(path.Join("assets", "Bm437_IBM_CGA.png"), 8, 8))
	theGame.SetAndInitUserInterface(uiController)
	theGame.ShowStartMenu()

	screenSize := uiController.GetDeviceIndependentScreenSize()

	ebiten.SetWindowTitle("Lux Aeterna")
	ebiten.SetWindowSize(screenSize.X, screenSize.Y)
	if startInfos.UserConfiguration.Fullscreen {
		uiController.SetFullscreen(true)
	}
	ebiten.SetScreenClearedEveryFrame(true)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSizeLimits(640, 480, -1, -1)
	ebiten.SetWindowClosingHandled(false)

	err := ebiten.RunGameWithOptions(uiController, &ebiten.RunGameOptions{
		GraphicsLibrary: ebiten.GraphicsLibraryOpenGL,
	})
	theGame.SaveGame() // decision 11: autosave on quit
	if err != nil && !errors.Is(err, ebiten.Termination) {
		log.Fatal(err)
	}
}
