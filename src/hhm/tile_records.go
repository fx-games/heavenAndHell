package hhm

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"rpg/src/recfile"
)

// TileRecord is a map tile as icons/tiles.rec defines it: what the game calls it, the icon it is drawn with (the
// remapper's business) and what it blocks, from its tags.
type TileRecord struct {
	Name  string
	Icon  int32
	Flags Flags
}

// tileTags are the words a tiles.rec record's tags: field may use.
var tileTags = map[string]func(*Flags){
	"blocks_movement": func(f *Flags) { f.BlocksMovement = true },
	"blocks_view":     func(f *Flags) { f.BlocksView = true },
	"blocks_missiles": func(f *Flags) { f.BlocksMissiles = true },
	"trap":            func(f *Flags) { f.HoldsTrap = true },
	"is_damaging":     func(f *Flags) { f.Damaging = true },
}

// ReadTileRecords reads every record of a mod's <dataDir>/icons/tiles.rec ("assets/data" in the game's file
// system, "data" in a mod's own directory); a record with no icon of its own borrows its same_as record's.
func ReadTileRecords(fsys fs.FS, dataDir string) (map[string]TileRecord, error) {
	b, err := fs.ReadFile(fsys, path.Join(dataDir, "icons", "tiles.rec"))
	if err != nil {
		return nil, err
	}
	recs, sameAs := map[string]TileRecord{}, map[string]string{}
	for _, rec := range recfile.Read(bytes.NewReader(b)) {
		var id, alias string
		var t TileRecord
		hasIcon := false
		for _, field := range rec {
			switch field.Name {
			case "internal_name":
				id = field.Value
			case "name":
				t.Name = field.Value
			case "icon":
				t.Icon, hasIcon = field.AsInt32(), true
			case "same_as":
				alias = field.Value
			case "tags":
				for _, tag := range strings.FieldsFunc(field.Value, func(r rune) bool { return r == ',' || r == ' ' }) {
					set, ok := tileTags[tag]
					if !ok {
						return nil, fmt.Errorf("tiles.rec: %s: unknown tag %q", id, tag)
					}
					set(&t.Flags)
				}
			}
		}
		if alias != "" && !hasIcon { // its own icon, once the remapper gives it one, wins over same_as
			sameAs[id] = alias
		}
		recs[id] = t
	}
	for id, target := range sameAs {
		t := recs[id]
		t.Icon = recs[target].Icon
		recs[id] = t
	}
	return recs, nil
}

// DrawnTile is what the tile a map draws as number id is: the tiles.rec record the tileset names for it, or,
// for a tile that stands for no record, that picture with the tileset's own markers and no name.
func DrawnTile(ts *Tileset, recs map[string]TileRecord, id int) TileRecord {
	if name, ok := ts.Records[id]; ok {
		if t, ok := recs[name]; ok {
			return t
		}
	}
	return TileRecord{Icon: int32(id), Flags: ts.FlagsFor(id)}
}

// CheckTileRecords fails on a tileset tile that names a record tiles.rec does not have.
func CheckTileRecords(ts *Tileset, recs map[string]TileRecord) error {
	for id, name := range ts.Records {
		if _, ok := recs[name]; !ok {
			return fmt.Errorf("tileset: tile %d stands for %s, which tiles.rec does not define", id, name)
		}
	}
	return nil
}
