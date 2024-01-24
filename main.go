package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	_ "image/png"
	"io"
	"log"
	"math/rand"
	"os"
	"rpg/common"
	"rpg/dungen"
	"rpg/game"
	"rpg/geometry"
	"rpg/ldtk_go"
	"runtime/pprof"
	"strings"
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
	startInfos := StartInfo{
		Title:          "RPG",
		ScreenSize:     userConfig.WindowSize,
		TileScale:      4.0,
		LDTKMapFile:    "assets/maps.ldtk",
		FirstLevelName: "Town",
		FontInfos:      FontInfo{
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

	startTileGame(startInfos)
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

func startTileGame(startInfos StartInfo) {

	// init game
	deviceIndependentScreenSize := startInfos.ScreenSize
	tileScaleFactor := startInfos.TileScale
	theGame := game.NewGameInstance(startInfos.Title, deviceIndependentScreenSize, tileScaleFactor)

	// load ttf font
	theGame.SetTTFFont(mustLoad(os.Open(startInfos.FontInfos.TTFFile)), startInfos.FontInfos.TTFSize)

	// load map
	SetMapFromLDTK(theGame, startInfos.LDTKMapFile, startInfos.FirstLevelName, startInfos.TileMapAnimations)

	theGame.InitGame(startInfos.UserConfiguration)

	runGameWithEbiten(theGame)
}
func SetMapFromLDTK(theGame *game.GameInstance, ldtkFilename string, firstLevelName string, animations map[int32]game.TilemapAnimInfo) {
	ldtkMapProject, _ := ldtk_go.Open(ldtkFilename)
	theGame.SetLDTKProject(ldtkMapProject)

	townMap, atlas := theGame.NewGridMapFromLDTK(firstLevelName)
	theGame.SetWorldTexture(atlas)
	theGame.SetTileMapAnimations(animations)
	theGame.SetMap(townMap)
}
func mustLoad(open *os.File, err error) io.ReaderAt {
	if err != nil {
		panic(err)
	}
	return open
}

// runGameWithEbiten will start the infinitely running game loop
// game will receive the update and draw calls
func runGameWithEbiten(game *game.GameInstance) {
	screenSize := game.GetDeviceIndependentScreenSize()

	ebiten.SetWindowTitle(game.GetTitle())
	ebiten.SetWindowSize(screenSize.X, screenSize.Y)
	ebiten.SetScreenClearedEveryFrame(true)

	if err := ebiten.RunGameWithOptions(game, &ebiten.RunGameOptions{
		GraphicsLibrary: ebiten.GraphicsLibraryOpenGL,
	}); err != nil && !errors.Is(err, ebiten.Termination) {
		log.Fatal(err)
	}
}

type FontInfo struct {
	TTFFile          string
	TTFSize          float64
}

type StartInfo struct {
	FontInfos         FontInfo
	ScreenSize        geometry.Point
	TileScale         float64
	Title             string
	LDTKMapFile       string
	FirstLevelName    string
	TileMapAnimations map[int32]game.TilemapAnimInfo
	UserConfiguration game.Configuration
}
