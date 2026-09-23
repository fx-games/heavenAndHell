package common

import (
	"fmt"
	"strings"
	"text/template"

	"rpg/src/rules"
)

// defaultItemName is how an item reads when it has no mod data behind it: the bare name and the stack count.
var defaultItemName = template.Must(template.New("default").Parse(rules.DefaultItemNameTemplate))

// itemName runs the mod's template for where ("inventory" or "shop") on count of item.
func itemName(where string, item *Item, count int) string {
	t := defaultItemName
	if item.data != nil {
		t = item.data.ItemNameTemplate(where)
	}
	var b strings.Builder
	if err := t.Execute(&b, item.view(count)); err != nil {
		return item.Name()
	}
	return b.String()
}

func (i *Item) view(count int) rules.ItemView {
	v := rules.ItemView{Name: i.Name(), Count: count, IsWeapon: i.IsWeapon(), IsArmor: i.IsArmor()}
	if i.IsLightSource() && !i.HasInfiniteCharges() {
		v.Turns = i.GetCharges()
	}
	if d := i.roll; d != nil {
		v.MinDam, v.MaxDam, v.ToDam = d.MinDam, d.MaxDam, d.PLDamMod
		v.AC, v.ACBonus = d.AC, d.AC*d.PLAC/100
		v.Charges, v.MaxCharge, v.Dur, v.MaxDur = d.Charges, d.MaxCharges, d.Durability, d.MaxDur
		v.Dice = fmt.Sprintf("%d-%d", d.MinDam, d.MaxDam)
		if d.Base != nil {
			if dice := d.Base.Weapon.Damage; dice.DiceCount() > 0 {
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
