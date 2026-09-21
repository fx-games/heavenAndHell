package defs

type LightDefinition struct {
	LightColor        string
	FlickerPattern    string
	FlickerFrameDelay float64
}

func (d LightDefinition) IsValid() bool {
	return d.LightColor != "" && d.FlickerPattern != "" && d.FlickerFrameDelay > 0
}

// GetFireFlickerPattern is the torch flicker (brightness per frame) from H&H's light items.
func GetFireFlickerPattern() []float64 {
	return []float64{1.0, 1.0, 1.0, 1.0, 1.0, 1.0, 0.85, 0.92, 0.9, 0.90, 0.92, 0.96, 0.97, 0.99, 1.0, 1.0, 1.0, 0.9, 1.0, 1.0, 1.0, 1.0, 0.9, 0.93, 0.98}
}
