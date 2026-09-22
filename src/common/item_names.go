package common

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"text/template"

	"rpg/files"
	"rpg/src/recfile"
)

// How an item reads in the inventory and in the buy/sell screens is the mod's: data/item_names.rec holds a Go
// text/template for each ("inventory:", and "shop:", which falls back to the inventory's), run on an ItemView.
// A mod without the file shows the bare name and the stack count.
const defaultItemNameTemplate = `{{.Name}}{{if gt .Count 1}} (x{{.Count}}){{end}}`

var itemNameTemplates = map[string]*template.Template{}

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

// LoadItemNameTemplates reads the active mod's item_names.rec. Called after mod selection.
func LoadItemNameTemplates() error {
	itemNameTemplates = map[string]*template.Template{}
	f, err := files.Open(path.Join("assets", "data", "item_names.rec"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("item_names.rec: %w", err)
	}
	defer f.Close()
	recs := recfile.Read(f)
	if len(recs) == 0 {
		return nil
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
		itemNameTemplates[field.Name] = t
	}
	return nil
}

var defaultItemName = template.Must(template.New("default").Parse(defaultItemNameTemplate))

// itemName runs the mod's template for where ("inventory" or "shop") on count of item.
func itemName(where string, item *Item, count int) string {
	t := itemNameTemplates[where]
	if t == nil {
		t = itemNameTemplates["inventory"]
	}
	if t == nil {
		t = defaultItemName
	}
	var b strings.Builder
	if err := t.Execute(&b, item.view(count)); err != nil {
		return item.Name()
	}
	return b.String()
}

func (i *Item) view(count int) ItemView {
	v := ItemView{Name: i.Name(), Count: count, IsWeapon: i.IsWeapon(), IsArmor: i.IsArmor()}
	if i.IsLightSource() && !i.HasInfiniteCharges() {
		v.Turns = i.GetCharges()
	}
	if d := i.d1; d != nil {
		v.MinDam, v.MaxDam, v.ToDam = d.MinDam, d.MaxDam, d.PLDamMod
		v.AC, v.ACBonus = d.AC, d.AC*d.PLAC/100
		v.Charges, v.MaxCharge, v.Dur, v.MaxDur = d.Charges, d.MaxCharges, d.Durability, d.MaxDur
		v.Dice = fmt.Sprintf("%d-%d", d.MinDam, d.MaxDam)
		if i.d1Data != nil && d.BaseIdx >= 0 && d.BaseIdx < len(i.d1Data.Items) {
			if dice := i.d1Data.Items[d.BaseIdx].Weapon.Damage; dice.DiceCount() > 0 {
				v.Dice = dice.ShortString()
			}
		}
		v.IsWeapon, v.IsArmor = d.MaxDam > 0, d.AC > 0
	}
	return v
}

// InventoryName is how the mod shows the item in the inventory; ShopName in the buy and sell screens.
func (i *Item) InventoryName() string { return itemName("inventory", i, i.Counter()) }
func (i *Item) ShopName() string      { return itemName("shop", i, i.Counter()) }

func (s InventoryStack) InventoryName() string { return itemName("inventory", s[0], len(s)) }
func (s InventoryStack) ShopName() string      { return itemName("shop", s[0], len(s)) }
