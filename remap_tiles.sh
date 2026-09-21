#!/bin/zsh
# Remap tiles icons in files/mods/<mod>/data/icons/tiles.rec — read fresh on every game start, no rebuild needed.
# In the remapper: click a name on the left, click a tile on the right, s saves, F10 quits.
MOD=${1:-d1} # the mod to remap: ./remap_tiles.sh heavenandhell
cd "${0:A:h}" && remapper 8 8 files/mods/$MOD/classic_roguelike.png files/mods/$MOD/data/icons/tiles.rec
