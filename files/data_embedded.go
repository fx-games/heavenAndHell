//go:build embedded

package files

import (
	"embed"
	"os"
	"path/filepath"
)

// The mods are everything the game reads at run time: a mod is one complete game, its data, fonts, music and
// sounds included. files/assets is source material only and stays out.
//
//go:embed mods
var embeddedFS embed.FS

func init() {
	sourceFileSystem = embeddedFS
	if exe, err := os.Executable(); err == nil {
		appDir = filepath.Dir(exe) // the binary is the whole game: its config.rec lives next to it
	}
}
