//go:build !js

package app

import (
	"log"
	"rpg/src/common"
	"rpg/src/console"
	"rpg/src/game"
)

func runGameWithConsole(theGame *game.GameInstance, context common.UIContext) {
	for {
		theGame.SetConsoleMode(true)
		con := console.NewConsole(context)
		theGame.SetAndInitUserInterface(con)
		switched := false
		theGame.SetModSwitcher(func() { switched = true; con.QuitGame() })
		if err := con.Run(theGame.ShowStartMenu); err != nil {
			log.Fatal(err)
		}
		if !switched {
			theGame.SaveGame() // decision 11: autosave on quit
			return
		}
		theGame, context = createGameAndUIContext() // the mod picker saved the new mod to the config
	}
}

