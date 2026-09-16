// Command d1import extracts Diablo's level data from a DIABDAT.MPQ into the text files the game reads from
// assets/data/d1/levels. You need your own copy of Diablo; the game itself does not, which is the point.
//
//	go run ./cmd/d1import -mpq reference/DIABDAT.MPQ
//
// With -sfx it instead extracts the sound files the game asks for (everything sounds.rec points at under
// diablo_sfx, plus d1.SfxFiles' speech and all of Hellfire's additions) out of DIABDAT.MPQ and HELLFIRE.MPQ.
// They land as .wav; convert them to .ogg with the ffmpeg line in AGENTS.md, since the engine only decodes Vorbis.
//
//	go run ./cmd/d1import -sfx
package main

import (
	"rpg/cmd/internal/mpq"

	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"rpg/d1"
)

func main() {
	mpqPath := flag.String("mpq", "", "path to DIABDAT.MPQ (default: look in "+fmt.Sprint(mpq.MPQPaths)+")")
	hellfire := flag.String("hellfire", "reference", "directory holding HELLFIRE.MPQ and the HF*.MPQ expansion archives")
	sfx := flag.Bool("sfx", false, "extract sound files instead of level data")
	dest := flag.String("dest", "", "directory to write (default: assets/data/d1/levels, or assets/diablo_sfx with -sfx)")
	flag.Parse()

	if *mpqPath != "" {
		mpq.MPQPaths = []string{*mpqPath}
	}
	if *sfx {
		if err := importSfx(*hellfire, *dest); err != nil {
			log.Fatal(err)
		}
		return
	}
	a, err := mpq.DiabDat()
	if err != nil {
		log.Fatal(err)
	}
	// the Crypt's and the Nest's tile tables live in HELLFIRE.MPQ
	read := mpq.ReadFrom(append([]*mpq.MPQ{a}, mpq.HellfireArchives(*hellfire)...))
	if *dest == "" {
		*dest = "files/assets/data/d1/levels"
	}
	if err := d1.ImportLevels(read, *dest); err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "wrote level data to %s\n", *dest)
}

// importSfx writes every sound the game can use out of whichever archives are present. Neither is required: with
// only Hellfire you get its own sounds, with only Diablo you get D1's, and with neither it says so and stops.
func importSfx(hellfirePath, dest string) error {
	if dest == "" {
		dest = "files/assets/diablo_sfx"
	}
	// hfvoice.mpq carries the expansion's speech at full quality, so it goes first and wins over the cut-down
	// copies in hellfire.mpq; DIABDAT is last because it has none of Hellfire's own sounds.
	archives := mpq.HellfireArchives(hellfirePath)
	if a, err := mpq.DiabDat(); err == nil {
		archives = append(archives, a)
	} else {
		fmt.Fprintf(os.Stderr, "no DIABDAT.MPQ (%v), skipping Diablo's own sounds\n", err)
	}
	if len(archives) == 0 {
		return fmt.Errorf("no MPQ found: looked for DIABDAT.MPQ and %s/{HELLFIRE,HFVOICE,HFMONK,HFMUSIC}.MPQ", hellfirePath)
	}
	want, err := wantedSfx("files/assets/data/sounds.rec", d1.SfxFiles)
	if err != nil {
		return err
	}
	want = append(want, hellfireSfx()...)
	n, err := extractSfx(archives, want, dest)
	if err != nil {
		return err
	}
	music, err := extractMusic(archives, filepath.Join(filepath.Dir(dest), "diablo_music"))
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %d of %d sound files to %s (and %d music tracks)\n", n, len(want), dest, music)
	fmt.Fprintf(os.Stderr, "now convert them to .ogg (see AGENTS.md); the engine only decodes Vorbis\n")
	return nil
}
