#!/usr/bin/env bash
# Pick a mod and a class, then let the autoplay bot play it (src/app/autoplay.go).
cd "$(dirname "$0")/.." || exit 1

PS3="Mod: "
select mod in heavenandhell d1; do [ -n "$mod" ] && break; done

if [ "$mod" = d1 ]; then classes=(Warrior Rogue Sorcerer); else classes=(Warrior Archer Mage); fi
PS3="Class: "
select name in "${classes[@]}"; do [ -n "$name" ] && break; done
class=$((REPLY - 1))

PS3="Tactics: "
select mode in plain tactical; do [ -n "$mode" ] && break; done
tactical=""
[ "$mode" = tactical ] && tactical=1

read -r -p "Seed [1]: " seed
read -r -p "Runs [1]: " runs

AUTOPLAY_TACTICAL=$tactical go run ./src autoplay "$mod" "${seed:-1}" "${runs:-1}" "$class" 2>/dev/null
