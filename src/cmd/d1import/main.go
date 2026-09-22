// Command d1import extracts Diablo's level data from a DIABDAT.MPQ into the text files the game reads from
// mods/d1/data/levels. You need your own copy of Diablo; the game itself does not, which is the point.
//
//	go run ./src/cmd/d1import -mpq reference/DIABDAT.MPQ
package main

import (
	"rpg/src/cmd/internal/mpq"

	"flag"
	"fmt"
	"log"
	"os"

	"rpg/src/d1"
)

func main() {
	mpqPath := flag.String("mpq", "", "path to DIABDAT.MPQ (default: look in "+fmt.Sprint(mpq.MPQPaths)+")")
	dest := flag.String("dest", "", "directory to write (default: mods/d1/data/levels)")
	flag.Parse()

	if *mpqPath != "" {
		mpq.MPQPaths = []string{*mpqPath}
	}
	a, err := mpq.DiabDat()
	if err != nil {
		log.Fatal(err)
	}
	read := mpq.ReadFrom([]*mpq.MPQ{a})
	if *dest == "" {
		*dest = "files/mods/d1/data/levels"
	}
	if err := d1.ImportLevels(read, *dest); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "wrote level data to %s\n", *dest)
}
