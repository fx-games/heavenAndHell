package app

import (
	"log"
	"rpg/src/common"
	"rpg/src/game"
)

// The browser has no terminal: ASCII mode (tcell, cview) stays out of the web build.
func runGameWithConsole(*game.GameInstance, common.UIContext) { log.Fatal("ascii mode needs a terminal") }
