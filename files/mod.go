package files

import (
	"io/fs"
	"path"
)

// A mod is one complete game: its files shadow the engine's own under assets/.
// Lookups try mods/<name>/<p> first and fall back to assets/<p>, so engine-owned
// resources (fonts, shaders, the fallback room) stay in one place.
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
		if f, err := m.base.Open(p); err == nil {
			return f, nil
		}
	}
	return m.base.Open(name)
}

func (m modFS) ReadFile(name string) ([]byte, error) {
	if p, ok := m.modPath(name); ok {
		if b, err := m.base.ReadFile(p); err == nil {
			return b, nil
		}
	}
	return m.base.ReadFile(name)
}

func (m modFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if p, ok := m.modPath(name); ok {
		if e, err := m.base.ReadDir(p); err == nil {
			return e, nil
		}
	}
	return m.base.ReadDir(name)
}
