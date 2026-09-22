package game

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"
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
			return headings(common.SectionLines(common.ActorInfoSections(&g.state, g.state.GetPlayer())))
		}},
		{c.FeedVisibleItems, "visible_items", g.visibleItemsFeed},
		{c.FeedVisibleActors, "visible_actors", g.visibleActorsFeed},
		{c.FeedQuests, "quests", func() []string { return headings(g.state.D1QuestLog()) }},
		{c.FeedMinimap, "minimap", g.minimapFeed},
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

// minimapFeed draws the minimap's colors with half blocks: each character is two cells stacked, the upper one
// its foreground and the lower one its background, so the map keeps its shape in a terminal.
// ponytail: the whole map, unexplored margins included; crop to the explored bounds if it gets too wide.
func (g *GameInstance) minimapFeed() []string {
	size := g.state.GetMap().MapSize()
	color := func(x, y int) [4]byte {
		return common.MiniMapColor(&g.state, g.configuration, geometry.Point{X: x, Y: y})
	}
	var lines []string
	for y := 0; y < size.Y; y += 2 {
		var line strings.Builder
		for x := 0; x < size.X; x++ {
			top, bottom := color(x, y), [4]byte{}
			if y+1 < size.Y {
				bottom = color(x, y+1)
			}
			fmt.Fprintf(&line, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", top[0], top[1], top[2], bottom[0], bottom[1], bottom[2])
		}
		lines = append(lines, line.String())
	}
	return lines
}

// headings colors the unindented lines of a text window (titles, quest names) gold and the labels of its
// indented "label: value" rows gray.
func headings(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		switch {
		case line == "":
		case !strings.HasPrefix(line, " "):
			line = "[:lightText]" + line + "[:white]"
		default:
			if label, value, ok := strings.Cut(line, ": "); ok {
				line = "[:darkText]" + label + ":[:white] " + value
			}
		}
		out[i] = line
	}
	return out
}

var colorCode = regexp.MustCompile(`\[:([\w,]+)\]`)

var ansiColors = map[string]string{
	"white": "37", "red": "31",
	"darkText": "38;2;175;131;59", "lightText": "38;2;224;176;33", // defs.DarkTextColor, defs.LightTextColor
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
