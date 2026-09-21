#!/bin/zsh
# Assign voice files to D1's speech ids in files/mods/<mod>/data/voices.rec — read on every game start, no rebuild.
# Each record's "text:" is what the original line said (reference/d1_voice_transcripts.tsv), so you know what to
# record or pick. In sfxmapper: click a line on the left, browse on the right, "Add". s saves, F10 quits.
MOD=${1:-d1} # the mod to remap: ./remap_voices.sh heavenandhell
cd "${0:A:h}" && sfxmapper 8 8 files/mods/$MOD/classic_roguelike.png files/mods/$MOD/data/voices.rec files/mods/$MOD \
  files/mods/$MOD/data/icons/actors.rec files/mods/$MOD/data/icons/items.rec files/mods/$MOD/data/icons/tiles.rec
