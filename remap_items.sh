#!/bin/zsh
# Remap items icons in files/mods/d1/data/icons/items.rec — read fresh on every game start, no rebuild needed.
# In the remapper: click a name on the left, click a tile on the right, s saves, F10 quits.
cd "${0:A:h}" && remapper 8 8 files/mods/d1/classic_roguelike.png files/mods/d1/data/icons/items.rec
