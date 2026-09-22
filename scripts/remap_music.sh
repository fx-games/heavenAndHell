#!/bin/zsh
# Assign a track to each of the five area themes in files/mods/<mod>/data/music.rec — read on every game start.
# In sfxmapper: click a theme on the left, browse to a track on the right, "Add". s saves, F10 quits.
MOD=${1:-d1} # the mod to remap: scripts/remap_music.sh heavenandhell
cd "${0:A:h}/.." && sfxmapper 8 8 files/mods/$MOD/classic_roguelike.png files/mods/$MOD/data/music.rec files/mods/$MOD \
  files/mods/$MOD/data/icons/actors.rec files/mods/$MOD/data/icons/items.rec files/mods/$MOD/data/icons/tiles.rec
