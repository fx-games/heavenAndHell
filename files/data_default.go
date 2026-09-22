//go:build !embedded

package files

import (
	"os"
	"path/filepath"
)

// Without the embedded tag the game reads its mods from disk: a release keeps them in mods/ next to the binary.
// Failing that (go run, tests), the working directory and its parents are searched for the repo's files/mods.
func init() {
	dir := findModsRoot()
	if exe, err := os.Executable(); err == nil && dir == filepath.Dir(exe) {
		appDir = dir // a release: its config.rec lives next to the binary too
	}
	if asFileSystem, ok := os.DirFS(dir).(FileSystem); ok {
		sourceFileSystem = asFileSystem
	}
}

func findModsRoot() string {
	if exe, err := os.Executable(); err == nil {
		if dir := filepath.Dir(exe); isDir(filepath.Join(dir, "mods")) {
			return dir
		}
	}
	for dir, _ := filepath.Abs("."); ; dir = filepath.Dir(dir) {
		if isDir(filepath.Join(dir, "files", "mods")) {
			return filepath.Join(dir, "files")
		}
		if filepath.Dir(dir) == dir {
			return "." // no mods anywhere: the first read fails and names the path
		}
	}
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
