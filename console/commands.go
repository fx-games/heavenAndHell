package console

import (
	"strings"

	"github.com/memmaker/go/cview"
	mgeom "github.com/memmaker/go/geometry"
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"
	"rpg/textui"
	"rpg/util"
)

// setupCommands binds the commands of files/assets/data/ascii/keymap.txt to the game.
func (c *Console) setupCommands() {
	act := func() common.PlayerActions { return c.context.PlayerAction() }
	debug := func() common.DebugActions { return c.context.DebugActions() }
	move := func(dx, dy int, force bool) func() {
		return func() {
			c.forceAggression = force
			act().ManualMovePlayer(geometry.Point{X: dx, Y: dy})
			c.forceAggression = false
		}
	}
	commands := map[string]func(){
		"north":       move(0, -1, false),
		"south":       move(0, 1, false),
		"west":        move(-1, 0, false),
		"east":        move(1, 0, false),
		"force_north": move(0, -1, true),
		"force_south": move(0, 1, true),
		"force_west":  move(-1, 0, true),
		"force_east":  move(1, 0, true),

		"wait":         func() { act().EndTurn(10) },
		"pickup":       func() { act().PickUpItem() },
		"auto_explore": func() { act().StartAutoExplore() },
		"auto_attack":  func() { act().AutoAttack() },
		"force_auto_attack": func() {
			c.forceAggression = true
			act().AutoAttack()
			c.forceAggression = false
		},
		"rest": func() { act().RestUntilHealed() },

		"aimed_ranged_attack": func() { act().AimedRangedAttack() },
		"quick_ranged_attack": func() { act().QuickRangedAttack() },
		"throw":               func() { act().AimedThrow() },
		"spells":              c.openSpellMenu,
		"recast_spell":        func() { act().CastLastSpellAtNearest() },

		"inventory":  c.openInventory,
		"character":  c.ShowFullStats, // the side panel, like the tile UI's character sheet
		"level_up":   c.openLevelUpMenu,
		"log":        c.showLog,
		"quest_log":  func() { c.OpenTextWindow(util.ToLabelText(c.context.GameState().D1QuestLog())) },
		"center_map": c.ui.CenterMapOnPlayer,
		"look":       c.look,
		"examine":    c.look,

		"scroll_map_up":    func() { c.ui.ScrollMapBy(mgeom.Point{Y: -8}) },
		"scroll_map_down":  func() { c.ui.ScrollMapBy(mgeom.Point{Y: 8}) },
		"scroll_map_left":  func() { c.ui.ScrollMapBy(mgeom.Point{X: -8}) },
		"scroll_map_right": func() { c.ui.ScrollMapBy(mgeom.Point{X: 8}) },

		"system_menu": c.openSystemMenu,
		"help":        c.showHelp,

		"flags":          func() { c.OpenTextWindow(util.ToLabelText(c.context.GameState().GetFlags().AsStrings())) },
		"wiz_kill_level": func() { debug().KillLevel() },
		"wiz_level_up":   func() { debug().GoOneLevelUp() },
		"wiz_level_down": func() { debug().GoOneLevelDown() },
		"wiz_fog_of_war": func() { c.disableFog = !c.disableFog },
		"wizard":         func() { debug().OpenDebugMenu() },
	}
	for command, action := range commands {
		c.ui.SetCommand(command, action)
	}
}

var commandNames = map[string]string{
	"north": "North", "south": "South", "west": "West", "east": "East",
	"force_north": "Attack north", "force_south": "Attack south", "force_west": "Attack west", "force_east": "Attack east",
	"wait": "Wait", "pickup": "Pick up", "auto_explore": "Auto explore", "auto_attack": "Attack nearest",
	"force_auto_attack": "Attack nearest (any)", "rest": "Rest until healed",
	"aimed_ranged_attack": "Aimed ranged attack", "quick_ranged_attack": "Ranged attack nearest", "throw": "Throw",
	"spells": "Cast a spell", "recast_spell": "Recast at nearest",
	"inventory": "Inventory", "character": "Character", "level_up": "Spend stat points", "log": "Message log",
	"quest_log": "Quest log", "look": "Look", "examine": "Examine", "system_menu": "Pause menu", "help": "Help",
	"center_map": "Center the map", "scroll_map_up": "Scroll the map", "scroll_map_down": "Scroll the map",
	"scroll_map_left": "Scroll the map", "scroll_map_right": "Scroll the map",
	"flags": "Flags", "wiz_kill_level": "Wizard: kill level", "wiz_level_up": "Wizard: level up",
	"wiz_level_down": "Wizard: level down", "wiz_fog_of_war": "Wizard: fog of war", "wizard": "Wizard menu",
}

func (c *Console) showHelp() {
	c.ui.ShowKeyBindings(commandNames,
		[]string{"north", "south", "west", "east", "force_north", "force_south", "force_west", "force_east", "wait", "rest", "pickup", "auto_explore", "look", "center_map", "scroll_map_up", "system_menu", "help"},
		[]string{"auto_attack", "force_auto_attack", "aimed_ranged_attack", "quick_ranged_attack", "throw", "spells", "recast_spell", "inventory", "character", "level_up", "log", "quest_log", "flags", "wizard"},
	)
}

func (c *Console) showLog() {
	var lines []string
	for _, line := range c.context.GetLog() {
		lines = append(lines, convertColorCodes(line))
	}
	c.ui.ShowLog(lines)
}

func (c *Console) openSpellMenu() {
	items := common.SpellMenuItems(c.context.GameState(), c.context.PlayerAction())
	if len(items) == 0 {
		c.msg("You don't know any spells and carry no scrolls or staves.")
		return
	}
	c.ui.OpenMenuWithTitle("Spells", menuItems(items))
}

func (c *Console) openLevelUpMenu() {
	player := c.context.GameState().GetPlayer()
	if !player.CanLevelUp() {
		return
	}
	c.ShowFullStats()
	c.ui.OpenMenuWithTitle("Level up", menuItems(common.LevelUpMenuItems(player, c.openLevelUpMenu, c.ShowFullStats)))
}

func (c *Console) openSystemMenu() {
	if c.ui.IsTargeting() {
		c.ui.CancelTargeting()
		return
	}
	c.OpenMenu([]util.MenuItem{
		{MainText: "Resume"},
		{MainText: "Auto explore", Action: func() {
			c.OpenMenu(append(common.AutoExploreSettings(c.config, c.openSystemMenu), util.MenuItem{MainText: "Back", Action: c.openSystemMenu}))
		}},
		{MainText: "Gameplay", Action: func() {
			c.OpenMenu(append(common.GameplaySettings(c.config, c.openSystemMenu), util.MenuItem{MainText: "Back", Action: c.openSystemMenu}))
		}},
		{MainText: "Key bindings", Action: c.showHelp},
		{MainText: "Quit game", Action: c.ui.QuitGame},
	})
}

// look moves a cursor over the map; confirming shows details of what is there, like right clicking in the tile UI.
func (c *Console) look() {
	var candidates []mgeom.Point
	for _, actor := range c.visibleActorsByDistance() {
		candidates = append(candidates, mg(actor.MapPosition()))
	}
	start := c.GetPlayerPosition()
	if len(candidates) > 0 {
		start = candidates[0]
	}
	c.ui.SelectTarget(candidates, start, func(targetPos mgeom.Point) (mgeom.Point, []mgeom.Point, string) {
		return targetPos, nil, strings.TrimSpace(c.describe(targetPos))
	}, c.showDetailsAt, nil)
}

func (c *Console) showDetailsAt(loc mgeom.Point) {
	pos := rg(loc)
	currentMap := c.context.GameState().GetMap()
	if actor := c.visibleActorAt(pos); actor != nil {
		c.context.PlayerAction().ShowActorInfoAt(pos)
	} else if currentMap.Contains(pos) && c.canSeeCell(pos) && currentMap.IsItemAt(pos) {
		c.showItemInfo(currentMap.ItemAt(pos))
	}
}

// Mouse

func (c *Console) onMapClicked(loc mgeom.Point, isRightClick bool) {
	pos := rg(loc)
	currentMap := c.context.GameState().GetMap()
	if !currentMap.Contains(pos) {
		return
	}
	if isRightClick {
		if actor := c.visibleActorAt(pos); actor != nil {
			c.context.PlayerAction().ShowActorInfoAt(pos)
		}
	} else if currentMap.IsExplored(pos) {
		c.context.PlayerAction().CancelAutoMove()
		c.context.PlayerAction().StartPlayerAutoMovement(pos)
	}
}

// onRightPanelClicked uses the clicked inventory line, drops it with a modifier and examines it with a right click.
func (c *Console) onRightPanelClicked(line int, isRightClick bool, modified bool) {
	entries := c.inventoryEntries(nil)
	if c.fullStats || line < 0 || line >= len(entries) {
		return
	}
	item := entries[line].(invEntry).stack[0]
	if isRightClick || modified {
		c.context.PlayerAction().DropItem(item)
		return
	}
	c.useItem(item, nil)
}

// describeInventoryLine names the item under the mouse, like the tile UI's info label.
func (c *Console) describeInventoryLine(line int) string {
	entries := c.inventoryEntries(nil)
	if c.fullStats || line < 0 || line >= len(entries) {
		return " "
	}
	return entries[line].(invEntry).stack[0].Name()
}

// Inventory

type invEntry struct {
	stack      common.InventoryStack
	key        rune
	usable     func() bool
	onSelected func()
}

func (e invEntry) Shortcut() rune { return e.key }

func (e invEntry) InventoryName() string { return itemName(e.stack, e.stack.BandStyleName()) }

func (e invEntry) RightColumn() string { return "" }

func (e invEntry) IsUsable() bool { return e.usable != nil && e.usable() }

// shortcut is a-z, then 0-9: capital letters mean shift in the inventory.
func shortcut(index int) rune {
	if index < 26 {
		return 'a' + rune(index)
	}
	if index < 36 {
		return '0' + rune(index-26)
	}
	return ' '
}

func itemName(stack common.InventoryStack, name string) string {
	nameColor := stack.TintColor()
	if !stack[0].MeetsRequirements() {
		nameColor = defs.Red
	}
	return colorTag(nameColor) + cview.Escape(name) + "[-]"
}

func (c *Console) inventoryEntries(filter func(*common.Item) bool) []textui.InventoryItem {
	player := c.context.GameState().GetPlayer()
	inventory := player.GetInventory()
	stacks := inventory.StackedItems()
	if filter != nil {
		stacks = inventory.StackedItemsWithFilter(filter)
	}
	common.SortInventory(stacks)
	var entries []textui.InventoryItem
	for i, stack := range stacks {
		item := stack[0]
		entries = append(entries, invEntry{stack: stack, key: shortcut(i), usable: func() bool {
			return item.IsEquippable() || len(c.context.PlayerAction().GetContextActionsForItem(item)) > 0
		}})
	}
	return entries
}

func (c *Console) openInventory() {
	player := c.context.GameState().GetPlayer()
	if player.GetInventory().IsEmpty() {
		c.msg("You are not carrying anything.")
		return
	}
	act := c.context.PlayerAction()
	firstItem := func(entry textui.InventoryItem) *common.Item { return entry.(invEntry).stack[0] }
	var inventory *textui.TextInventory
	inventory = c.ui.OpenInventoryForManagement(
		func() []textui.InventoryItem { return c.inventoryEntries(nil) },
		func(entry textui.InventoryItem) bool { return player.GetEquipment().IsEquipped(firstItem(entry)) },
		func(entry textui.InventoryItem) { c.useItem(firstItem(entry), inventory) },
		func(entry textui.InventoryItem) { act.DropItem(firstItem(entry)) },
		func(entry textui.InventoryItem) { c.showItemInfo(firstItem(entry)) },
		func(entry textui.InventoryItem, done func()) {
			item := firstItem(entry)
			actions := common.ItemActions(act, player, item, c.ShowFullStats)
			actions = append(actions,
				util.MenuItem{MainText: "Drop", Action: func() { act.DropItem(item) }},
				util.MenuItem{MainText: "Examine", Action: func() { c.showItemInfo(item) }},
			)
			inventory.Close()
			c.OpenMenu(actions)
		},
	)
}

// useItem equips or uses item like clicking it in the tile UI. Anything but equipping closes the inventory first,
// since using an item may start targeting or open a menu.
func (c *Console) useItem(item *common.Item, inventory *textui.TextInventory) {
	player := c.context.GameState().GetPlayer()
	actions := common.ItemActions(c.context.PlayerAction(), player, item, c.ShowFullStats)
	if len(actions) == 0 {
		return
	}
	if inventory != nil && (len(actions) > 1 || !item.IsEquippable()) {
		inventory.Close()
	}
	if len(actions) == 1 {
		actions[0].Action()
		return
	}
	c.OpenMenu(actions)
}

// showItemInfo is the tile UI's item tooltip: the item, compared to what equipping it would replace.
func (c *Console) showItemInfo(item *common.Item) {
	parse := parseColorCodes
	equipment := c.context.GameState().GetPlayer().GetEquipment()
	equipped, comparison := equipment.D1CompareForEquipping(item)
	if equipment.IsEquipped(item) || len(equipped) == 0 {
		c.OpenTextWindow(item.GetTooltipLines(parse))
		return
	}
	lines := item.D1TooltipLinesComparedTo(comparison, parse)
	for _, e := range equipped {
		lines = append(lines, util.LabelTextFromString(""), util.LabelTextFromStringWithColor("Equipped:", defs.OffWhite))
		lines = append(lines, e.Item.D1TooltipLinesComparedTo(e.Cmp, parse)...)
		if e.Note != "" {
			lines = append(lines, util.LabelTextFromStringWithColor(e.Note, defs.NegativeHighlightColor))
		}
	}
	c.OpenTextWindow(lines)
}

// parseColorCodes splits [:red] coded text into colored parts for the game's tooltip builders.
func parseColorCodes(text string) []util.ColoredTextPart {
	parts := colorCodePattern.Split(text, -1)
	codes := colorCodePattern.FindAllStringSubmatch(text, -1)
	var result []util.ColoredTextPart
	current := defs.White
	for i, part := range parts {
		if i > 0 {
			current = colorFromCode(codes[i-1][1])
		}
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, util.ColoredTextPart{Text: part, Color: current})
		}
	}
	return result
}
