// Command d1import extracts Diablo's level data from a DIABDAT.MPQ into the text files the game reads from
// assets/data/d1/levels. You need your own copy of Diablo; the game itself does not, which is the point.
//
//	go run ./cmd/d1import -mpq reference/DIABDAT.MPQ
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"rpg/d1"
)

func main() {
	mpq := flag.String("mpq", "", "path to DIABDAT.MPQ (default: look in "+fmt.Sprint(MPQPaths)+")")
	dest := flag.String("dest", "files/assets/data/d1/levels", "levels directory to write")
	flag.Parse()

	if *mpq != "" {
		MPQPaths = []string{*mpq}
	}
	a, err := DiabDat()
	if err != nil {
		log.Fatal(err)
	}
	if err := d1.ImportLevels(a.ReadFile, *dest); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "wrote level data to %s\n", *dest)
}
