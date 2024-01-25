package game

import (
	"os"
	"rpg/geometry"
	"rpg/recfile"
)

type LevelName string

const (
	LevelNameTown     LevelName = "Town"
	LevelNameTownHell LevelName = "TownHell"
)

func isTownLevel(levelName string) bool {
	return levelName == string(LevelNameTown) || levelName == string(LevelNameTownHell)
}

type Configuration struct {
	AudioEnabled          bool
	ActorBobbingEnabled   bool
	WindowSize            geometry.Point
	DefaultFont           string
	DefaultFontSize       float64
	AutoPickup            bool
	CardinalMovementOnly  bool
	KeepPlayerCentered    bool

	DrawTilesBehindActors bool
	DrawTilesBehindItems  bool
	DrawTilesBehindObjects bool
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
	file, err := os.Open(recFilename)
	if err != nil {
		defaultConfig := NewDefaultConfig()
		defaultConfig.SaveToRecFile(recFilename)
		return defaultConfig
	}
	records := recfile.Read(file)
	if len(records) == 0 {
		defaultConfig := NewDefaultConfig()
		defaultConfig.SaveToRecFile(recFilename)
		return defaultConfig
	}
	configMap := records[0].ToMap()

	return Configuration{
		AudioEnabled:        configMap.GetBoolOrFalse("AudioEnabled"),
		ActorBobbingEnabled: configMap.GetBoolOrTrue("ActorBobbingEnabled"),
		CardinalMovementOnly: configMap.GetBoolOrFalse("CardinalMovementOnly"),
		DefaultFont:         configMap.GetStringOrDefault("DefaultFont", "assets/fonts/exocet-blizzard-light.ttf"),
		DefaultFontSize:     configMap.GetFloatOrDefault("DefaultFontSize", 20.0),
		AutoPickup:          configMap.GetBoolOrTrue("AutoPickup"),
		KeepPlayerCentered:        configMap.GetBoolOrTrue("KeepPlayerCentered"),
		DrawTilesBehindActors: configMap.GetBoolOrFalse("DrawTilesBehindActors"),
		DrawTilesBehindItems:  configMap.GetBoolOrFalse("DrawTilesBehindItems"),
		DrawTilesBehindObjects: configMap.GetBoolOrFalse("DrawTilesBehindObjects"),
		WindowSize: geometry.Point{ //1280, Y: 800
			X: configMap.GetIntOrDefault("WindowWidth", 1280),
			Y: configMap.GetIntOrDefault("WindowHeight", 800),
		},
	}
}

func NewDefaultConfig() Configuration {
	return Configuration{AudioEnabled: true}
}
