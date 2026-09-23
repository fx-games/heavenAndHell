package rules

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

// How an item reads in the inventory and in the buy/sell screens is the mod's: data/item_names.rec holds a Go
// text/template for each ("inventory:", and "shop:", which falls back to the inventory's), run on an ItemView.
// A mod without the file shows the bare name and the stack count.
const DefaultItemNameTemplate = `{{.Name}}{{if gt .Count 1}} (x{{.Count}}){{end}}`

// ItemView is what an item-name template can show.
type ItemView struct {
	Name               string
	Count              int    // the stack's size
	IsWeapon, IsArmor  bool   // weapons carry damage, armour AC
	Dice               string // the base item's damage as declared, "1d4" (or "1-4" when it names a range)
	MinDam, MaxDam     int
	ToDam              int // flat bonus damage (a "damage" affix)
	AC, ACBonus        int // armour class, and what the item's armour affixes add to it
	Charges, MaxCharge int
	Dur, MaxDur        int
	Turns              int // a light source's charges left, 0 for one that never runs out
}

var defaultItemName = template.Must(template.New("default").Parse(DefaultItemNameTemplate))

// ItemNameTemplate is the mod's template for where ("inventory" or "shop"); shop falls back to inventory.
func (d *Data) ItemNameTemplate(where string) *template.Template {
	if t := d.itemNames[where]; t != nil {
		return t
	}
	if t := d.itemNames["inventory"]; t != nil {
		return t
	}
	return defaultItemName
}

// loadItemNames reads item_names.rec.
func loadItemNames(fsys fs.FS, d *Data) error {
	d.itemNames = map[string]*template.Template{}
	recs, err := ReadRec(fsys, "item_names.rec")
	if errors.Is(err, fs.ErrNotExist) || err == nil && len(recs) == 0 {
		return nil
	}
	if err != nil {
		return err
	}
	for _, field := range recs[0] {
		if field.Name != "inventory" && field.Name != "shop" {
			return fmt.Errorf("item_names.rec: unknown field %q (want inventory, shop)", field.Name)
		}
		t, err := template.New(field.Name).Option("missingkey=error").Parse(field.Value)
		if err != nil {
			return fmt.Errorf("item_names.rec: %s: %w", field.Name, err)
		}
		if err := t.Execute(&strings.Builder{}, ItemView{Name: "check", Count: 2}); err != nil {
			return fmt.Errorf("item_names.rec: %s: %w", field.Name, err)
		}
		d.itemNames[field.Name] = t
	}
	return nil
}
