package main

import (
	"rpg/cmd/internal/mpq"

	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hellfireMusic maps Hellfire's two new tracks to the cue names d1AreaMusic asks for. They live in hfmusic.mpq,
// which the expansion's disc ships alongside hellfire.mpq; D1's own dtowne/dlvla-d come out of DIABDAT.
var hellfireMusic = map[string]string{
	`Music\DLvlE.wav`: "nest.wav",
	`Music\DLvlF.wav`: "crypt.wav",
}

// hellfireSfx lists every sound Hellfire adds, taken from the paths in reference/diablo-hellfire's MONSTDAT.CPP
// (the "%c%i" monster patterns, expanded) and EFFECTS.H. None of them are in DIABDAT.MPQ.
//
// The Monk's own voice set ("Sfx\Monk\Monk*.wav") is in hfmonk.mpq and hfvoice.mpq rather than hellfire.mpq, so
// it is listed here too and simply skipped when neither archive is around — the Monk then falls silent
// (d1HeroVoice in common/a_factory.go).
func hellfireSfx() []string {
	var out []string
	// the new monsters: <name><a|h|d|s><take>.WAV, attack / hit / death / special
	for _, m := range []string{"Biclop", "Crypt", "Defile", "FTwin", "FleshT", "GDiggr", "HBoar", "HDemon",
		"HSpawn", "HelBat", "KBrute", "Lich", "Lworm", "Nakrul", "Psyco", "Reaper", "SLord", "SWing",
		"Satyr", "Shred", "Stingr", "TchAnt", "TmbRat", "psyco"} {
		for _, c := range "ahds" {
			for i := 1; i <= 4; i++ {
				out = append(out, fmt.Sprintf(`Monsters\newsfx\%s%c%d.WAV`, m, c, i))
			}
		}
	}
	// the expansion's speech and one-off effects
	for _, n := range []string{"DEFILER1", "DEFILER2", "DEFILER3", "DEFILER4", "DEFILER6", "DEFILER7", "DEFILER8",
		"Farmer1", "Farmer2", "Farmer2A", "Farmer3", "Farmer4", "Farmer5", "Farmer6", "Farmer7", "Farmer8", "Farmer9",
		"NAKRUL1", "NAKRUL2", "NAKRUL3", "NAKRUL4", "NAKRUL5", "NAKRUL6",
		"NARATR3", "Naratr4", "Naratr5", "Naratr6", "Naratr7", "Naratr8", "Naratr9",
		"COWSUT1", "COWSUT2", "COWSUT3", "COWSUT4", "COWSUT4A", "COWSUT5", "COWSUT6", "COWSUT7", "COWSUT8",
		"COWSUT9", "COWSUT10", "COWSUT11", "COWSUT12",
		"TEDDYBR1", "TEDDYBR2", "TEDDYBR3", "TEDDYBR4"} {
		out = append(out, `Sfx\Hellfire\`+n+".wav")
	}
	for _, n := range []string{"Crclos", "Cropen", "PodPop5", "PodPop8", "UrnPop2", "UrnPop3"} {
		out = append(out, `Sfx\Items\`+n+".wav")
	}
	for _, n := range []string{"FBallBow", "NestXpld", "NewBFire", "Sting1"} {
		out = append(out, `Sfx\Misc\`+n+".wav")
	}
	for i := 1; i <= 102; i++ { // hfmonk.mpq, absent from a plain Hellfire install
		out = append(out, fmt.Sprintf(`Sfx\Monk\Monk%02d.wav`, i))
	}
	return out
}

// sfxSource is sfxDest backwards: the MPQ path a wanted file came from. Everything under "monsters/" lives in
// the archive's Monsters folder, the rest under Sfx. MPQ lookups are case insensitive, so the case here is only
// for readability.
func sfxSource(rel string) string {
	rel = strings.TrimSuffix(strings.ReplaceAll(rel, "/", `\`), ".ogg") + ".wav"
	if strings.HasPrefix(strings.ToLower(rel), `monsters\`) {
		return rel
	}
	return `Sfx\` + rel
}

// wantedSfx is every Diablo sound the game already asks for: the files sounds.rec points at under diablo_sfx and
// the speech ids in d1.SfxFiles. Deriving the list from what is wired up means a new sound event is extracted
// without touching this tool.
func wantedSfx(soundsRec string, speech map[string]string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(rel string) {
		src := sfxSource(rel)
		if !seen[src] {
			seen[src] = true
			out = append(out, src)
		}
	}
	b, err := os.ReadFile(soundsRec)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		rel, ok := strings.CutPrefix(line, "sound: diablo_sfx/")
		if ok {
			add(rel)
		}
	}
	for _, file := range speech {
		add(file)
	}
	return out, nil
}

// sfxDest turns an MPQ sound path into the file the game looks for: the path below its top folder, lowercased.
// "Sfx\Hellfire\Farmer1.wav" becomes "hellfire/farmer1.wav", matching d1.SfxFiles' convention.
func sfxDest(name string) string {
	parts := strings.Split(strings.ToLower(name), `\`)
	if len(parts) > 1 && parts[0] == "sfx" { // Monsters keeps its folder, the way sounds.rec spells it
		parts = parts[1:]
	}
	return filepath.Join(parts...)
}

// extractSfx pulls every sound it can find out of the archives into dest, skipping what isn't there (a plain
// Hellfire install has no hfmonk.mpq, and a shareware DIABDAT is missing most speech). Returns how many it wrote.
func extractSfx(archives []*mpq.MPQ, names []string, dest string) (int, error) {
	written := 0
	for _, name := range names {
		var data []byte
		for _, a := range archives {
			if b, err := a.ReadFile(name); err == nil {
				data = b
				break
			}
		}
		if data == nil {
			continue
		}
		out := filepath.Join(dest, sfxDest(name))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}

// extractMusic writes Hellfire's two new tracks under the names d1AreaMusic looks for. D1's own tracks stay a
// manual step: they need renaming choices (dtowne -> town and so on) that AGENTS.md spells out.
func extractMusic(archives []*mpq.MPQ, dest string) (int, error) {
	written := 0
	for name, out := range hellfireMusic {
		for _, a := range archives {
			b, err := a.ReadFile(name)
			if err != nil {
				continue
			}
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return written, err
			}
			if err := os.WriteFile(filepath.Join(dest, out), b, 0o644); err != nil {
				return written, err
			}
			written++
			break
		}
	}
	return written, nil
}
