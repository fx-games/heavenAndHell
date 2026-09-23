# mapedit

The map editor for this engine. It is its own Go module with its own binary, so
nothing it needs ends up in the game. Being a separate module, it runs from its
own directory:

```sh
cd tools/mapedit
go run .                      # the heavenandhell mod
go run . -mod d1              # another mod under files/mods
go run . -mod d1 -mode pieces
```

It finds the repo by walking up from wherever it is started, so `-mod` takes a
plain mod name; pass a path to edit a mod somewhere else.

It edits the two kinds of map this engine has, and replaces the LDtk toolchain,
which could only do the first one.

## Maps (F1)

The levels in `files/mods/<mod>/maps`: one `.hhm` per level, plus `tileset.hht`.
These are what the `drawn` generator stamps out whole (the towns), and what
`accretion` stamps out room by room (every dungeon template and decoration).

- **Tools** `1` brush, `2` rect (Shift outlines), `3` fill, `4` pick, `5` erase,
  `6` entity, `7` select.
- **Tiles** come from the mod's tileset image. The four markers next to the
  palette are what the game reads off a drawn tile: what it blocks, whether it
  is water, whether it hurts. They belong to the tile, not to the level, so a
  change there is saved into `tileset.hht` with the levels.
- **Entities** are the engine's whole vocabulary, `src/hhm`'s schema: NPC, Sign,
  Door, Stash, Shrine on the Entities layer; Location, POI and the four
  Connectors on the Meta layer. Selecting one opens its fields.
- **Level fields** (`DisplayName`, `Weight`, `IsDecoration`, `UsesDecoration`,
  `SlotID`, `PlugsIntoSlotID`, `CanBeLit`) are in the same inspector when
  nothing is selected. The dungeon generators read every one of them.
- New, copy, rename, resize and delete are on the left. A rename writes a new
  file and deletes the old one; `levels.rec` still names the old one, so fix it
  there too.

## Set pieces (F2)

Diablo's `.DUN` rooms, as the text files in `files/mods/<mod>/data/levels/l1..l4`.
LDtk could not touch these at all.

A megatile has no art in this game: the engine reads the `til`/`sol` tables and
paints a wall or a floor, so that is what the editor draws - each megatile as
its four quarters, coloured by what they block. The palette is every megatile
the level type has.

The four overlays (`transparency`, `items`, `monsters`, `objects`) sit at twice
the megatile resolution, the way a `.DUN` does. Pick one in the tool bar, put
the id you want in the field on the right, and paint.

The writer produces the files byte for byte as they already are: re-saving every
shipped set piece changes nothing, which is what `dun_test.go` checks.

## Everywhere

Wheel zooms, middle-drag or Space-drag pans, arrows scroll. `Ctrl+S` saves,
`Ctrl+Z` / `Ctrl+Shift+Z` undo and redo, `G` grid, `F` the flag tint, `N` entity
names, `I` the set-piece numbers. The bar at the bottom says what the thing
under the pointer is and what the current tool does.
