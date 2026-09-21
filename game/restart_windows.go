package game

import (
	"log"
	"os"
	"os/exec"
)

// restart starts a fresh copy of the game and quits this one (Windows has no exec).
func restart() {
	exe, err := os.Executable()
	if err == nil {
		cmd := exec.Command(exe, os.Args[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		err = cmd.Start()
	}
	if err != nil {
		log.Printf("could not restart, start the game again to play the new mod: %v", err)
	}
	os.Exit(0)
}
