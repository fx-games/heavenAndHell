package app

import (
	"os"
	"strings"
	"testing"

	"rpg/files"
	"rpg/src/recfile"
	"rpg/src/rules"
)

// Every prop record in tiles.rec is some object's icon (a same_as target that nothing folds to would be dead).
func TestPropFolds(t *testing.T) {
	data, err := rules.Load(files.GetFileSystem())
	if err != nil {
		t.Fatal(err)
	}
	reached := map[string]bool{"prop.debris": true, "prop.portal": true, "prop.portal.2": true}
	for kind := range data.Objects {
		id := data.PropIcon(kind)
		reached[id], reached[id+".used"] = true, true
	}
	f, err := os.Open("files/mods/d1/data/icons/tiles.rec")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	recs := recfile.Read(f)
	for _, rec := range recs { // a same_as target is reached through the objects drawn with it
		m := rec.ToMap("")
		if target := m.GetStringOrDefault("same_as", ""); target != "" && reached[m.GetStringOrDefault("internal_name", "")] {
			reached[target] = true
		}
	}
	for _, rec := range recs {
		for _, field := range rec {
			if field.Name == "internal_name" && strings.HasPrefix(field.Value, "prop.") && !reached[field.Value] {
				t.Errorf("tiles.rec: %s isn't any object's icon", field.Value)
			}
		}
	}
}
