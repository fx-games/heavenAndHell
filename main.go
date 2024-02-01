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
	"path"
	"rpg/common"
	"rpg/console"
	"rpg/dungen"
	"rpg/game"
	"rpg/ldtk_go"
	"rpg/recfile"
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
	if len(os.Args) > 1 {
		argCmd := strings.ToLower(os.Args[1])
		switch argCmd {
		case "loot":
			genLoot(theGame)
		case "ascii":
			runGameWithConsole(theGame)
		case "csv_dump":
			dumpGameData()
		case "validate":
			return
		}
	} else {
		runGameWithEbiten(theGame)
	}
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

func dumpGameData() {
	file, _ := os.Open(path.Join("assets", "data", "enemies", "enemies.rec"))
	records := recfile.Read(file)
	file.Close()

	outFile, _ := os.Create(path.Join("assets", "data", "enemies", "enemies.csv"))
	fieldNames := []string{"dlvl", "name", "icon", "internal_name", "base_speed", "base_health", "stats_experience", "stats_strength", "stats_dexterity", "stats_intelligence", "equipment", "targeted_skill", "death_string"}
	recfile.WriteCSV(outFile, fieldNames, records)
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

func runGameWithConsole(theGame *game.GameInstance) {
	theGame.SetConsoleMode(true)
	con := console.NewConsole(theGame)

	terminationCode := 0
	loopConfig := util.LoopConfig{
		TargetFPS:        5,
		IdleThreshold:    1,
		CurrentTimeFunc:  func() float64 {return float64(time.Now().UnixNano()) * 1e-9},
		ProcessInputFunc: con.HandleInput,
		UpdateFunc:       con.Update,
		RenderFunc:       con.Draw,
	}

	loop := util.CreateLoop(loopConfig, &terminationCode)
	loop()

	if terminationCode == 2 {
		theGame.SetConsoleMode(false)
		defer runGameWithEbiten(theGame)
	}
}

// runGameWithEbiten will start the infinitely running game loop
// game will receive the update and draw calls
func runGameWithEbiten(theGame *game.GameInstance) {
	screenSize := theGame.GetDeviceIndependentScreenSize()

	ebiten.SetWindowTitle(theGame.GetTitle())
	ebiten.SetWindowSize(screenSize.X, screenSize.Y)
	ebiten.SetScreenClearedEveryFrame(true)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSizeLimits(640, 480, -1, -1)
	ebiten.SetWindowClosingHandled(false)

	err := ebiten.RunGameWithOptions(theGame, &ebiten.RunGameOptions{
		GraphicsLibrary: ebiten.GraphicsLibraryOpenGL,
	})

	if err != nil {
		if errors.Is(err, game.ContinueWithAscii) {
			defer runGameWithConsole(theGame)
		} else if !errors.Is(err, ebiten.Termination) {
			log.Fatal(err)
		}
	}
}
