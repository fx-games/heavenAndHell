#!/bin/zsh
# Assign sound files to sound events in files/assets/data/sounds.rec — read on every game start, no rebuild needed.
# In sfxmapper: click an event on the left; its sounds show in the middle ("play", "Del"); browse on the right and
# assign with "Add" or by dragging a sound onto the middle pane. s saves, F10 quits. An event with several sounds picks one at random.
cd "${0:A:h}" && sfxmapper 8 8 files/assets/classic_roguelike.png files/assets/data/sounds.rec files/assets \
  files/assets/data/icons/actors.rec files/assets/data/icons/items.rec files/assets/data/icons/tiles.rec
