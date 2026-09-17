package main

import (
	"os"
	"strings"
	"testing"

	"rpg/d1"
	"rpg/defs"
	"rpg/files"
	"rpg/recfile"
)

// tiles.rec and defs.PropIconID agree: every folded object lands on a record, every prop record is some object's.
func TestPropFolds(t *testing.T) {
	data, err := d1.Load(files.GetFileSystem())
	if err != nil {
		t.Fatal(err)
	}
	reached := map[string]bool{"prop.debris": true, "prop.portal": true, "prop.portal.2": true}
	for _, o := range data.Objects {
		id := defs.PropIconID(o.Name)
		if _, ok := defs.IconOK(id); !ok && id != defs.IconID("prop", o.Name) {
			t.Errorf("%s folds to %s, which tiles.rec lacks", o.Name, id)
		}
		reached[id], reached[id+".used"] = true, true
	}
	f, err := os.Open("files/assets/data/icons/tiles.rec")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, rec := range recfile.Read(f) {
		for _, field := range rec {
			if field.Name == "internal_name" && strings.HasPrefix(field.Value, "prop.") && !reached[field.Value] {
				t.Errorf("tiles.rec: %s isn't any object's icon", field.Value)
			}
		}
	}
}
