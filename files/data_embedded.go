//go:build embedded

package files

import "embed"

// Exactly what the game reads at run time, and nothing else. Left out on purpose:
//   - assets/sources.for.tiles, assets/maps/backups and the raw music packs (including the source files in
//     assets/music that are not .ogg): working material, not runtime data.
//   - assets/voice: optional, and empty in this tree; an embed of a missing directory would not build.
//
// Mods ship with the binary too: a mod is one complete game, and files/mods/<name> is where its data lives.
//
//go:embed assets/Bm437_IBM_CGA.png
//go:embed assets/classic_roguelike.png
//go:embed assets/maps.ldtk
//go:embed assets/endMessage.txt
//go:embed assets/names-female.txt
//go:embed assets/names-male.txt
//go:embed assets/data
//go:embed assets/fonts
//go:embed assets/sfx
//go:embed assets/ang_sfx
//go:embed assets/ambience
//go:embed assets/music/*.ogg
//go:embed mods
var embeddedFS embed.FS

func init() {
	sourceFileSystem = embeddedFS
}
