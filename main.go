package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	_ "image/png"
	"log"
	"math/rand"
	"os"
	"rpg/common"
	"rpg/console"
	"rpg/defs"
	"rpg/dungen"
	"rpg/game"
	"rpg/gui"
	"rpg/ldtk"
	"rpg/renderer"
	"rpg/util"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"
)

func main() {

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

	userConfig := common.NewConfigurationFromRecFile("config.rec")
	startInfos := game.StartInfo{
		Title:       "RPG",
		ScreenSize:  userConfig.WindowSize,
		TileScale:   4.0,
		LDTKMapFile: "assets/maps.ldtk",
		FontInfos: game.FontInfo{
			TTFFile: userConfig.DefaultFont,
			TTFSize: userConfig.DefaultFontSize,
		},
		UserConfiguration: userConfig,
	}
	theGame := game.NewGameInstance(startInfos, NewDummyUI())
	if len(os.Args) > 1 {
		argCmd := strings.ToLower(os.Args[1])
		switch argCmd {
		case "loot":
			genLoot(theGame)
		case "ascii":
			runGameWithConsole(theGame)
		case "list":
			dumpGameData(theGame.GetGameData())
		case "create":
			startCreator(theGame.GetGameData())
		case "validate":
			return
		}
	} else {
		tileRenderer := renderer.NewTileRenderer(startInfos.TileScale)
		tileRenderer.SetColorFromName(common.ColorFromName)
		tileRenderer.SetDefaultAtlas(theGame.GetWorldState().GetWorldTexture())
		tileRenderer.SetDefaultColors(common.DarkTextColor, color.Black)
		tileRenderer.SetWhiteTile(defs.IconWhite)
		uiContext := gui.UIContext{
			GameState:    theGame.GetWorldState(),
			PlayerAction: theGame.GetActions(),
			AudioPlayer:  theGame.GetAudio(),
			ResetGame:    theGame.ResetGame,
			Animator:     theGame.GetActionBroker(),
			DebugActions: common.DebugActions{},
		}
		tileMapAnimations := map[int32]gui.TilemapAnimInfo{
			28:                       {FrameCount: 2, DelayInSeconds: 1.5},
			32:                       {FrameCount: 2, DelayInSeconds: 0.2},
			defs.IconEffectFireSmall: {FrameCount: 2, DelayInSeconds: 0.2},
		}
		uiController := gui.NewUIController(tileRenderer, uiContext, userConfig)
		uiController.SetTileMapAnimations(tileMapAnimations)
		uiController.SetTTFFont(util.MustOpen(startInfos.FontInfos.TTFFile), startInfos.FontInfos.TTFSize)
		theGame.SetAndInitUserInterface(uiController)
		runGameWithEbiten(uiController)
	}
}

func startCreator(data defs.DataContainer) {
	creator := NewCreator(data)
	creator.Run()
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

func dumpGameData(gameData defs.DataContainer) {
	println("Defined Items:")
	for key, _ := range gameData.ItemDefinitions {
		println(fmt.Sprintf("  %s", key))
	}

	println("Defined Enemies:")
	for key, _ := range gameData.EnemyDefinitions {
		println(fmt.Sprintf("  %s", key))
	}
}

func maptest() {

	ldtkMapProject, _ := ldtk.Open("assets/maps.ldtk")
	testSource := rand.New(rand.NewSource(42))
	dunGen := dungen.NewAccretionGenerator(testSource)
	// dunGen.SetDebugSteps(true)
	dunGen.SetRoomCount(12)
	dunGen.LoadTemplatesFrom(ldtkMapProject, "")
	for {
		generatedLayout := dunGen.Generate(64, 64)
		generatedLayout.Print()
		// wait for key
		var input string
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("-more-")
		input, _ = reader.ReadString('\n')
		if strings.ToLower(input) == "q" {
			break
		}
	}

}

func runGameWithConsole(theGame *game.GameInstance) {
	theGame.SetConsoleMode(true)
	con := console.NewConsole(theGame)

	terminationCode := 0
	loopConfig := util.LoopConfig{
		TargetFPS:        5,
		IdleThreshold:    1,
		CurrentTimeFunc:  func() float64 { return float64(time.Now().UnixNano()) * 1e-9 },
		ProcessInputFunc: con.HandleInput,
		UpdateFunc:       con.Update,
		RenderFunc:       con.Draw,
	}

	loop := util.CreateLoop(loopConfig, &terminationCode)
	loop()
}

// runGameWithEbiten will start the infinitely running game loop
// game will receive the update and draw calls
func runGameWithEbiten(theGame *gui.UIController) {
	screenSize := theGame.GetDeviceIndependentScreenSize()

	ebiten.SetWindowTitle("roguelike")
	ebiten.SetWindowSize(screenSize.X, screenSize.Y)
	ebiten.SetScreenClearedEveryFrame(true)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSizeLimits(640, 480, -1, -1)
	ebiten.SetWindowClosingHandled(false)

	err := ebiten.RunGameWithOptions(theGame, &ebiten.RunGameOptions{
		GraphicsLibrary: ebiten.GraphicsLibraryOpenGL,
	})
	if err != nil && !errors.Is(err, ebiten.Termination) {
		log.Fatal(err)
	}
}
