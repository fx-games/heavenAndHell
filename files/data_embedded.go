//go:build embedded

package files

import "embed"

// Exactly what the game reads at run time, and nothing else. Left out on purpose:
//   - assets/sources.for.tiles, assets/maps/backups and the raw sound and music packs: working material, not
//     runtime data.
//
// Mods ship with the binary too: a mod is one complete game, and files/mods/<name> is where its data lives -
// its music and sound included. The engine owns no audio of its own.
//
//go:embed assets/Bm437_IBM_CGA.png
//go:embed assets/endMessage.txt
//go:embed assets/names-female.txt
//go:embed assets/names-male.txt
//go:embed assets/data
//go:embed assets/fonts
//go:embed mods
var embeddedFS embed.FS

func init() {
	sourceFileSystem = embeddedFS
}
