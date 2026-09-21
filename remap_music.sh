#!/bin/zsh
# Assign a track to each of the five area themes in files/mods/d1/data/music.rec — read on every game start.
# In sfxmapper: click a theme on the left, browse to a track on the right, "Add". s saves, F10 quits.
cd "${0:A:h}" && sfxmapper 8 8 files/mods/d1/classic_roguelike.png files/mods/d1/data/music.rec files/mods/d1 \
  files/mods/d1/data/icons/actors.rec files/mods/d1/data/icons/items.rec files/mods/d1/data/icons/tiles.rec
