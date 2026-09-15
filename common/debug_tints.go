package common

import (
	"image/color"
	"rpg/defs"
)

// TintedTile is one icon the map renderer draws with a non-white tint (graphics mode, before lighting and the
// red hurt flash, which are dynamic).
type TintedTile struct {
	Icon      int32
	Color     color.RGBA
	ID, Cause string
}

// DebugTints lists every static tint the game applies, for "rpg debug_tints": D1 monster variants, and used D1
// objects whose ".used" icon is the same tile as the fresh one; everything else has its own remappable icon.
func (g *Factory) DebugTints() []TintedTile {
	var out []TintedTile
	for mt, m := range g.Generator.D.Monsters {
		if a := g.NewD1Monster(mt); a.GetColor() != defs.White {
			out = append(out, TintedTile{a.icon, a.GetColor(), "monster." + m.ID, m.Name + ": variant tint"})
		}
	}
	for _, o := range g.Generator.D.Objects {
		fresh, ok := defs.IconOK("prop." + o.Name)
		if used, hasUsed := defs.IconOK("prop." + o.Name + ".used"); ok && hasUsed && fresh == used {
			out = append(out, TintedTile{used, D1UsedTint, "prop." + o.Name + ".used", "used object: same icon as fresh"})
		}
	}
	return out
}
