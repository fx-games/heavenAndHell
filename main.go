package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	_ "image/png"
	"log"
	"math/rand"
	"os"
	"rpg/common"
	"rpg/console"
	"rpg/dungen"
	"rpg/game"
	"rpg/ldtk_go"
	"rpg/util"
	"runtime/pprof"
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

	userConfig := game.NewConfigurationFromRecFile("config.rec")
	startInfos := game.StartInfo{
		Title:          "RPG",
		ScreenSize:     userConfig.WindowSize,
		TileScale:      4.0,
		LDTKMapFile:    "assets/maps.ldtk",
		FontInfos:      game.FontInfo{
			TTFFile: userConfig.DefaultFont,
			TTFSize: userConfig.DefaultFontSize,
		},

		TileMapAnimations: map[int32]game.TilemapAnimInfo{
			28:                         {FrameCount: 2, DelayInSeconds: 1.5},
			32:                         {FrameCount: 2, DelayInSeconds: 0.2},
			common.IconEffectFireSmall: {FrameCount: 2, DelayInSeconds: 0.2},
		},
		UserConfiguration: userConfig,
	}

	theGame := game.NewGameInstance(startInfos)
	if len(os.Args) > 1 && strings.ToLower(os.Args[1]) == "ascii" {
		runGameWithConsole(theGame)
	} else {
		runGameWithEbiten(theGame)
	}
}

func maptest() {

	ldtkMapProject, _ := ldtk_go.Open("assets/maps.ldtk")

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

func runGameWithConsole(game *game.GameInstance) {
	game.SetConsoleMode(true)
	con := console.NewConsole(game)

	loopConfig := util.LoopConfig{
		TargetFPS:        5,
		IdleThreshold:    1,
		CurrentTimeFunc:  func() float64 {return float64(time.Now().UnixNano()) * 1e-9},
		ProcessInputFunc: con.HandleInput,
		UpdateFunc:       con.Update,
		RenderFunc:       con.Draw,
	}

	loop := util.CreateLoop(loopConfig)

	loop()
}

// runGameWithEbiten will start the infinitely running game loop
// game will receive the update and draw calls
func runGameWithEbiten(game *game.GameInstance) {
	screenSize := game.GetDeviceIndependentScreenSize()

	ebiten.SetWindowTitle(game.GetTitle())
	ebiten.SetWindowSize(screenSize.X, screenSize.Y)
	ebiten.SetScreenClearedEveryFrame(true)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSizeLimits(640, 480, -1, -1)

	if err := ebiten.RunGameWithOptions(game, &ebiten.RunGameOptions{
		GraphicsLibrary: ebiten.GraphicsLibraryOpenGL,
	}); err != nil && !errors.Is(err, ebiten.Termination) {
		log.Fatal(err)
	}
}
