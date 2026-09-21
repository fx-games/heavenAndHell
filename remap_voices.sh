#!/bin/zsh
# Assign voice files to D1's speech ids in files/assets/data/voices.rec — read on every game start, no rebuild.
# Each record's "text:" is what the original line said (reference/d1_voice_transcripts.tsv), so you know what to
# record or pick. In sfxmapper: click a line on the left, browse on the right, "Add". s saves, F10 quits.
cd "${0:A:h}" && sfxmapper 8 8 files/mods/d1/classic_roguelike.png files/assets/data/voices.rec files/assets \
  files/mods/d1/data/icons/actors.rec files/mods/d1/data/icons/items.rec files/mods/d1/data/icons/tiles.rec
