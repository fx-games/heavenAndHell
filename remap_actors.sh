#!/bin/zsh
# Remap actors icons in files/assets/data/icons/actors.rec — read fresh on every game start, no rebuild needed.
# In the remapper: click a name on the left, click a tile on the right, s saves, F10 quits.
cd "${0:A:h}" && remapper 8 8 files/assets/classic_roguelike.png files/assets/data/icons/actors.rec
