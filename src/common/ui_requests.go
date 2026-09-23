package common

import "rpg/src/util"

// A UIRequest is something the rules need the player to see or choose. The rules only describe it and hand it to
// ActionContext.Show; which window that becomes is the frontend's business (game.show).
type UIRequest interface{ uiRequest() }

// ChooseOption offers a menu of options.
type ChooseOption struct{ Options []util.MenuItem }

// ChooseService lists the hero's items a paid service applies to, each with its price; Pick performs it.
type ChooseService struct {
	Items []*Item
	Price func(*Item) int
	Pick  func(*Item)
}

// OpenShop shows Vendor's stock for buying; Filter, when set, restricts it (basic vs. premium items).
type OpenShop struct {
	Vendor *Actor
	Filter func(*Item) bool
}

// OpenSelling shows what the hero could sell to Vendor.
type OpenSelling struct{ Vendor *Actor }

// ShowText is a text window.
type ShowText struct{ Lines []string }

// ShowItem is an item's description.
type ShowItem struct{ Item *Item }

// OpenStash is the hero's stash in town.
type OpenStash struct{}

// OpenEnchantmentTransfer is the menu that moves an enchantment from one item to another.
type OpenEnchantmentTransfer struct{}

// ChooseFromList is a list of items to pick from, laid out like the inventory.
type ChooseFromList struct{ List []*TextualStack }

// ChooseInventoryItem lets the hero pick an item of their pack that Filter accepts, by its letter.
type ChooseInventoryItem struct {
	Filter func(*Item) bool
	Picked func(letter rune)
}

// CloseMenus closes every open menu and window.
type CloseMenus struct{}

// ShowStats is the hero's full character sheet.
type ShowStats struct{}

// ShowGameOver is the menu after the hero died.
type ShowGameOver struct{}

// EndTargeting drops the targeting cursor: the hero attacked or cast, however it was aimed.
type EndTargeting struct{}

func (ChooseOption) uiRequest()            {}
func (ChooseFromList) uiRequest()          {}
func (ChooseInventoryItem) uiRequest()     {}
func (CloseMenus) uiRequest()              {}
func (ShowStats) uiRequest()               {}
func (ShowGameOver) uiRequest()            {}
func (EndTargeting) uiRequest()            {}
func (ChooseService) uiRequest()           {}
func (OpenShop) uiRequest()                {}
func (OpenSelling) uiRequest()             {}
func (ShowText) uiRequest()                {}
func (ShowItem) uiRequest()                {}
func (OpenStash) uiRequest()               {}
func (OpenEnchantmentTransfer) uiRequest() {}
