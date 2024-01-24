package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"rpg/geometry"
	"rpg/renderer"
)

type HybridRenderer struct {
	*renderer.TileRenderer
	*renderer.MapRenderer
	defaultAtlasForMap renderer.TextureAtlas
}

func (h *HybridRenderer) DrawImageOnScreen(screenX int, screenY int, size geometry.Point, image *ebiten.Image) {
	h.TileRenderer.DrawImageOnScreen(screenX, screenY, size, image)
}

func (h *HybridRenderer) DrawOnMapF(pos geometry.PointF, icon int32, color color.Color) {
	h.MapRenderer.DrawOnMapF(pos, h.defaultAtlasForMap, icon, color)
}

func (h *HybridRenderer) GetScaledTileSize() geometry.PointF {
	return h.MapRenderer.GetScaledTileSize()
}

func NewHybridRenderer(tileRenderer *renderer.TileRenderer, mapRenderer *renderer.MapRenderer) *HybridRenderer {
	return &HybridRenderer{
		TileRenderer: tileRenderer,
		MapRenderer:  mapRenderer,
	}
}
func (h *HybridRenderer) DrawStringOnMap(mapPos, screenOffset geometry.Point, text string, color color.Color) {
	h.MapRenderer.DrawStringOnMapWithOffset(mapPos, screenOffset, text, color)
}
func (h *HybridRenderer) DrawOnMap(mapPos geometry.Point, icon int32, color color.Color) {
	h.MapRenderer.DrawOnMap(mapPos, h.defaultAtlasForMap, icon, color)
}

func (h *HybridRenderer) SetDefaultAtlasForMap(texture renderer.TextureAtlas) {
	h.defaultAtlasForMap = texture
}

func (h *HybridRenderer) GetVisibleMap() geometry.Rect {
	return h.MapRenderer.GetVisibleMap()
}
