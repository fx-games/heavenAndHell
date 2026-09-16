package common

import (
	"fmt"
	"io/fs"
	"os"
	"rpg/geometry"
	"rpg/recfile"
	"strings"
)

type ItemNamingScheme string

const (
	ItemNamingSchemeShort        ItemNamingScheme = "Short"
	ItemNamingSchemeEnchantments ItemNamingScheme = "Enchantments"
	ItemNamingSchemeBandStyle    ItemNamingScheme = "BandStyle"
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
	UserConfiguration *Configuration
	FileSystem        fs.ReadDirFS
}

type Configuration struct {
	AudioEnabled         bool
	ActorBobbingEnabled  bool
	WindowSize           geometry.Point
	DefaultFont          string
	DefaultFontSize      float64
	AutoPickup           bool
	CardinalMovementOnly bool
	KeepPlayerCentered   bool

	ItemNamesInShop ItemNamingScheme

	DrawTilesBehindActors  bool
	DrawTilesBehindItems   bool
	DrawTilesBehindObjects bool
	ServeStats             bool
	ServeInventory         bool
	ServeLog               bool
	ServeEquipment         bool
	FlipActorsHorizontally bool

	SoundVolume, MusicVolume, VoiceVolume float64 // 0..1, set in the audio menu

	DiabloAudioEnabled bool // gates assets/diablo_sfx and assets/diablo_music; takes effect on next launch

	AutoExploreOpenContainers bool // auto explore also bumps chests/barrels/sarcophagi it finds

	WeaponBreakWarning bool // ask for confirmation before an attack that would destroy a zero-durability weapon

	Fullscreen bool // set in the video menu or with F11
}

func (c Configuration) ToRecord() recfile.Record {
	return recfile.Record{
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
			Name:  "DefaultFont",
			Value: c.DefaultFont,
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
		{
			Name:  "ServeStats",
			Value: recfile.BoolStr(c.ServeStats),
		},
		{
			Name:  "ServeInventory",
			Value: recfile.BoolStr(c.ServeInventory),
		},
		{
			Name:  "ServeEquipment",
			Value: recfile.BoolStr(c.ServeEquipment),
		},
		{
			Name:  "ServeLog",
			Value: recfile.BoolStr(c.ServeLog),
		},
		{
			Name:  "ItemNamesInShop",
			Value: string(c.ItemNamesInShop),
		},
		{
			Name:  "FlipActorsHorizontally",
			Value: recfile.BoolStr(c.FlipActorsHorizontally),
		},
		{Name: "SoundVolume", Value: recfile.FloatStr(c.SoundVolume)},
		{Name: "MusicVolume", Value: recfile.FloatStr(c.MusicVolume)},
		{Name: "VoiceVolume", Value: recfile.FloatStr(c.VoiceVolume)},
		{Name: "DiabloAudioEnabled", Value: recfile.BoolStr(c.DiabloAudioEnabled)},
		{Name: "AutoExploreOpenContainers", Value: recfile.BoolStr(c.AutoExploreOpenContainers)},
		{Name: "WeaponBreakWarning", Value: recfile.BoolStr(c.WeaponBreakWarning)},
		{Name: "Fullscreen", Value: recfile.BoolStr(c.Fullscreen)},
	}
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
		DefaultFont:               configMap.GetStringOrDefault("DefaultFont", "assets/fonts/Ac437_EagleSpCGA_Alt3.ttf"),
		DefaultFontSize:           configMap.GetFloatOrDefault("DefaultFontSize", 16.0),
		AutoPickup:                configMap.GetBoolOrTrue("AutoPickup"),
		KeepPlayerCentered:        configMap.GetBoolOrTrue("KeepPlayerCentered"),
		DrawTilesBehindActors:     configMap.GetBoolOrFalse("DrawTilesBehindActors"),
		DrawTilesBehindItems:      configMap.GetBoolOrFalse("DrawTilesBehindItems"),
		DrawTilesBehindObjects:    configMap.GetBoolOrFalse("DrawTilesBehindObjects"),
		ServeStats:                configMap.GetBoolOrFalse("ServeStats"),
		ServeInventory:            configMap.GetBoolOrFalse("ServeInventory"),
		ServeEquipment:            configMap.GetBoolOrFalse("ServeEquipment"),
		ServeLog:                  configMap.GetBoolOrFalse("ServeLog"),
		FlipActorsHorizontally:    configMap.GetBoolOrTrue("FlipActorsHorizontally"),
		ItemNamesInShop:           ItemNamingScheme(configMap.GetStringOrDefault("ItemNamesInShop", string(ItemNamingSchemeEnchantments))),
		SoundVolume:               configMap.GetFloatOrDefault("SoundVolume", 1),
		MusicVolume:               configMap.GetFloatOrDefault("MusicVolume", 1),
		VoiceVolume:               configMap.GetFloatOrDefault("VoiceVolume", 1),
		DiabloAudioEnabled:        configMap.GetBoolOrTrue("DiabloAudioEnabled"),
		AutoExploreOpenContainers: configMap.GetBoolOrFalse("AutoExploreOpenContainers"),
		WeaponBreakWarning:        configMap.GetBoolOrTrue("WeaponBreakWarning"),
		Fullscreen:                configMap.GetBoolOrFalse("Fullscreen"),
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

// ConfigFile is the user configuration next to the game.
const ConfigFile = "config.rec"

// SaveVolumes writes the volume settings into the config file in place, keeping its other lines and comments.
func (c Configuration) SaveVolumes(recFilename string) error {
	data, err := os.ReadFile(recFilename)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for _, kv := range [][2]string{{"SoundVolume", recfile.FloatStr(c.SoundVolume)}, {"MusicVolume", recfile.FloatStr(c.MusicVolume)}, {"VoiceVolume", recfile.FloatStr(c.VoiceVolume)}} {
		line, found := kv[0]+": "+kv[1], false
		for i, l := range lines {
			if strings.HasPrefix(l, kv[0]+":") {
				lines[i], found = line, true
			}
		}
		if !found {
			lines = append(lines, line)
		}
	}
	return os.WriteFile(recFilename, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
