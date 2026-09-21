package defs

// itemsIcons is the name->icon lookup for files/assets/data/icons/items.rec, edited at runtime with ./remap_items.sh.
// Item icons are resolved dynamically by defs.Icon(defs.IconID("item", ...)) (see common/d1_item.go), so this
// file carries no named Go constants.
var itemsIcons map[string]int32

func loadItemsIcons() {
	itemsIcons = loadIconRec("items")
}
