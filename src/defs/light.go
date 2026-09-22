package defs

import (
	"strconv"
	"strings"
)

type LightDefinition struct {
	LightColor        string
	FlickerPattern    string
	FlickerFrameDelay float64
}

func (d LightDefinition) IsValid() bool {
	return d.LightColor != "" && d.FlickerPattern != "" && d.FlickerFrameDelay > 0
}

// FlickerPattern is a light's brightness per frame, from its light_flicker_pattern: a built-in name, fire (a
// torch's uneven flame), smooth (a slow even pulse) or steady (no flicker), or the mod's own pattern written out as
// numbers, e.g. "1.0 0.9 0.8 0.9". Anything else flickers like fire.
func FlickerPattern(spec string) []float64 {
	switch spec {
	case "fire", "":
		return GetFireFlickerPattern()
	case "smooth":
		return GetSmoothFlickerPattern()
	case "steady":
		return []float64{1.0}
	}
	var pattern []float64
	for _, f := range strings.Fields(spec) {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return GetFireFlickerPattern()
		}
		pattern = append(pattern, v)
	}
	if len(pattern) == 0 {
		return GetFireFlickerPattern()
	}
	return pattern
}

// GetFireFlickerPattern is the torch flicker (brightness per frame) from H&H's light items.
func GetFireFlickerPattern() []float64 {
	return []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 0.85, 0.92, 0.9, 0.90, 0.92, 0.96, 0.97, 0.99, 1.0, 1.0, 1.0, 0.9, 1.0, 1.0, 1.0, 1.0, 0.9, 0.93, 0.98}
}

func GetSmoothFlickerPattern() []float64 {
	var smoothPattern []float64
	low := 0.7
	high := 1.0
	current := high
	stepCount := 40
	// we want to go from high -> low -> high
	stepSize := ((high - low) * 2) / float64(stepCount)
	reversedDirection := false
	for i := 0; i < stepCount; i++ {
		smoothPattern = append(smoothPattern, current)
		if !reversedDirection {
			current -= stepSize
			if current < low {
				reversedDirection = true
				current = low
			}
		} else {
			current += stepSize
		}
	}
	return smoothPattern
}
