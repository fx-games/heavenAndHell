//go:build !windows

package game

import (
	"log"
	"os"
	"syscall"
)

// restart replaces this process with a fresh copy of the game: same PID and binary, so a `go run` parent keeps
// its temp binary alive (starting a child and exiting let go run delete it mid-start, and Metal crashed).
func restart() {
	exe, err := os.Executable()
	if err == nil {
		err = syscall.Exec(exe, os.Args, os.Environ())
	}
	log.Printf("could not restart, start the game again to play the new mod: %v", err)
	os.Exit(0)
}
