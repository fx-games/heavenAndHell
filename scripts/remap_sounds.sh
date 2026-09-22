#!/bin/zsh
# Assign sound files to sound events in files/mods/<mod>/data/sounds.rec — read on every game start, no rebuild needed.
# In sfxmapper: click an event on the left; its sounds show in the middle ("play", "Del"); browse on the right and
# assign with "Add" or by dragging a sound onto the middle pane. s saves, F10 quits. An event with several sounds picks one at random.
MOD=${1:-d1} # the mod to remap: scripts/remap_sounds.sh heavenandhell
cd "${0:A:h}/.." && sfxmapper 8 8 files/mods/$MOD/classic_roguelike.png files/mods/$MOD/data/sounds.rec files/mods/$MOD \
  files/mods/$MOD/data/icons/actors.rec files/mods/$MOD/data/icons/items.rec files/mods/$MOD/data/icons/tiles.rec
