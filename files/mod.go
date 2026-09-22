package files

import (
	"io/fs"
	"path"
)

// A mod is one complete game: every assets/<p> the game reads is mods/<name>/<p>. There is no engine fallback;
// files/assets holds only source material the game never reads.
type modFS struct {
	name string
	base FileSystem
}

var activeMod string

// SelectMod points every assets/ lookup at mods/<name> first. Empty name clears it.
func SelectMod(name string) {
	activeMod = name
	if name == "" {
		return
	}
	if _, ok := sourceFileSystem.(modFS); ok {
		return
	}
	sourceFileSystem = modFS{name: name, base: sourceFileSystem}
}

func ActiveMod() string { return activeMod }

// ModAvailable reports whether the selected mod actually has a directory under mods/. Without this check the
// first content read is what fails, and it blames a file inside the mod ("assets/data/xl_monst.txt") rather
// than the mod that is missing, which sends whoever hit it looking in the wrong place.
func ModAvailable(name string) bool {
	if name == "" {
		return true // no mod selected: the engine's own assets are the game
	}
	if _, err := sourceFileSystem.ReadDir(path.Join("mods", name)); err != nil {
		return false
	}
	return true
}

// SetFileSystem replaces the source tree and clears the active mod. The build tags do this for the game; tests
// use it to root the tree at files/ and then SelectMod, so they read through the same overlay the game does.
func SetFileSystem(fsys FileSystem) { sourceFileSystem, activeMod = fsys, "" }

// modPath maps assets/foo to mods/<mod>/foo. Anything outside assets/ is engine-internal.
func (m modFS) modPath(p string) (string, bool) {
	if activeMod == "" {
		return "", false
	}
	rest, ok := cutAssets(p)
	if !ok {
		return "", false
	}
	return path.Join("mods", activeMod, rest), true
}

func cutAssets(p string) (string, bool) {
	const prefix = "assets/"
	if len(p) > len(prefix) && p[:len(prefix)] == prefix {
		return p[len(prefix):], true
	}
	return "", false
}

func (m modFS) Open(name string) (fs.File, error) {
	if p, ok := m.modPath(name); ok {
		return m.base.Open(p)
	}
	return m.base.Open(name)
}

func (m modFS) ReadFile(name string) ([]byte, error) {
	if p, ok := m.modPath(name); ok {
		return m.base.ReadFile(p)
	}
	return m.base.ReadFile(name)
}

func (m modFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if p, ok := m.modPath(name); ok {
		return m.base.ReadDir(p)
	}
	return m.base.ReadDir(name)
}

// Mods lists the mod directories under mods/, the choices of the start menu's mod picker.
func Mods() []string {
	entries, _ := sourceFileSystem.ReadDir("mods")
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}
