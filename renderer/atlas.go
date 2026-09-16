package renderer

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
	"image/color"
	"log"
	"rpg/files"
	"rpg/geometry"
)

type CellDrawInfo struct {
	Icon  int32
	Color color.Color
	Atlas TextureAtlas
}
type TextureAtlas struct {
	imageData *ebiten.Image
	tileSizeX int
	tileSizeY int
}

func (a TextureAtlas) GetTileSize() geometry.Point {
	return geometry.Point{X: a.tileSizeX, Y: a.tileSizeY}
}

func NewTextureAtlas(imageFilename string, tileSizeX, tileSizeY int) TextureAtlas {
	return TextureAtlas{
		imageData: ebiten.NewImageFromImage(mustLoadImage(imageFilename)),
		tileSizeX: tileSizeX,
		tileSizeY: tileSizeY,
	}
}

func mustLoadImage(filename string) image.Image {
	img, _, err := image.Decode(files.MustOpenReader(filename))
	if err != nil {
		log.Fatal(err)
	}
	return img
}
