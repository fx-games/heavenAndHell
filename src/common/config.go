package common

import (
	"cmp"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"rpg/files"
	"rpg/src/geometry"
	"rpg/src/recfile"
	"rpg/src/util"
	"slices"
	"strconv"
	"strings"
)

type FontInfo struct {
	TTFFile string
	TTFSize float64
}

type StartInfo struct {
	FontInfos         FontInfo
	ScreenSize        geometry.Point
	TileScale         float64
	Title             string
	LDTKMapFile       string
	SpriteSheet       string
	UserConfiguration *Configuration
	FileSystem        fs.ReadDirFS
}

type Configuration struct {
	AudioEnabled         bool
	ActorBobbingEnabled  bool
	WindowSize           geometry.Point
	Font                 string // the player's pick among the mod's fonts/ (a file name); empty: the mod's Font
	DefaultFontSize      float64
	AutoPickup           bool
	CardinalMovementOnly bool
	KeepPlayerCentered   bool

	DrawTilesBehindActors  bool
	DrawTilesBehindItems   bool
	DrawTilesBehindObjects bool
	// text feeds: ANSI files under feeds/, rewritten when their content changes (see game/feeds.go)
	FeedInventory          bool
	FeedEquipment          bool
	FeedCharacter          bool
	FeedVisibleItems       bool
	FeedVisibleActors      bool
	FeedLog                bool
	FeedQuests             bool
	FeedMinimap            bool
	HideSidebar            bool // the tile UI's right-hand bar; the map takes its room
	FlipActorsHorizontally bool

	SoundVolume, MusicVolume, VoiceVolume float64 // 0..1, set in the audio menu

	AutoExploreOpenContainers bool // auto explore also bumps chests/barrels/sarcophagi it finds

	AutosaveOnLevelChange bool // the player's choice; a mod's data/mod.rec may fix it (AutosavesOnLevelChange)

	Fullscreen bool // set in the video menu or with F11

	// SpriteSheet is the tile atlas the Icon* numbers index. A mod ships its own at assets/, where the overlay
	// finds it, or names another path here; the engine's default is the fallback.
	SpriteSheet string

	// Mod names the game to play: files/mods/<Mod> shadows the engine's own assets/. One mod is one complete game.
	Mod string

	// MiniMapColors overrides minimap pixel colors by name, from repeated
	// "MiniMapColor: name=rrggbb" lines. Unknown names are kept and written back, but never drawn.
	MiniMapColors map[string][4]byte
}

// parseMiniMapColors reads the comma-joined "name=rrggbb" values of the MiniMapColor field.
func parseMiniMapColors(joined string) map[string][4]byte {
	colors := make(map[string][4]byte)
	for _, entry := range strings.Split(joined, ",") {
		name, hex, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if !ok {
			continue
		}
		hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
		rgb, err := strconv.ParseUint(hex, 16, 32)
		if err != nil || len(hex) != 6 {
			continue
		}
		colors[strings.TrimSpace(name)] = [4]byte{byte(rgb >> 16), byte(rgb >> 8), byte(rgb), 255}
	}
	return colors
}

func (c Configuration) ToRecord() recfile.Record {
	record := recfile.Record{
		{
			Name:  "AudioEnabled",
			Value: recfile.BoolStr(c.AudioEnabled),
		},
		{
			Name:  "ActorBobbingEnabled",
			Value: recfile.BoolStr(c.ActorBobbingEnabled),
		},
		{
			Name:  "DrawTilesBehindActors",
			Value: recfile.BoolStr(c.DrawTilesBehindActors),
		},
		{
			Name:  "DrawTilesBehindItems",
			Value: recfile.BoolStr(c.DrawTilesBehindItems),
		},
		{
			Name:  "DrawTilesBehindObjects",
			Value: recfile.BoolStr(c.DrawTilesBehindObjects),
		},
		{
			Name:  "Font",
			Value: c.Font,
		},
		{
			Name:  "DefaultFontSize",
			Value: recfile.FloatStr(c.DefaultFontSize),
		},
		{
			Name:  "AutoPickup",
			Value: recfile.BoolStr(c.AutoPickup),
		},
		{
			Name:  "WindowWidth",
			Value: recfile.IntStr(c.WindowSize.X),
		},
		{
			Name:  "WindowHeight",
			Value: recfile.IntStr(c.WindowSize.Y),
		},
		{
			Name:  "CardinalMovementOnly",
			Value: recfile.BoolStr(c.CardinalMovementOnly),
		},
		{
			Name:  "KeepPlayerCentered",
			Value: recfile.BoolStr(c.KeepPlayerCentered),
		},
		{Name: "FeedInventory", Value: recfile.BoolStr(c.FeedInventory)},
		{Name: "FeedEquipment", Value: recfile.BoolStr(c.FeedEquipment)},
		{Name: "FeedCharacter", Value: recfile.BoolStr(c.FeedCharacter)},
		{Name: "FeedVisibleItems", Value: recfile.BoolStr(c.FeedVisibleItems)},
		{Name: "FeedVisibleActors", Value: recfile.BoolStr(c.FeedVisibleActors)},
		{Name: "FeedLog", Value: recfile.BoolStr(c.FeedLog)},
		{Name: "FeedQuests", Value: recfile.BoolStr(c.FeedQuests)},
		{Name: "FeedMinimap", Value: recfile.BoolStr(c.FeedMinimap)},
		{Name: "HideSidebar", Value: recfile.BoolStr(c.HideSidebar)},
		{
			Name:  "FlipActorsHorizontally",
			Value: recfile.BoolStr(c.FlipActorsHorizontally),
		},
		{Name: "SoundVolume", Value: recfile.FloatStr(c.SoundVolume)},
		{Name: "MusicVolume", Value: recfile.FloatStr(c.MusicVolume)},
		{Name: "VoiceVolume", Value: recfile.FloatStr(c.VoiceVolume)},
		{Name: "AutoExploreOpenContainers", Value: recfile.BoolStr(c.AutoExploreOpenContainers)},
		{Name: "AutosaveOnLevelChange", Value: recfile.BoolStr(c.AutosaveOnLevelChange)},
		{Name: "Fullscreen", Value: recfile.BoolStr(c.Fullscreen)},
		{Name: "Mod", Value: c.Mod},
		{Name: "SpriteSheet", Value: c.SpriteSheet},
	}
	// written back on every save, so a menu toggle cannot drop the user's palette
	for _, name := range slices.Sorted(maps.Keys(c.MiniMapColors)) {
		rgba := c.MiniMapColors[name]
		record = append(record, recfile.Field{Name: "MiniMapColor", Value: fmt.Sprintf("%s=%02x%02x%02x", name, rgba[0], rgba[1], rgba[2])})
	}
	return record
}

func (c Configuration) SaveToRecFile(recFilename string) {
	file, err := os.Create(recFilename)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	recfile.Write(file, []recfile.Record{c.ToRecord()})
}
func NewConfigurationFromRecFile(recFilename string) Configuration {
	println(fmt.Sprintf("Loading configuration from %s", recFilename))
	file, err := os.Open(recFilename)
	configExists := false
	var configMap recfile.DataMap

	if err == nil {
		records := recfile.Read(file)
		if len(records) > 0 {
			configMap = records[0].ToMap(",")
			configExists = true
		}
	}

	configuration := Configuration{
		AudioEnabled:              configMap.GetBoolOrFalse("AudioEnabled"),
		ActorBobbingEnabled:       configMap.GetBoolOrTrue("ActorBobbingEnabled"),
		CardinalMovementOnly:      configMap.GetBoolOrFalse("CardinalMovementOnly"),
		Font:                      configMap.GetStringOrDefault("Font", ""),
		DefaultFontSize:           configMap.GetFloatOrDefault("DefaultFontSize", 16.0),
		AutoPickup:                configMap.GetBoolOrTrue("AutoPickup"),
		AutosaveOnLevelChange:     configMap.GetBoolOrTrue("AutosaveOnLevelChange"),
		KeepPlayerCentered:        configMap.GetBoolOrTrue("KeepPlayerCentered"),
		DrawTilesBehindActors:     configMap.GetBoolOrFalse("DrawTilesBehindActors"),
		DrawTilesBehindItems:      configMap.GetBoolOrFalse("DrawTilesBehindItems"),
		DrawTilesBehindObjects:    configMap.GetBoolOrFalse("DrawTilesBehindObjects"),
		FeedInventory:             configMap.GetBoolOrFalse("FeedInventory"),
		FeedEquipment:             configMap.GetBoolOrFalse("FeedEquipment"),
		FeedCharacter:             configMap.GetBoolOrFalse("FeedCharacter"),
		FeedVisibleItems:          configMap.GetBoolOrFalse("FeedVisibleItems"),
		FeedVisibleActors:         configMap.GetBoolOrFalse("FeedVisibleActors"),
		FeedLog:                   configMap.GetBoolOrFalse("FeedLog"),
		FeedQuests:                configMap.GetBoolOrFalse("FeedQuests"),
		FeedMinimap:               configMap.GetBoolOrFalse("FeedMinimap"),
		HideSidebar:               configMap.GetBoolOrFalse("HideSidebar"),
		FlipActorsHorizontally:    configMap.GetBoolOrTrue("FlipActorsHorizontally"),
		SoundVolume:               configMap.GetFloatOrDefault("SoundVolume", 1),
		MusicVolume:               configMap.GetFloatOrDefault("MusicVolume", 1),
		VoiceVolume:               configMap.GetFloatOrDefault("VoiceVolume", 1),
		AutoExploreOpenContainers: configMap.GetBoolOrFalse("AutoExploreOpenContainers"),
		Fullscreen:                configMap.GetBoolOrFalse("Fullscreen"),
		Mod:                       cmp.Or(configMap["Mod"], DefaultMod), // an empty "Mod:" is no mod named either
		SpriteSheet:               configMap.GetStringOrDefault("SpriteSheet", DefaultSpriteSheet),
		MiniMapColors:             parseMiniMapColors(configMap.GetStringOrDefault("MiniMapColor", "")),
		WindowSize: geometry.Point{ //1280, Y: 800
			X: configMap.GetIntOrDefault("WindowWidth", 1280),
			Y: configMap.GetIntOrDefault("WindowHeight", 800),
		},
	}
	if !configExists {
		configuration.SaveToRecFile(recFilename)
	}
	return configuration
}

// DefaultMod is the game played when config.rec does not name one, and DefaultSpriteSheet where a mod keeps its
// atlas: the overlay maps assets/ to the active mod.
const (
	DefaultMod         = "heavenandhell"
	DefaultSpriteSheet = "assets/classic_roguelike.png"
)

// ConfigFile is the user configuration next to the game.
var ConfigFile = files.AppPath("config.rec")

// SaveVolumes writes the volume settings into the config file in place, keeping its other lines and comments.
func (c Configuration) SaveVolumes(recFilename string) error {
	return setConfigFields(recFilename, [][2]string{{"SoundVolume", recfile.FloatStr(c.SoundVolume)}, {"MusicVolume", recfile.FloatStr(c.MusicVolume)}, {"VoiceVolume", recfile.FloatStr(c.VoiceVolume)}})
}

// SaveMod writes the mod to start with next time into the config file, keeping its other lines.
func SaveMod(recFilename, mod string) error {
	return setConfigFields(recFilename, [][2]string{{"Mod", mod}})
}

// setConfigFields replaces each field's line in place. A missing field goes before the first field, past the header: only
// the first record is read, lines above its header are not part of it, and a line appended after a trailing
// blank line would be ignored too.
func setConfigFields(recFilename string, fields [][2]string) error {
	data, err := os.ReadFile(recFilename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	header := 0
	for header < len(lines) && (lines[header] == "" || strings.HasPrefix(lines[header], "%") || strings.HasPrefix(lines[header], "#")) {
		header++
	}
	for _, kv := range fields {
		line, found := kv[0]+": "+kv[1], false
		for i, l := range lines {
			if strings.HasPrefix(l, kv[0]+":") {
				lines[i], found = line, true
			}
		}
		if !found {
			lines = append(lines[:header], append([]string{line}, lines[header:]...)...)
		}
	}
	return os.WriteFile(recFilename, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// ModSetting is a setting the active mod fixes in its data/mod.rec, and whether it does. The player can't change
// such a setting: the mod's value wins over config.rec.
func ModSetting(name string) (value string, fixed bool) {
	f, err := files.Open("assets/data/mod.rec")
	if err != nil {
		return "", false
	}
	defer f.Close()
	recs := recfile.Read(f)
	if len(recs) == 0 {
		return "", false
	}
	for _, field := range recs[0] {
		if field.Name == name {
			return field.Value, true
		}
	}
	return "", false
}

// AutosavesOnLevelChange: whether a level change saves the game - the mod's word if it has one, else the player's.
func (c *Configuration) AutosavesOnLevelChange() bool {
	if v, fixed := ModSetting("AutosaveOnLevelChange"); fixed {
		return v == "true"
	}
	return c.AutosaveOnLevelChange
}

// ModFonts are the .ttf files in the active mod's fonts/ folder, the choices of the font menu.
func ModFonts() []string {
	entries, _ := files.GetFileSystem().ReadDir("assets/fonts")
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(strings.ToLower(e.Name()), ".ttf") {
			names = append(names, e.Name())
		}
	}
	return names
}

// FontName is the font the UI draws with: the player's pick when the mod has it, else the mod's Font (mod.rec).
func (c *Configuration) FontName() string {
	if c.Font != "" && slices.Contains(ModFonts(), c.Font) {
		return c.Font
	}
	name, _ := ModSetting("Font")
	return name
}

// FontFile is FontName's path for files.Open.
func (c *Configuration) FontFile() string { return path.Join("assets", "fonts", c.FontName()) }

// FontMenu lists the mod's fonts; picking one saves it to config.rec, hands it to setFont and calls reopen.
func FontMenu(cfg *Configuration, setFont func(file string), reopen, back func()) []util.MenuItem {
	var items []util.MenuItem
	current := cfg.FontName()
	for _, name := range ModFonts() {
		label := strings.TrimSuffix(name, ".ttf")
		if name == current {
			label = "> " + label
		}
		items = append(items, util.MenuItem{MainText: label, Action: func() {
			cfg.Font = name
			cfg.SaveToRecFile(ConfigFile)
			setFont(cfg.FontFile())
			reopen()
		}})
	}
	return append(items, util.MenuItem{MainText: "Back", Action: back})
}
