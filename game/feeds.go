package game

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"rpg/common"
	"rpg/defs"
	"slices"
	"strings"
)

// feedDir holds the text feeds: one ANSI file per enabled feed, for `tail -F`, `watch cat` or a file watcher
// in another terminal.
const feedDir = "feeds"

// writeFeeds rewrites each enabled feed whose text changed since its last write. Called after every player turn
// and whenever the pack or the gear changes.
func (g *GameInstance) writeFeeds() {
	if g.state.GetPlayer() == nil {
		return
	}
	c := g.configuration
	feeds := []struct {
		on     bool
		name   string
		render func() []string
	}{
		{c.FeedInventory, "inventory", g.state.GetPlayer().GetInventory().AsColorCodedStrings},
		{c.FeedEquipment, "equipment", g.equipmentFeed},
		{c.FeedCharacter, "character", func() []string {
			return common.SectionLines(common.ActorInfoSections(&g.state, g.state.GetPlayer()))
		}},
		{c.FeedVisibleItems, "visible_items", g.visibleItemsFeed},
		{c.FeedVisibleActors, "visible_actors", g.visibleActorsFeed},
		{c.FeedQuests, "quests", g.state.D1QuestLog},
	}
	if g.feeds == nil {
		g.feeds = map[string]string{}
	}
	for _, f := range feeds {
		if !f.on {
			delete(g.feeds, f.name) // switched back on, it writes again even if nothing changed
			continue
		}
		text := ansi(strings.Join(f.render(), "\n")) + "\n"
		if g.feeds[f.name] == text {
			continue
		}
		g.feeds[f.name] = text
		writeFeedFile(f.name, text)
	}
}

// writeFeedFile replaces the file in one rename, so a reader never sees it half written.
func writeFeedFile(name, text string) {
	path := filepath.Join(feedDir, name+".ans")
	if err := os.MkdirAll(feedDir, 0o755); err != nil {
		println("ERR: feed " + name + ": " + err.Error())
		return
	}
	if err := os.WriteFile(path+".tmp", []byte(text), 0o644); err != nil {
		println("ERR: feed " + name + ": " + err.Error())
		return
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		println("ERR: feed " + name + ": " + err.Error())
	}
}

// appendLogFeed adds one message to feeds/log.ans; unlike the others this feed only grows, so `tail -f` follows it.
func (g *GameInstance) appendLogFeed(message string) {
	if !g.configuration.FeedLog {
		return
	}
	if err := os.MkdirAll(feedDir, 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(feedDir, "log.ans"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		println("ERR: feed log: " + err.Error())
		return
	}
	defer f.Close()
	f.WriteString(ansi(message) + "\n")
}

// equipmentFeed: one line per equipped item, its pack letter, name and slot, sorted by slot.
func (g *GameInstance) equipmentFeed() []string {
	equipment := g.state.GetPlayer().GetEquipment()
	inventory := g.state.GetPlayer().GetInventory()
	items := equipment.AllItems()
	slices.SortStableFunc(items, func(a, b *common.Item) int { return cmp.Compare(equipment.GetSlot(a), equipment.GetSlot(b)) })
	width := 0
	for _, item := range items {
		width = max(width, len(item.InventoryName()))
	}
	var lines []string
	for _, item := range items {
		name := item.InventoryName()
		lines = append(lines, fmt.Sprintf("[:white]%c) %s%s%s [:white]<-- %s", inventory.GetLetterForItem(item),
			defs.ColorCodeFromColor(item.TintColor()), name, strings.Repeat(" ", width-len(name)), equipment.GetSlot(item).ToString()))
	}
	return lines
}

// visibleItemsFeed: the items on the floor the player can see, nearest first.
func (g *GameInstance) visibleItemsFeed() []string {
	player := g.state.GetPlayer()
	var items []*common.Item
	for _, item := range g.state.GetMap().Items() {
		if player.CanSee(item.MapPosition()) {
			items = append(items, item)
		}
	}
	pos := player.MapPosition()
	slices.SortFunc(items, func(a, b *common.Item) int {
		return cmp.Compare(a.MapPosition().ManhattanDistanceTo(pos), b.MapPosition().ManhattanDistanceTo(pos))
	})
	var lines []string
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s%s [:white](%d)", defs.ColorCodeFromColor(item.TintColor()), item.InventoryName(), item.MapPosition().ManhattanDistanceTo(pos)))
	}
	return lines
}

// visibleActorsFeed: who the player sees, nearest first; enemies in red.
func (g *GameInstance) visibleActorsFeed() []string {
	player := g.state.GetPlayer()
	vis := g.state.GetVisibility()
	var actors []*common.Actor
	for actor := range vis.GetVisibleActors() {
		if actor != player {
			actors = append(actors, actor)
		}
	}
	pos := player.MapPosition()
	slices.SortFunc(actors, func(a, b *common.Actor) int { // map order is random: break ties by position
		ap, bp := a.MapPosition(), b.MapPosition()
		return cmp.Or(ap.ManhattanDistanceTo(pos)-bp.ManhattanDistanceTo(pos), ap.Y-bp.Y, ap.X-bp.X)
	})
	enemies := vis.GetVisibleEnemies()
	var lines []string
	for _, actor := range actors {
		color := "[:white]"
		if slices.Contains(enemies, actor) {
			color = "[:red]"
		}
		lines = append(lines, fmt.Sprintf("%s%s [:white](%d)", color, actor.Name(), actor.MapPosition().ManhattanDistanceTo(pos)))
	}
	return lines
}

var colorCode = regexp.MustCompile(`\[:([\w,]+)\]`)

var ansiColors = map[string]string{
	"white": "37", "red": "31", "green": "32", "yellow": "33", "blue": "34",
	"darkText": "90", "lightText": "97", "magic": "94", "rare": "93", "set": "92", "unique": "33",
}

// ansi turns the game's color codes ([:red], [:24,54,222]) into ANSI escapes, resetting at each line end.
func ansi(text string) string {
	text = colorCode.ReplaceAllStringFunc(text, func(code string) string {
		name := colorCode.FindStringSubmatch(code)[1]
		if sgr, ok := ansiColors[name]; ok {
			return "\x1b[" + sgr + "m"
		}
		var r, g, b int
		if n, _ := fmt.Sscanf(name, "%d,%d,%d", &r, &g, &b); n == 3 {
			return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
		}
		return "\x1b[0m"
	})
	return strings.ReplaceAll(text, "\n", "\x1b[0m\n") + "\x1b[0m"
}
