package d1

import (
	"errors"
	"fmt"
	"io/fs"

	"rpg/src/recfile"
)

// The tables that used to be Go literals live in this mod's own .rec files. readRec and readRecMulti are the only
// two ways they are read: a table group gets a function that turns records into whatever the literal used to be,
// and nothing more. Fields are looked up by name, so a .rec may carry columns this game ignores.
func readRec(fsys fs.FS, name string) ([]recfile.Record, error) {
	recs, err := readRecMulti(fsys, name)
	if err != nil {
		return nil, err
	}
	return recs["default"], nil
}

func readRecMulti(fsys fs.FS, name string) (map[string][]recfile.Record, error) {
	f, err := fsys.Open(dataPath(name))
	if err != nil {
		return nil, fmt.Errorf("d1 data %s: %w", name, err)
	}
	defer f.Close()
	return recfile.ReadMulti(f), nil
}

func loadMonsterTables(fsys fs.FS, d *Data) error {
	recs, err := readRecMulti(fsys, "monsters.rec")
	// monsters.rec glosses the AI_* brains in plain English for the actor popup. A mod whose monsters are driven
	// by skills rather than by Diablo's brains names no AI_* at all and ships no file; the glossary stays empty.
	if errors.Is(err, fs.ErrNotExist) {
		d.MonsterAIDescriptions = map[string]string{}
		return nil
	}
	if err != nil {
		return err
	}
	d.MonsterAIDescriptions = make(map[string]string, len(recs["ai"]))
	for _, r := range recs["ai"] {
		m := r.ToMap("")
		d.MonsterAIDescriptions[m.GetStringOrDefault("id", "")] = m.GetStringOrDefault("description", "")
	}
	return nil
}

// fieldsNamed returns every value a record carries under key, in file order. Tables that are really one long list
// of numbers (D1 has a few) spread it over repeated fields so the file stays readable and diffs stay small.
func fieldsNamed(r recfile.Record, key string) []string {
	var out []string
	for _, f := range r {
		if f.Name == key {
			out = append(out, f.Value)
		}
	}
	return out
}
