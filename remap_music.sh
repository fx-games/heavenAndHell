#!/bin/zsh
# Assign a track to each of the five area themes in files/assets/data/music.rec — read on every game start.
# In sfxmapper: click a theme on the left, browse to a track on the right, "Add". s saves, F10 quits.
cd "${0:A:h}" && sfxmapper 8 8 files/assets/classic_roguelike.png files/assets/data/music.rec files/assets \
  files/assets/data/icons/actors.rec files/assets/data/icons/items.rec files/assets/data/icons/tiles.rec
