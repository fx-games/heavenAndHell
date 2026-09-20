// Command d1import extracts Diablo's level data from a DIABDAT.MPQ into the text files the game reads from
// assets/data/d1/levels. You need your own copy of Diablo; the game itself does not, which is the point.
//
//	go run ./cmd/d1import -mpq reference/DIABDAT.MPQ
package main

import (
	"rpg/cmd/internal/mpq"

	"flag"
	"fmt"
	"log"
	"os"

	"rpg/d1"
)

func main() {
	mpqPath := flag.String("mpq", "", "path to DIABDAT.MPQ (default: look in "+fmt.Sprint(mpq.MPQPaths)+")")
	dest := flag.String("dest", "", "directory to write (default: assets/data/d1/levels)")
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
		*dest = "files/assets/data/d1/levels"
	}
	if err := d1.ImportLevels(read, *dest); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "wrote level data to %s\n", *dest)
}
