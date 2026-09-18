// Package console is the ASCII frontend: it implements common.UserInterface with the textui framework.
package console

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"log"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/memmaker/go/cview"
	mgeom "github.com/memmaker/go/geometry"
	"github.com/memmaker/go/textiles"
	"rpg/common"
	"rpg/defs"
	"rpg/files"
	"rpg/geometry"
	"rpg/stats"
	"rpg/textui"
	"rpg/util"
)

type Console struct {
	context         common.UIContext
	config          *common.Configuration
	ui              *textui.UI
	msg             func(message string)
	fullStats       bool // the right panel shows the character sheet instead of the inventory until the next turn
	panelsDirty     bool // the side panels only change on a turn, so they are rebuilt then instead of every frame
	forceAggression bool
	disableFog      bool
	ticks           uint64
	lastTarget      common.TargetInfo
}

func NewConsole(gameContext common.UIContext) *Console {
	c := &Console{
		context: gameContext,
		config:  gameContext.StartInfos.UserConfiguration,
		msg:     func(string) {},
	}
	c.ui = textui.NewTextUI(c, textui.Settings{
		Theme:             mustRead("assets/data/ascii/ui.rec"),
		Palette:           mustRead("assets/data/ascii/palette.rec"),
		KeyMap:            mustRead("assets/data/ascii/keymap.txt"),
		RightPanelWidth:   34,
		MinTerminalWidth:  80,
		MinTerminalHeight: 24,
		AnimationDelay:    40 * time.Millisecond,
	})
	c.ui.OnMapHovered = c.describe
	c.ui.OnMapClicked = c.onMapClicked
	c.ui.OnRightPanelClicked = c.onRightPanelClicked
	c.ui.OnRightPanelHovered = c.describeInventoryLine
	c.setupCommands()
	return c
}

func mustRead(file string) *bytes.Reader {
	data, err := files.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	return bytes.NewReader(data)
}

// Run blocks until the player quits. onScreenReady runs once the terminal is ready for the first menus.
func (c *Console) Run(onScreenReady func()) error {
	return c.ui.Run(30, c.tick, onScreenReady)
}

func (c *Console) tick() {
	c.ticks++
	state := c.context.GameState()
	if state == nil || state.GetMap() == nil || state.GetPlayer() == nil {
		return
	}
	c.context.Animator().Tick()
	if state.GetPlayer().IsAlive() {
		for _, actor := range state.GetMap().Actors() {
			actor.Tick()
		}
	}
	c.ui.SetAnimationIcons(c.animationIcons())
	if c.panelsDirty {
		c.panelsDirty = false
		c.refreshPanels()
	}
}

// Panels

func (c *Console) refreshPanels() {
	state := c.context.GameState()
	player := state.GetPlayer()
	c.ui.SetStatusBar(c.statusLine())
	if c.fullStats {
		c.ui.SetRightPanel(labelsToText(player.GetFullStats()))
	} else {
		var lines []string
		for _, entry := range c.inventoryEntries(nil) {
			lines = append(lines, textui.InventoryLabel(entry, player.GetEquipment().IsEquipped(entry.(invEntry).stack[0])))
		}
		c.ui.SetRightPanel(strings.Join(lines, "\n"))
	}
	c.ui.SetLowerRightPanel(c.visibleActorsText())
}

func (c *Console) statusLine() string {
	state := c.context.GameState()
	player := state.GetPlayer()
	playerStats := player.GetStats()
	resource := func(label string, name stats.Name) string {
		res := playerStats.GetResource(name)
		textColor := defs.LightTextColor
		switch {
		case res.IsLessThanPercent(0.25):
			textColor = color.RGBA{R: 255, G: 30, B: 30, A: 255}
		case res.IsLessThanPercent(0.5):
			textColor = color.RGBA{R: 255, G: 150, B: 30, A: 255}
		case res.IsLessThanPercent(0.75):
			textColor = color.RGBA{R: 255, G: 255, B: 30, A: 255}
		}
		return fmt.Sprintf("%s: %s%s[-]", label, colorTag(textColor), res.ToString())
	}
	parts := []string{
		cview.Escape(strings.ReplaceAll(state.GetMap().GetDisplayName(), "\n", " ")),
		resource("HP", stats.Health),
		resource("MP", stats.Mana),
		fmt.Sprintf("Gold: %d", playerStats.GetAttribute(stats.Gold)),
	}
	if h := player.Hero(); h != nil && h.StatPts > 0 {
		parts = append(parts, fmt.Sprintf("%sStat points: %d (u)[-]", colorTag(defs.PositiveHighlightColor), h.StatPts))
	}
	for _, effect := range c.timedEffects() {
		parts = append(parts, cview.Escape(effect))
	}
	return " " + strings.Join(parts, " | ")
}

// timedEffects are the player's status effects and delayed spells (the tile UI's timer overlay).
func (c *Console) timedEffects() []string {
	state := c.context.GameState()
	var result []string
	for _, being := range state.GetTimeKeeper().GetBeingsByFilter(func(being common.TimedBeing) bool {
		_, isDelayedSpell := being.(*common.UntargetedAction)
		return isDelayedSpell
	}) {
		result = append(result, being.(*common.UntargetedAction).Name())
	}
	player := state.GetPlayer()
	for statusName, turnsLeft := range player.GetStatusEffects().AllEntries() {
		result = append(result, effectText(statusName, turnsLeft))
	}
	for _, statusEffect := range player.GetParametrizedStatusEffects() {
		result = append(result, effectText(string(statusEffect.GetName()), statusEffect.TurnsLeft()))
	}
	slices.Sort(result)
	return result
}

func effectText(name string, turnsLeft int) string {
	if turnsLeft > 0 {
		return fmt.Sprintf("%s(%d)", name, turnsLeft)
	}
	return name
}

// visibleActorsText lists the actors in view with health bars, the tile UI's health bars over the actors.
func (c *Console) visibleActorsText() string {
	state := c.context.GameState()
	player := state.GetPlayer()
	actors := c.visibleActorsByDistance()
	theme := c.ui.Theme()
	stageColors := [3]color.RGBA{theme.GetColorByName("neon_green_3"), theme.GetColorByName("yellow_4"), theme.GetColorByName("red_6")}
	var lines []string
	for _, actor := range actors {
		icon := c.actorIcon(actor)
		if actor.IsHostileTo(player) {
			icon.Bg = theme.GetColorByName("red_13")
		}
		health := actor.GetStats().GetResource(stats.Health)
		percent := float64(health.Value()) / float64(max(1, health.GetMax()))
		bar := textui.RuneBarFromPercent('*', percent, 5, stageColors)
		lines = append(lines, fmt.Sprintf(" %s %s %s", textui.IconAsString(icon), bar, cview.Escape(actor.Name())))
	}
	return strings.Join(lines, "\n")
}

// common.UserInterface

func (c *Console) ToggleMapInfo() {}

func (c *Console) GetVisibleMap() geometry.Rect {
	r := c.ui.VisibleMap()
	return geometry.NewRect(r.Min.X, r.Min.Y, r.Max.X, r.Max.Y)
}

func (c *Console) IsPlayerForcingAggression() bool { return c.forceAggression }

func (c *Console) HasOverlays() bool { return false }

func (c *Console) ShowLogMessage(message string) {
	if message == "" {
		return
	}
	var lines []string
	for _, line := range c.context.GetLog() {
		lines = append(lines, convertColorCodes(line))
	}
	c.ui.UpdateLogWindow(lines)
}

func (c *Console) Cancel() {
	if c.ui.IsTargeting() {
		c.ui.CancelTargeting()
		return
	}
	c.ui.CloseAllModals()
}

func (c *Console) PopAll() { c.ui.CloseAllModals() }

func (c *Console) ShowGameOverMenu() {
	items := c.context.StartMenuItems(true)
	c.ui.OpenMenuNotClosable("You died", menuItems(items))
}

func (c *Console) QuitGame()                        { c.ui.QuitGame() }
func (c *Console) OpenMenu(options []util.MenuItem) { c.ui.OpenMenu(menuItems(options)) }

func (c *Console) OpenUncancelableMenu(options []util.MenuItem) {
	c.ui.OpenMenuNotClosable("", menuItems(options))
}

func (c *Console) OpenTextWindow(msg []util.LabelText) {
	if len(msg) > 0 {
		c.ui.OpenTextWindow(labelsToText(msg))
	}
}

// OpenInfoWindow renders the sections as plain indented lines in a text window.
func (c *Console) OpenInfoWindow(title string, sections []common.D1SheetSection) {
	c.ui.OpenTextWindow(strings.Join(common.SectionLines(title, sections), "\n"))
}

func (c *Console) ShowItemInfo(item *common.Item) {
	c.OpenTextWindow(item.GetTooltipLines(func(string) []util.ColoredTextPart { return nil }))
}

func (c *Console) CancelTargeting() { c.ui.CancelTargeting() }

func (c *Console) ShowFullStats() { c.fullStats, c.panelsDirty = true, true }

func (c *Console) UpdateStatsLabel() { c.fullStats, c.panelsDirty = false, true }

func (c *Console) OnMapChanged() {
	c.panelsDirty = true
	c.ui.CancelTargeting()
	c.ui.CloseAllModals() // like the tile UI, whose setupUI drops any open widget
	c.ui.CenterMapOnPlayer()
}

// OpenNewGameScreen asks for class, look, name and permadeath one after the other.
func (c *Console) OpenNewGameScreen(start func(class defs.LoadoutDefinition, permadeath bool, name string), back func()) {
	var items []textui.MenuItem
	for _, class := range common.PlayableClasses(c.config, c.context.Definitions.ClassDefinitions) {
		items = append(items, textui.MenuItem{Name: class.Name, CloseMenus: true, Action: func() {
			c.ui.OpenChoiceDialogue("New game", "Choose a look for your "+class.Name, []string{"Male", "Female", "Back"}, func(index int, _ string) {
				if index == 2 {
					c.OpenNewGameScreen(start, back)
					return
				}
				if index == 1 {
					class.Icon = defs.FemaleIconByClass[class.InternalName]
				}
				c.ui.AskForString("Name", common.RandomHeroName(index == 1), func(name string) {
					c.ui.OpenChoiceDialogue("New game", "Permadeath deletes the save when you die.", []string{"Normal", "Permadeath"}, func(mode int, _ string) {
						start(class, mode == 1, name)
					})
				})
			})
		}})
	}
	items = append(items, textui.MenuItem{Name: "Back", Action: back, CloseMenus: true})
	c.ui.OpenMenuNotClosable("Choose a class", items)
}

// OpenSaveSlots lists the slots as a menu; the terminal draws no frames, so there is no screenshot to show.
func (c *Console) OpenSaveSlots(title string, slots []common.SaveSlot, pick func(slot int), back func()) {
	var items []textui.MenuItem
	for _, slot := range slots {
		chosen := slot.Slot
		items = append(items, textui.MenuItem{Name: slot.Summary(), CloseMenus: true, Action: func() { pick(chosen) }})
	}
	items = append(items, textui.MenuItem{Name: "Back", Action: back, CloseMenus: true})
	c.ui.OpenMenuNotClosable(title, items)
}

// CaptureThumbnail: the terminal UI has no framebuffer to screenshot.
func (c *Console) CaptureThumbnail() image.Image { return nil }

func (c *Console) BeginTargeting(rules common.TargetRules, onHit *common.HitEffect) {
	var candidates []mgeom.Point
	for _, enemy := range c.visibleEnemiesByDistance() {
		candidates = append(candidates, mg(enemy.MapPosition()))
	}
	c.ui.SelectTarget(candidates, mg(rules.GetPreSelectedTarget()), func(targetPos mgeom.Point) (mgeom.Point, []mgeom.Point, string) {
		c.lastTarget = rules.OnTargetChanged(rg(targetPos))
		var highlights []mgeom.Point
		for _, highlight := range c.lastTarget.Highlights {
			highlights = append(highlights, mg(highlight.Pos))
		}
		return mg(c.lastTarget.AdjustedTarget), highlights, c.lastTarget.InfoText
	}, func(mgeom.Point) {
		if target := c.lastTarget; target.IsValid {
			onHit.Activate(rules.GetAttacker(), target.AdjustedTarget, target.Hitpositions)
		}
	}, nil)
}

func (c *Console) OpenShopMenu(npc *common.Actor) {
	c.OpenShopMenuFiltered(npc, func(*common.Item) bool { return true })
}

func (c *Console) OpenShopMenuFiltered(vendor *common.Actor, filter func(*common.Item) bool) {
	player := c.context.GameState().GetPlayer()
	gold := player.GetStats().GetAttribute(stats.Gold)
	stacks := vendor.GetInventory().StackedItemsWithFilter(filter)
	if len(stacks) == 0 {
		return
	}
	common.SortInventory(stacks)
	var items []textui.VendorItem
	for _, stack := range stacks {
		item := stack[0]
		items = append(items, textui.VendorItem{
			Name:         itemName(stack, common.ShopItemName(c.config.ItemNamesInShop, stack)),
			Price:        item.BuyPrice(),
			TooExpensive: item.BuyPrice() > gold,
			Buy: func() {
				c.msg(common.BuyItem(player, vendor, item))
				c.OpenShopMenuFiltered(vendor, filter)
			},
			Inspect: func() { c.showItemInfo(item) },
		})
	}
	c.ui.OpenVendorMenu(c.vendorTitle("Buy"), items, nil)
}

func (c *Console) BeginPlayerSellingTo(vendor *common.Actor) {
	player := c.context.GameState().GetPlayer()
	stacks := common.SellableStacks(vendor, player)
	if len(stacks) == 0 {
		c.msg("You have nothing to sell")
		return
	}
	common.SortInventory(stacks)
	var items []textui.VendorItem
	for _, stack := range stacks {
		item := stack[0]
		items = append(items, textui.VendorItem{
			Name:  itemName(stack, common.ShopItemName(c.config.ItemNamesInShop, stack)),
			Price: common.D1SellPrice(item),
			Buy: func() {
				c.msg(common.SellItem(player, item))
				c.BeginPlayerSellingTo(vendor)
			},
			Inspect: func() { c.showItemInfo(item) },
		})
	}
	c.ui.OpenVendorMenu(c.vendorTitle("Sell"), items, nil)
}

func (c *Console) vendorTitle(verb string) string {
	gold := c.context.GameState().GetPlayer().GetStats().GetAttribute(stats.Gold)
	return fmt.Sprintf("%s (%d gold, %s examines)", verb, gold, c.ui.GetKeysForCommandAsString(textui.KeyLayerMain, "examine"))
}

func (c *Console) OpenListInventoryForSelection(filter func(element *common.Item) bool, onSelected func(itemKey rune)) {
	c.ui.CloseAllModals()
	entries := c.inventoryEntries(filter)
	if len(entries) == 0 {
		c.msg("You are not carrying anything.")
		return
	}
	inventory := c.context.GameState().GetPlayer().GetInventory()
	c.ui.OpenInventoryForSelection(entries, "Select an item", func(item textui.InventoryItem) {
		onSelected(inventory.GetLetterForItemStack(item.(invEntry).stack))
	})
}

func (c *Console) OpenItemList(items []*common.TextualStack) {
	var entries []textui.InventoryItem
	for i, item := range items {
		entries = append(entries, invEntry{stack: item.InventoryStack, key: shortcut(i), onSelected: item.OnSelected})
	}
	if len(entries) > 0 {
		c.ui.OpenInventoryForSelection(entries, "Select an item", func(item textui.InventoryItem) { item.(invEntry).onSelected() })
	}
}

func (c *Console) OpenServiceMenu(items []*common.Item, price func(*common.Item) int, pick func(*common.Item)) {
	gold := c.context.GameState().GetPlayer().GetStats().GetAttribute(stats.Gold)
	var vendorItems []textui.VendorItem
	for _, item := range items {
		vendorItems = append(vendorItems, textui.VendorItem{
			Name:         itemName(common.InventoryStack{item}, item.BandStyleName()),
			Price:        price(item),
			TooExpensive: price(item) > gold,
			Buy:          func() { pick(item) },
			Inspect:      func() { c.showItemInfo(item) },
		})
	}
	c.ui.OpenVendorMenu(c.vendorTitle("Service"), vendorItems, nil)
}

func (c *Console) UpdateMiniMap() {}

// OpenPlayerStashInTown shows the pack and the stash side by side; picking an item moves it across.
func (c *Console) OpenPlayerStashInTown() {
	state := c.context.GameState()
	player := state.GetPlayer()
	stash, inventory := state.GetPlayerStashInTown(), player.GetInventory()
	invStacks, stashStacks := inventory.StackedItems(), stash.StackedItems()
	common.SortInventory(invStacks)
	common.SortInventory(stashStacks)
	labels := func(stacks []common.InventoryStack) []string {
		var result []string
		for _, stack := range stacks {
			result = append(result, itemName(stack, stack.BandStyleName()))
		}
		return result
	}
	c.ui.ShowGiveAndTakeContainer("Inventory", labels(invStacks), "Stash", labels(stashStacks), func(index int) {
		item := invStacks[index][0]
		if player.GetEquipment().IsEquipped(item) {
			c.context.PlayerAction().UnequipItem(item)
		}
		inventory.Remove(item)
		stash.Add(item)
		c.OpenPlayerStashInTown()
	}, func(index int) {
		item := stashStacks[index][0]
		stash.Remove(item)
		inventory.Add(item)
		c.OpenPlayerStashInTown()
	}, func() {
		for _, item := range stash.Items() {
			stash.Remove(item)
			inventory.Add(item)
		}
	})
}

func (c *Console) OnPlayerTurnTaken() {
	c.UpdateStatsLabel()
	if c.config.KeepPlayerCentered {
		c.ui.CenterMapOnPlayer()
	} else {
		c.ui.UpdateMapScrollPosition()
	}
}

func (c *Console) SetPlayerMessage(message func(message string)) { c.msg = message }

func (c *Console) SetDebugActions(actions func() common.DebugActions) {
	c.context.DebugActions = actions
}

func (c *Console) FadeToBlack() { c.ui.FadeToBlack() }

func (c *Console) FadeFromBlack() { c.ui.FadeFromBlack() }

func (c *Console) Confirm(message string, onConfirm func()) {
	c.ui.AskForConfirmation("Confirm", message, func(didConfirm bool) {
		if didConfirm {
			onConfirm()
		}
	})
}

// Text

func menuItems(options []util.MenuItem) []textui.MenuItem {
	var items []textui.MenuItem
	for _, option := range options {
		name := cview.Escape(option.MainText)
		if option.UseColorCodes && len(option.MainTextWithColorCodes) > 0 {
			name = coloredPartsToText(option.MainTextWithColorCodes)
		} else if rgba, ok := option.MainTextColor.(color.RGBA); ok {
			name = colorTag(rgba) + name + "[-]"
		}
		items = append(items, textui.MenuItem{Name: name, Action: option.Action, CloseMenus: true})
	}
	return items
}

func labelsToText(lines []util.LabelText) string {
	var result []string
	for _, line := range lines {
		result = append(result, labelToText(line))
	}
	return strings.Join(result, "\n")
}

func labelToText(line util.LabelText) string {
	if len(line.TextWithColorCodes) > 0 {
		return coloredPartsToText(line.TextWithColorCodes)
	}
	if line.TextColor != nil {
		return colorTag(line.TextColor) + cview.Escape(line.Text) + "[-]"
	}
	return cview.Escape(line.Text)
}

func coloredPartsToText(parts []util.ColoredTextPart) string {
	var result []string
	for _, part := range parts {
		result = append(result, colorTag(part.Color)+cview.Escape(part.Text))
	}
	return strings.Join(result, " ") + "[-]"
}

func colorTag(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return textiles.RGBAToFgColorCode(color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 255})
}

var colorCodePattern = regexp.MustCompile(`\[:([\w,]+)\]`)

// convertColorCodes turns the game's [:red] and [:24,54,222] codes into cview color tags.
func convertColorCodes(message string) string {
	parts := colorCodePattern.Split(message, -1)
	codes := colorCodePattern.FindAllStringSubmatch(message, -1)
	var result strings.Builder
	result.WriteString(cview.Escape(parts[0]))
	for i, code := range codes {
		result.WriteString(colorTag(colorFromCode(code[1])))
		result.WriteString(cview.Escape(parts[i+1]))
	}
	return result.String() + "[-]"
}

func colorFromCode(code string) color.RGBA {
	values := strings.Split(code, ",")
	if len(values) != 3 {
		return defs.ColorFromName(code)
	}
	r, _ := strconv.Atoi(values[0])
	g, _ := strconv.Atoi(values[1])
	b, _ := strconv.Atoi(values[2])
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
}

func mg(p geometry.Point) mgeom.Point { return mgeom.Point{X: p.X, Y: p.Y} }

func rg(p mgeom.Point) geometry.Point { return geometry.Point{X: p.X, Y: p.Y} }
