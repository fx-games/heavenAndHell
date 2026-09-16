package defs

// itemsIcons is the name->icon lookup for files/assets/data/icons/items.rec, edited at runtime with ./remap_items.sh.
// Item icons are resolved dynamically by defs.Icon("item.ITYPE_*"/"item.IMISC_*"/"item.<unique name>"), so this
// file carries no named Go constants.
var itemsIcons = loadIconRec("items")
