package util

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"rpg/geometry"
)

func GetLoopingFrameFromTick(tick uint64, delayBetweenFramesInSeconds float64, frameCount int) int32 {
	ticksPerInterval := secondsToTicksF(delayBetweenFramesInSeconds)
	intervallCount := float64(tick) / ticksPerInterval
	return int32(intervallCount) % int32(frameCount)
}

func GetPercentageFromTick(tick uint64, cycleTimeInSeconds float64) float64 {
	ticksPerInterval := secondsToTicksF(cycleTimeInSeconds)
	intervallCount := float64(tick) / ticksPerInterval
	// only return the fractional part
	return intervallCount - float64(int64(intervallCount))
}

func SecondsToTicks(seconds float64) uint64 {
	return uint64(secondsToTicksF(seconds))
}

func secondsToTicksF(seconds float64) float64 {
	return safeTPS() * seconds
}

func safeTPS() float64 {
	return max(60, ebiten.ActualTPS())
}

func LerpPoint(start, end geometry.Point, percent float64) geometry.Point {
	return geometry.Point{
		X: int(float64(start.X) + float64(end.X-start.X)*percent),
		Y: int(float64(start.Y) + float64(end.Y-start.Y)*percent),
	}
}

func LerpPointF(start, end geometry.PointF, percent float64) geometry.PointF {
	return geometry.PointF{
		X: start.X + (end.X-start.X)*percent,
		Y: start.Y + (end.Y-start.Y)*percent,
	}
}

func LerpColor(start, end color.Color, percent float64) color.Color {
	r1, g1, b1, a1 := start.RGBA()
	r2, g2, b2, a2 := end.RGBA()
	c := func(a, b uint32) uint8 {
		lo, hi := float64(a>>8), float64(b>>8)
		return uint8(max(min(lo+(hi-lo)*percent, 255), 0))
	}
	return color.RGBA{
		R: c(r1, r2),
		G: c(g1, g2),
		B: c(b1, b2),
		A: c(a1, a2),
	}
}
