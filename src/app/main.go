package app

import (
	"errors"
	"fmt"
	"image/color"
	_ "image/png"
	"log"
	"os"
	"rpg/files"
	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/game"
	"rpg/src/gui"
	"rpg/src/hhm"
	"rpg/src/renderer"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

func Main() {
	setupLogging()
	if len(os.Args) > 1 {
		argCmd := strings.ToLower(os.Args[1])
		switch argCmd {
		case "debug_tints":
			theGame, uiContext := createGameAndUIContext()
			debugTints(theGame, uiContext.StartInfos.SpriteSheet, os.Args[2:])
		case "loot":
			theGame, _ := createGameAndUIContext()
			genLoot(theGame)
		case "ascii":
			runGameWithConsole(createGameAndUIContext())
		case "playthrough":
			playthrough()
		case "autoplay":
			autoplay()
		}
	} else {
		runGameWithEbiten(createGameAndUIContext())
	}
}

// modError says which mod is missing and where it was looked for, rather than failing later on some file inside
// it. There is no engine game to fall back to: a mod must exist, load and start (docs/LEVELS.md).
func modError(name string) error {
	if files.ModAvailable(name) {
		return nil
	}
	return fmt.Errorf("mod %q not found: expected a directory at files/mods/%s (set Mod: in %s)", name, name, common.ConfigFile)
}

func mustHaveMod(name string) {
	if err := modError(name); err != nil {
		log.Fatal(err)
	}
}

// modOverride, when set, is played instead of the config's mod: the playthrough is D1's whatever the user picked.
var modOverride string

func createGameAndUIContext() (*game.GameInstance, common.UIContext) {
	userConfig := common.NewConfigurationFromRecFile(common.ConfigFile)
	if modOverride != "" {
		userConfig.Mod = modOverride
	}
	if !files.ModAvailable(userConfig.Mod) && modOverride == "" {
		userConfig.Mod = common.DefaultMod // config names a mod this build lacks (d1 left out): H&H it is
	}
	files.SelectMod(userConfig.Mod) // before anything reads content: assets/ resolves to the mod first from here on
	mustHaveMod(userConfig.Mod)
	// and before any Icon* is read: the numbers come from the mod's icon rec files
	if err := defs.LoadIcons(); err != nil {
		log.Fatal(err)
	}
	startInfos := common.StartInfo{
		Title:       "Of Heaven and Hell",
		ScreenSize:  userConfig.WindowSize,
		TileScale:   4.0,
		MapDir:      hhm.Dir,
		SpriteSheet: userConfig.SpriteSheet,
		FontInfos: common.FontInfo{
			TTFFile: userConfig.FontFile(),
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
		BackToMainMenu: theGame.BackToMainMenu,
		OpenSaveMenu:   theGame.OpenSaveMenu,
		OpenLoadMenu:   theGame.OpenLoadMenu,
		IsPermadeath:   theGame.IsPermadeath,
		QuitSaves:      theGame.QuitSaves,
		QuickSave:      theGame.QuickSave,
		QuickLoad:      theGame.QuickLoad,
		Animator: func() common.Animator {
			return theGame.GetActionBroker()
		},
		GetLog:      theGame.GetLog,
		StartInfos:  startInfos,
		Definitions: theGame.GetGameData(),
	}
	return theGame, uiContext
}

func genLoot(theGame *game.GameInstance) {
	if len(os.Args) < 3 {
		fmt.Println("usage: loot <dungeon_level>")
		return
	}
	dungeonLevel, _ := strconv.Atoi(os.Args[2])
	items := theGame.GenerateLoot(dungeonLevel, 10)
	for _, item := range items {
		fmt.Println(item.InventoryName())
	}
}

// modHost is the one ebiten.Game for the whole run; picking a mod swaps the controller it forwards to.
type modHost struct{ *gui.UIController }

func runGameWithEbiten(theGame *game.GameInstance, context common.UIContext) {
	host := &modHost{}
	var mount func(*game.GameInstance, common.UIContext)
	mount = func(g *game.GameInstance, ctx common.UIContext) {
		host.UIController = newEbitenUI(g, ctx)
		g.SetModSwitcher(func() { mount(createGameAndUIContext()) })
		g.ShowStartMenu()
		if titleImage, ok := gui.LoadTitleImage(); ok {
			host.UIController.ShowTitleScreen(titleImage)
		}
	}
	mount(theGame, context)
	webBridge(host)
	uiController := host.UIController
	screenSize := uiController.GetDeviceIndependentScreenSize()

	ebiten.SetWindowTitle("Of Heaven and Hell")
	ebiten.SetWindowSize(screenSize.X, screenSize.Y)
	if context.StartInfos.UserConfiguration.Fullscreen {
		uiController.SetFullscreen(true)
	}
	ebiten.SetScreenClearedEveryFrame(true)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowSizeLimits(640, 480, -1, -1)
	ebiten.SetWindowClosingHandled(true)

	err := ebiten.RunGameWithOptions(host, &ebiten.RunGameOptions{
		GraphicsLibrary: ebiten.GraphicsLibraryOpenGL,
	})
	if err != nil && !errors.Is(err, ebiten.Termination) {
		log.Fatal(err)
	}
}

// newEbitenUI builds the tile UI for one game: renderer, atlases and controller all read the active mod.
func newEbitenUI(theGame *game.GameInstance, context common.UIContext) *gui.UIController {
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
	theGame.SetAndInitUserInterface(uiController)
	uiController.OnQuit = theGame.SaveGame // decision 11: autosave on quit
	return uiController
}
