package renderer

type BitmapFont struct {
	atlas   TextureAtlas
	fontMap map[rune]uint16
}

func (f BitmapFont) IsLoaded() bool {
	return f.atlas.imageData != nil && f.fontMap != nil && len(f.fontMap) > 0
}

type FontAtlasDescription struct {
	IndexOfCapitalA int
	IndexOfSmallA   *int
	IndexOfZero     *int
	IndexOfOne      *int
	Chains          []SpecialCharacterChain
}
type SpecialCharacterChain struct {
	StartIndex int
	Characters []rune
}
