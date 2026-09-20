package d1

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

// ItemSnds is the sound an item makes landing on the floor (ItemDropSnds) and going into the inventory or being
// used (ItemInvSnds), by its cursor graphic (ItemCAnimTbl). Generated from devilution 1.09's items.cpp, HELLFIRE.
func ItemSnds(curs int) (drop, inv string) {
	if curs < 0 || curs >= len(itemCAnimTbl) {
		return "", ""
	}
	a := itemCAnimTbl[curs]
	return itemDropSnds[a], itemInvSnds[a]
}

// The three tables above live in mods/d1/data/item_sounds.rec: a "sound" record per sound class holding its drop
// and inventory sound, and an "anim" record whose repeated curs fields are ItemCAnimTbl.
var itemCAnimTbl []int
var itemDropSnds, itemInvSnds []string

func loadItemSounds(fsys fs.FS) (canim []int, drop, inv []string, err error) {
	recs, err := readRecMulti(fsys, "item_sounds.rec")
	if err != nil {
		return nil, nil, nil, err
	}
	for _, r := range recs["sound"] {
		m := r.ToMap("")
		drop = append(drop, m.GetStringOrDefault("drop", ""))
		inv = append(inv, m.GetStringOrDefault("inv", ""))
	}
	for _, r := range recs["anim"] {
		for _, line := range fieldsNamed(r, "curs") {
			for _, f := range strings.Fields(line) {
				n, convErr := strconv.Atoi(f)
				if convErr != nil {
					return nil, nil, nil, fmt.Errorf("item_sounds.rec: curs %q: %w", f, convErr)
				}
				if n >= len(drop) {
					return nil, nil, nil, fmt.Errorf("item_sounds.rec: curs %d has no sound class", n)
				}
				canim = append(canim, n)
			}
		}
	}
	return canim, drop, inv, nil
}
