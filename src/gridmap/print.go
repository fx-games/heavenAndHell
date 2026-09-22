// Debug dumps of the map to stdout.

package gridmap

import (
	"fmt"
	"rpg/src/geometry"
)

func (m *GridMap[ActorType, ItemType, ObjectType]) Print() {
	// walls and floors only
	for y := 0; y < m.mapHeight; y++ {
		for x := 0; x < m.mapWidth; x++ {
			pos := geometry.Point{X: x, Y: y}
			if m.IsTileWalkable(pos) {
				fmt.Printf(".")
			} else {
				fmt.Printf("#")
			}
		}
		fmt.Printf("\n")
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) PrintStairsAndActors() {
	// walls and floors only
	for y := 0; y < m.mapHeight; y++ {
		for x := 0; x < m.mapWidth; x++ {
			pos := geometry.Point{X: x, Y: y}
			if m.IsStairsAt(pos) {
				special := m.GetCell(pos).TileType.Special
				if special == SpecialTileStairsUp {
					fmt.Printf("<")
				} else if special == SpecialTileStairsDown {
					fmt.Printf(">")
				}

			} else if m.IsActorAt(pos) {
				fmt.Printf("a")
			} else if m.IsTileWalkable(pos) {
				fmt.Printf(".")
			} else {
				fmt.Printf("#")
			}
		}
		fmt.Printf("\n")
	}
}

func (m *GridMap[ActorType, ItemType, ObjectType]) PrintWithHighlight(hiPos geometry.Point) {
	// walls and floors only
	for y := 0; y < m.mapHeight; y++ {
		for x := 0; x < m.mapWidth; x++ {
			pos := geometry.Point{X: x, Y: y}
			if pos == hiPos {
				fmt.Printf("X")
			} else if m.IsStairsAt(pos) {
				special := m.GetCell(pos).TileType.Special
				if special == SpecialTileStairsUp {
					fmt.Printf("<")
				} else if special == SpecialTileStairsDown {
					fmt.Printf(">")
				}

			} else if m.IsActorAt(pos) {
				fmt.Printf("a")
			} else if m.IsTileWalkable(pos) {
				fmt.Printf(".")
			} else {
				fmt.Printf("#")
			}
		}
		fmt.Printf("\n")
	}
}
